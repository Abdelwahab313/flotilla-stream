package live

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	queueSize    = 16
	writeTimeout = 5 * time.Second
	pingInterval = 20 * time.Second
)

type item struct {
	ctx context.Context
	msg []byte
}

type client struct {
	send      chan item
	kicked    chan struct{}
	closeOnce sync.Once
}

func (c *client) kick() { c.closeOnce.Do(func() { close(c.kicked) }) }

// Hub fans every update out to every connected socket. Each socket has a
// bounded queue; a socket whose queue is full is disconnected rather than
// buffered without limit.
type Hub struct {
	Tracer trace.Tracer

	mu      sync.Mutex
	clients map[*client]struct{}
	conns   sync.WaitGroup
}

// Wait blocks until every socket handler has returned, or ctx ends. Call it
// after http.Server.Shutdown: hijacked sockets are not tracked by the server.
func (h *Hub) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() { h.conns.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hub) Clients() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *Hub) Broadcast(ctx context.Context, msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- item{ctx: ctx, msg: msg}:
		default:
			delete(h.clients, c)
			c.kick()
		}
	}
}

func (h *Hub) add() *client {
	c := &client{send: make(chan item, queueSize), kicked: make(chan struct{})}
	h.mu.Lock()
	if h.clients == nil {
		h.clients = map[*client]struct{}{}
	}
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	return c
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.conns.Add(1)
	defer h.conns.Done()
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx := r.Context()
	// The reader must not see shutdown: on a cancelled context it drops the TCP
	// connection without a close frame, racing the GoingAway below.
	peer := conn.CloseRead(context.WithoutCancel(ctx))

	c := h.add()
	defer h.remove(c)

	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	// Writes and pings outlive shutdown by at most writeTimeout so the loop gets
	// back here to send GoingAway; on a cancelled context the library drops the
	// connection without a close frame.
	work := context.WithoutCancel(ctx)
	for {
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusGoingAway, "")
			return
		case <-c.kicked:
			_ = conn.Close(websocket.StatusPolicyViolation, "too slow")
			return
		default:
		}
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusGoingAway, "")
			return
		case <-peer.Done():
			return
		case <-c.kicked:
			_ = conn.Close(websocket.StatusPolicyViolation, "too slow")
			return
		case it := <-c.send:
			if err := h.write(work, conn, it); err != nil {
				slog.Debug("socket write failed", "err", err)
				return
			}
		case <-ping.C:
			pctx, cancel := context.WithTimeout(work, writeTimeout)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (h *Hub) write(ctx context.Context, conn *websocket.Conn, it item) error {
	_, span := h.Tracer.Start(it.ctx, "live-gateway.ws_write", trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()
	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	if err := conn.Write(wctx, websocket.MessageText, it.msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "write")
		return err
	}
	return nil
}
