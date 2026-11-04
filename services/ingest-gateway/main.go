package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/Abdelwahab313/flotilla-stream/internal/health"
	"github.com/Abdelwahab313/flotilla-stream/internal/position"
	"github.com/Abdelwahab313/flotilla-stream/internal/telemetry"
	"github.com/Abdelwahab313/flotilla-stream/services/ingest-gateway/internal/ingest"
)

const service = "ingest-gateway"

type config struct {
	addr       string
	brokers    []string
	token      string
	drainDelay time.Duration
}

func loadConfig() (config, error) {
	c := config{
		addr:    getenv("INGEST_ADDR", "127.0.0.1:8081"),
		brokers: strings.Split(getenv("KAFKA_BROKERS", "localhost:9092"), ","),
		token:   getenv("INGEST_TOKEN", "dev-token"),
	}
	if len(c.token) < 8 {
		return c, errors.New("INGEST_TOKEN must be at least 8 characters")
	}
	d, err := time.ParseDuration(getenv("SHUTDOWN_DRAIN_DELAY", "5s"))
	if err != nil || d < 0 {
		return c, fmt.Errorf("SHUTDOWN_DRAIN_DELAY: want a duration like 5s, got %q", os.Getenv("SHUTDOWN_DRAIN_DELAY"))
	}
	c.drainDelay = d
	return c, nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("exit", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := telemetry.Setup(ctx, service)
	if err != nil {
		return err
	}
	defer shutdownTracing()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	kc, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.brokers...),
		kgo.DefaultProduceTopic(position.Topic),
		kgo.RequiredAcks(kgo.AllISRAcks()),
	)
	if err != nil {
		return fmt.Errorf("kafka client: %w", err)
	}
	defer kc.Close()

	ready := &health.Readiness{Probes: []health.Probe{{Name: "kafka topic " + position.Topic, Check: topicReady(kc, position.Topic)}}}
	mux := http.NewServeMux()
	mux.Handle("POST /v1/positions", otelhttp.NewHandler(&ingest.Handler{
		Token:          cfg.token,
		Producer:       kafkaProducer{kc: kc, tracer: otel.Tracer(service)},
		Now:            time.Now,
		ProduceTimeout: ingest.DefaultProduceTimeout,
	}, "POST /v1/positions"))
	mux.HandleFunc("GET /healthz", health.Live)
	mux.Handle("GET /readyz", ready)

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		slog.Info("listening", "addr", cfg.addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-gctx.Done()
		ready.Drain()
		slog.Info("draining", "delay", cfg.drainDelay.String())
		time.Sleep(cfg.drainDelay)
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(sctx)
	})
	return g.Wait()
}

type kafkaProducer struct {
	kc     *kgo.Client
	tracer trace.Tracer
}

func (p kafkaProducer) Produce(ctx context.Context, r position.Reported) error {
	ctx, span := p.tracer.Start(ctx, "ingest.produce", trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.destination.name", position.Topic),
			attribute.String("fleet.id", r.FleetID),
			attribute.String("driver.id", r.DriverID),
			attribute.Int64("position.seq", r.Seq),
		))
	defer span.End()

	value, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode %s: %w", r.DriverID, err)
	}
	rec := &kgo.Record{Key: []byte(r.Key()), Value: value}
	otel.GetTextMapPropagator().Inject(ctx, telemetry.RecordCarrier{R: rec})
	acked := make(chan error, 1)
	p.kc.Produce(ctx, rec, func(_ *kgo.Record, err error) { acked <- err })
	// Once a request is in flight the client cannot fail the record, so stop
	// waiting at the deadline. The record may still land, in partition order behind
	// anything sent earlier, and the projector's newest-seq rule discards it if stale.
	select {
	case err = <-acked:
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "produce")
		return fmt.Errorf("produce %s: %w", r.DriverID, err)
	}
	span.SetAttributes(attribute.Int("messaging.kafka.partition", int(rec.Partition)), attribute.Int64("messaging.kafka.offset", rec.Offset))
	return nil
}

// topicReady requires the topic to exist with a leader on every partition;
// a reachable broker alone does not mean a produce can succeed. It asks a
// broker directly: the client's cached metadata can lag by seconds.
func topicReady(kc *kgo.Client, topic string) health.Check {
	return func(ctx context.Context) error {
		req := kmsg.NewPtrMetadataRequest()
		rt := kmsg.NewMetadataRequestTopic()
		rt.Topic = kmsg.StringPtr(topic)
		req.Topics = append(req.Topics, rt)
		resp, err := req.RequestWith(ctx, kc)
		if err != nil {
			return fmt.Errorf("metadata: %w", err)
		}
		for _, t := range resp.Topics {
			if t.Topic == nil || *t.Topic != topic {
				continue
			}
			if err := kerr.ErrorForCode(t.ErrorCode); err != nil {
				return fmt.Errorf("topic %s: %w", topic, err)
			}
			if len(t.Partitions) == 0 {
				return fmt.Errorf("topic %s has no partitions", topic)
			}
			for _, p := range t.Partitions {
				if err := kerr.ErrorForCode(p.ErrorCode); err != nil || p.Leader < 0 {
					return fmt.Errorf("topic %s partition %d has no leader (%v)", topic, p.Partition, err)
				}
			}
			return nil
		}
		return fmt.Errorf("topic %s missing from metadata", topic)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
