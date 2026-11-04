package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/Abdelwahab313/flotilla-stream/internal/health"
	"github.com/Abdelwahab313/flotilla-stream/internal/position"
)

func readyz(rd http.Handler) int {
	rec := httptest.NewRecorder()
	rd.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	return rec.Code
}

func TestReadinessRequiresTopicWithLeaders(t *testing.T) {
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1))
	if err != nil {
		t.Fatalf("kfake: %v", err)
	}
	t.Cleanup(cluster.Close)
	kc, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	t.Cleanup(kc.Close)
	adm := kadm.NewClient(kc)
	rd := &health.Readiness{Probes: []health.Probe{{Name: "kafka topic", Check: topicReady(kc, position.Topic)}}}

	if got := readyz(rd); got != http.StatusServiceUnavailable {
		t.Fatalf("readyz with broker up and no topic = %d, want 503", got)
	}
	if _, err := adm.CreateTopic(context.Background(), 6, 1, nil, position.Topic); err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if got := readyz(rd); got != http.StatusOK {
		t.Fatalf("readyz right after the topic was created = %d, want 200", got)
	}
}

func TestReadinessWithBrokerDown(t *testing.T) {
	kc, err := kgo.NewClient(kgo.SeedBrokers("127.0.0.1:1"))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	t.Cleanup(kc.Close)
	rd := &health.Readiness{Probes: []health.Probe{{Name: "kafka topic", Check: topicReady(kc, position.Topic)}}}
	start := time.Now()
	if got := readyz(rd); got != http.StatusServiceUnavailable {
		t.Fatalf("readyz with no broker = %d, want 503", got)
	}
	if d := time.Since(start); d > 1500*time.Millisecond {
		t.Fatalf("readyz took %v, want under the 1 s probe budget plus slack", d)
	}
}

func TestProduceStopsWaitingAtDeadline(t *testing.T) {
	kc, err := kgo.NewClient(kgo.SeedBrokers("127.0.0.1:1"), kgo.DefaultProduceTopic(position.Topic))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	t.Cleanup(kc.Close)
	p := kafkaProducer{kc: kc, tracer: noop.NewTracerProvider().Tracer("test")}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := p.Produce(ctx, position.Reported{FleetID: "f", DriverID: "d", Seq: 1}); err == nil {
		t.Fatal("produce to an unreachable broker succeeded")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("produce returned after %v with a 300 ms deadline", d)
	}
}

func TestProduceAcked(t *testing.T) {
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(6, position.Topic))
	if err != nil {
		t.Fatalf("kfake: %v", err)
	}
	t.Cleanup(cluster.Close)
	kc, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...), kgo.DefaultProduceTopic(position.Topic))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	t.Cleanup(kc.Close)
	p := kafkaProducer{kc: kc, tracer: noop.NewTracerProvider().Tracer("test")}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Produce(ctx, position.Reported{FleetID: "f", DriverID: "d", Seq: 1}); err != nil {
		t.Fatalf("produce: %v", err)
	}
}
