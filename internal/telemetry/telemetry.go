package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Setup installs a JSON slog default logger, the W3C propagator and an OTLP/gRPC
// tracer provider (endpoint from OTEL_EXPORTER_OTLP_ENDPOINT, default localhost:4317).
// The returned shutdown flushes spans for at most shutdownTimeout: with the
// collector down, an unbounded flush retries exports and blocks process exit.
func Setup(ctx context.Context, service string) (func(), error) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("service", service))
	otel.SetTextMapPropagator(propagation.TraceContext{})

	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(attribute.String("service.name", service)))
	if err != nil {
		return nil, fmt.Errorf("resource: %w", err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	return func() {
		sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := tp.Shutdown(sctx); err != nil {
			slog.Warn("trace flush incomplete at shutdown", "err", err)
		}
	}, nil
}

const shutdownTimeout = 5 * time.Second

// RecordCarrier carries trace context in Kafka record headers.
type RecordCarrier struct{ R *kgo.Record }

func (c RecordCarrier) Get(key string) string {
	for _, h := range c.R.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c RecordCarrier) Set(key, value string) {
	for i, h := range c.R.Headers {
		if h.Key == key {
			c.R.Headers[i].Value = []byte(value)
			return
		}
	}
	c.R.Headers = append(c.R.Headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
}

func (c RecordCarrier) Keys() []string {
	keys := make([]string, len(c.R.Headers))
	for i, h := range c.R.Headers {
		keys[i] = h.Key
	}
	return keys
}
