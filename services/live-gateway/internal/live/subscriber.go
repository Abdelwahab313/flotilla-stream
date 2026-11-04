package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/Abdelwahab313/flotilla-stream/internal/health"
	"github.com/Abdelwahab313/flotilla-stream/internal/position"
)

type message struct {
	DriverID string  `json:"driver_id"`
	Seq      int64   `json:"seq"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
}

// Subscriber forwards the projector's accepted updates from Redis pub/sub to
// the hub. It subscribes until ctx ends, re-subscribing after any break.
type Subscriber struct {
	RDB     *redis.Client
	Hub     *Hub
	Tracer  trace.Tracer
	Backoff health.Backoff
	// Idle is how long a silent subscription waits before it pings Redis;
	// zero or negative uses defaultIdle.
	Idle time.Duration

	ready atomic.Bool
}

func (s *Subscriber) Ready(context.Context) error {
	if !s.ready.Load() {
		return errors.New("not subscribed")
	}
	return nil
}

const defaultIdle = 15 * time.Second

func (s *Subscriber) Run(ctx context.Context) error {
	b := s.Backoff
	idle := s.Idle
	if idle <= 0 {
		idle = defaultIdle
	}
	for {
		subscribed, err := s.subscribeOnce(ctx, idle)
		s.ready.Store(false)
		if ctx.Err() != nil {
			return nil
		}
		msg := "subscribe failed, retrying"
		if subscribed {
			b.Reset()
			msg = "subscription lost, retrying"
		}
		d := b.Next()
		slog.Warn(msg, "pattern", position.UpdatesPattern, "retry_in", d.String(), "err", err)
		if !health.Sleep(ctx, d) {
			return nil
		}
	}
}

func (s *Subscriber) subscribeOnce(ctx context.Context, idle time.Duration) (bool, error) {
	ps := s.RDB.PSubscribe(ctx, position.UpdatesPattern)
	defer ps.Close()
	// A blocking receive does not watch ctx; closing the subscription ends it.
	defer context.AfterFunc(ctx, func() { _ = ps.Close() })()
	if _, err := ps.Receive(ctx); err != nil {
		return false, fmt.Errorf("psubscribe %s: %w", position.UpdatesPattern, err)
	}
	s.ready.Store(true)
	slog.Info("subscribed", "pattern", position.UpdatesPattern)

	pinged := false
	for {
		msg, err := ps.ReceiveTimeout(ctx, idle)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() && !pinged && ctx.Err() == nil {
				if err := ps.Ping(ctx); err != nil {
					return true, fmt.Errorf("ping: %w", err)
				}
				pinged = true
				continue
			}
			return true, err
		}
		pinged = false
		if m, ok := msg.(*redis.Message); ok {
			s.forward(ctx, m)
		}
	}
}

func (s *Subscriber) forward(ctx context.Context, m *redis.Message) {
	var u position.Update
	if err := json.Unmarshal([]byte(m.Payload), &u); err != nil {
		slog.Warn("skipping undecodable update", "channel", m.Channel, "err", err)
		return
	}
	out, err := json.Marshal(message{DriverID: u.DriverID, Seq: u.Seq, Lat: u.Lat, Lon: u.Lon})
	if err != nil {
		return
	}
	uctx := otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(u.Trace))
	uctx, span := s.Tracer.Start(uctx, "live-gateway.broadcast", trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attribute.String("driver.id", u.DriverID), attribute.Int64("position.seq", u.Seq), attribute.Int("live.clients", s.Hub.Clients())))
	s.Hub.Broadcast(uctx, out)
	span.End()
}
