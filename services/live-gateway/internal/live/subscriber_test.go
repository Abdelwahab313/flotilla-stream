package live

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/Abdelwahab313/flotilla-stream/internal/health"
	"github.com/Abdelwahab313/flotilla-stream/internal/position"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func startRedisAt(t *testing.T, addr string) *miniredis.Miniredis {
	t.Helper()
	m := miniredis.NewMiniRedis()
	if err := m.StartAddr(addr); err != nil {
		t.Fatalf("miniredis at %s: %v", addr, err)
	}
	return m
}

func eventually(t *testing.T, within time.Duration, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not within %v", what, within)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSubscriberWaitsForRedisAndResubscribes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := freeAddr(t)
	rdb := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	h := &Hub{Tracer: noop.NewTracerProvider().Tracer("test")}
	cl := h.add()
	s := &Subscriber{RDB: rdb, Hub: h, Tracer: noop.NewTracerProvider().Tracer("test"),
		Backoff: health.Backoff{Min: 20 * time.Millisecond, Max: 200 * time.Millisecond}, Idle: 100 * time.Millisecond}
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	ready := func() bool { return s.Ready(ctx) == nil }
	time.Sleep(300 * time.Millisecond)
	if ready() {
		t.Fatal("ready with Redis down")
	}

	m := startRedisAt(t, addr)
	eventually(t, 15*time.Second, "ready after Redis came up", ready)
	assertForwarded(t, m, cl, 1)

	m.Close()
	eventually(t, 5*time.Second, "not ready after Redis went away", func() bool { return !ready() })

	m2 := startRedisAt(t, addr)
	defer m2.Close()
	eventually(t, 15*time.Second, "ready after Redis came back", ready)
	assertForwarded(t, m2, cl, 2)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop on shutdown")
	}
}

func assertForwarded(t *testing.T, m *miniredis.Miniredis, cl *client, seq int) {
	t.Helper()
	payload := `{"driver_id":"D-1","seq":` + strconv.Itoa(seq) + `,"lat":30,"lon":31}`
	eventually(t, 5*time.Second, "pattern subscriber registered", func() bool {
		return m.Publish(position.UpdatesChannel("fleet-t"), payload) > 0
	})
	select {
	case it := <-cl.send:
		if string(it.msg) != payload {
			t.Fatalf("forwarded %s, want %s", it.msg, payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("update seq %d not forwarded", seq)
	}
}

func TestSubscriberStopsWhileBlockedInReceive(t *testing.T) {
	m := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: m.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	s := &Subscriber{RDB: rdb, Hub: &Hub{}, Tracer: noop.NewTracerProvider().Tracer("test"), Backoff: health.Default, Idle: time.Minute}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	eventually(t, 5*time.Second, "subscribed", func() bool { return s.Ready(ctx) == nil })
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run still blocked in receive 1 s after shutdown")
	}
}

func TestSubscriberZeroValuesDoNotSpin(t *testing.T) {
	var dials atomic.Int64
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", Dialer: func(context.Context, string, string) (net.Conn, error) {
		dials.Add(1)
		return nil, errors.New("down")
	}})
	t.Cleanup(func() { _ = rdb.Close() })
	s := &Subscriber{RDB: rdb, Hub: &Hub{}, Tracer: noop.NewTracerProvider().Tracer("test")}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := dials.Load(); n > 20 {
		t.Fatalf("%d dials in 1 s with zero-valued Backoff, want a backed-off handful", n)
	}
}
