package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/errgroup"

	"github.com/Abdelwahab313/flotilla-stream/internal/health"
	"github.com/Abdelwahab313/flotilla-stream/internal/position"
	"github.com/Abdelwahab313/flotilla-stream/internal/telemetry"
	"github.com/Abdelwahab313/flotilla-stream/services/location-projector/internal/projector"
)

const (
	service = "location-projector"
	group   = "location-projector"
)

type config struct {
	addr      string
	brokers   []string
	redisAddr string
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

	cfg := config{
		addr:      getenv("PROJECTOR_ADDR", "127.0.0.1:8082"),
		brokers:   strings.Split(getenv("KAFKA_BROKERS", "localhost:9092"), ","),
		redisAddr: getenv("REDIS_ADDR", "localhost:6379"),
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.redisAddr})
	defer rdb.Close()

	// The group client joins as soon as it loads metadata, so it is created only
	// once Redis is reachable: a member that cannot write would hold partitions
	// and process nothing.
	probe, err := kgo.NewClient(kgo.SeedBrokers(cfg.brokers...))
	if err != nil {
		return fmt.Errorf("kafka client: %w", err)
	}
	defer probe.Close()

	mux := http.NewServeMux()
	redisUp := func(ctx context.Context) error { return rdb.Ping(ctx).Err() }
	mux.HandleFunc("GET /healthz", health.Live)
	mux.Handle("GET /readyz", &health.Readiness{Probes: []health.Probe{{Name: "kafka", Check: probe.Ping}, {Name: "redis", Check: redisUp}}})
	srv := &http.Server{Addr: cfg.addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := health.WaitFor(gctx, "redis", health.Default, redisUp); err != nil {
			return nil
		}
		if err := health.WaitFor(gctx, "kafka", health.Default, probe.Ping); err != nil {
			return nil
		}
		kc, err := kgo.NewClient(
			kgo.SeedBrokers(cfg.brokers...),
			kgo.ConsumerGroup(group),
			kgo.ConsumeTopics(position.Topic),
			kgo.DisableAutoCommit(),
			kgo.BlockRebalanceOnPoll(),
			kgo.FetchIsolationLevel(kgo.ReadCommitted()),
		)
		if err != nil {
			return fmt.Errorf("kafka group client: %w", err)
		}
		defer kc.Close()
		c := &projector.Consumer{
			Client:      kc,
			Store:       projector.Store{RDB: rdb},
			Tracer:      otel.Tracer(service),
			RetryDelays: []time.Duration{100 * time.Millisecond, 250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second, 2 * time.Second, 2 * time.Second, 2 * time.Second},
		}
		slog.Info("consuming", "topic", position.Topic, "group", group)
		return c.Run(gctx)
	})
	g.Go(func() error {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-gctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(sctx)
	})
	return g.Wait()
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
