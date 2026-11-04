package projector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/Abdelwahab313/flotilla-stream/internal/position"
	"github.com/Abdelwahab313/flotilla-stream/internal/telemetry"
)

const pollBatch = 500

type Applier interface {
	Apply(ctx context.Context, r position.Reported, trace map[string]string) (bool, error)
}

// Consumer drives a group client built with DisableAutoCommit and
// BlockRebalanceOnPoll.
type Consumer struct {
	Client      *kgo.Client
	Store       Applier
	Tracer      trace.Tracer
	RetryDelays []time.Duration

	skipped int64
}

var errStopped = errors.New("stopped")

// Run returns nil on shutdown and an error when a record cannot be applied
// after its retries; in both cases nothing past the last fully applied batch is
// committed, so a restart redelivers it.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		if err := c.pollOnce(ctx); err != nil {
			if errors.Is(err, errStopped) {
				return nil
			}
			return err
		}
	}
}

func (c *Consumer) pollOnce(ctx context.Context) error {
	fetches := c.Client.PollRecords(ctx, pollBatch)
	// Without this on every path, Close blocks forever leaving the group.
	defer c.Client.AllowRebalance()
	if ctx.Err() != nil || fetches.IsClientClosed() {
		return errStopped
	}
	fetches.EachError(func(topic string, partition int32, err error) {
		slog.Warn("fetch error", "topic", topic, "partition", partition, "err", err)
	})
	records := fetches.Records()
	if len(records) == 0 {
		return nil
	}
	// A polled batch is finished on shutdown unless it is stuck retrying.
	work := context.WithoutCancel(ctx)
	for _, rec := range records {
		if err := c.applyWithRetry(ctx, work, rec); err != nil {
			return err
		}
	}
	cctx, cancel := context.WithTimeout(work, 10*time.Second)
	defer cancel()
	if err := c.Client.CommitRecords(cctx, records...); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (c *Consumer) applyWithRetry(stop, work context.Context, rec *kgo.Record) error {
	for attempt := 0; ; attempt++ {
		err := c.apply(work, rec)
		if err == nil {
			return nil
		}
		if attempt >= len(c.RetryDelays) {
			return fmt.Errorf("partition %d offset %d after %d attempts: %w", rec.Partition, rec.Offset, attempt+1, err)
		}
		slog.Warn("apply failed, retrying", "partition", rec.Partition, "offset", rec.Offset, "attempt", attempt+1, "err", err)
		t := time.NewTimer(c.RetryDelays[attempt])
		select {
		case <-stop.Done():
			t.Stop()
			return errStopped
		case <-t.C:
		}
	}
}

func (c *Consumer) apply(ctx context.Context, rec *kgo.Record) error {
	ctx = otel.GetTextMapPropagator().Extract(ctx, telemetry.RecordCarrier{R: rec})
	ctx, span := c.Tracer.Start(ctx, "projector.apply", trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.destination.name", rec.Topic),
			attribute.Int("messaging.kafka.partition", int(rec.Partition)),
			attribute.Int64("messaging.kafka.offset", rec.Offset),
		))
	defer span.End()

	var r position.Reported
	err := json.Unmarshal(rec.Value, &r)
	if err == nil {
		err = r.Validate()
	}
	if err != nil {
		c.skipped++
		slog.Warn("skipping invalid record", "partition", rec.Partition, "offset", rec.Offset, "skipped_total", c.skipped, "err", err)
		span.SetStatus(codes.Error, "invalid")
		return nil
	}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	accepted, err := c.Store.Apply(ctx, r, carrier)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "apply")
		return err
	}
	span.SetAttributes(attribute.String("driver.id", r.DriverID), attribute.Int64("position.seq", r.Seq), attribute.Bool("position.accepted", accepted))
	return nil
}
