package live

import (
	"context"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"go.opentelemetry.io/otel/trace/noop"
)

func dial(t *testing.T, ctx context.Context, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

func waitClients(t *testing.T, h *Hub, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for h.Clients() != n {
		if time.Now().After(deadline) {
			t.Fatalf("clients = %d, want %d", h.Clients(), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestBroadcastReachesEverySocketInOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := &Hub{Tracer: noop.NewTracerProvider().Tracer("test")}
	srv := httptest.NewServer(h)
	defer srv.Close()

	a, b := dial(t, ctx, srv), dial(t, ctx, srv)
	waitClients(t, h, 2)
	for i := 1; i <= 3; i++ {
		h.Broadcast(ctx, []byte(strconv.Itoa(i)))
	}
	for _, c := range []*websocket.Conn{a, b} {
		for i := 1; i <= 3; i++ {
			_, got, err := c.Read(ctx)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if string(got) != strconv.Itoa(i) {
				t.Fatalf("got %q, want %d", got, i)
			}
		}
	}
}

func TestSlowSocketIsDisconnectedNotBuffered(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	h := &Hub{Tracer: noop.NewTracerProvider().Tracer("test")}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dial(t, ctx, srv)
	waitClients(t, h, 1)
	msg := []byte(strings.Repeat("x", 16<<10))
	for sent := 0; h.Clients() > 0; sent++ {
		if sent > 100_000 {
			t.Fatalf("socket still connected after %d unread messages", sent)
		}
		h.Broadcast(ctx, msg)
	}

	for {
		_, _, err := c.Read(ctx)
		if err == nil {
			continue
		}
		if got := websocket.CloseStatus(err); got != websocket.StatusPolicyViolation {
			t.Fatalf("close status = %v (err %v), want StatusPolicyViolation", got, err)
		}
		return
	}
}

func TestClosedSocketIsRemoved(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := &Hub{Tracer: noop.NewTracerProvider().Tracer("test")}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c := dial(t, ctx, srv)
	waitClients(t, h, 1)
	_ = c.Close(websocket.StatusNormalClosure, "")
	waitClients(t, h, 0)
}

func TestShutdownSendsGoingAwayAndWaitReturns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	base, stop := context.WithCancel(context.Background())
	h := &Hub{Tracer: noop.NewTracerProvider().Tracer("test")}
	srv := httptest.NewUnstartedServer(h)
	srv.Config.BaseContext = func(net.Listener) context.Context { return base }
	srv.Start()
	defer srv.Close()

	c := dial(t, ctx, srv)
	waitClients(t, h, 1)
	stop()
	_, _, err := c.Read(ctx)
	if got := websocket.CloseStatus(err); got != websocket.StatusGoingAway {
		t.Fatalf("close status = %v (err %v), want StatusGoingAway", got, err)
	}
	if err := h.Wait(ctx); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}
