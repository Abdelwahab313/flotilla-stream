package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/Abdelwahab313/flotilla-stream/internal/position"
	"github.com/Abdelwahab313/flotilla-stream/internal/telemetry"
	"github.com/Abdelwahab313/flotilla-stream/services/live-gateway/internal/live"
)

const service = "live-gateway"

//go:embed web
var webFS embed.FS

type message struct {
	DriverID string  `json:"driver_id"`
	Seq      int64   `json:"seq"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
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
	defer func() { _ = shutdownTracing(context.Background()) }()

	addr := getenv("LIVE_ADDR", "127.0.0.1:8080")
	rdb := redis.NewClient(&redis.Options{Addr: getenv("REDIS_ADDR", "localhost:6379")})
	defer rdb.Close()

	web, err := fs.Sub(webFS, "web")
	if err != nil {
		return fmt.Errorf("embedded page: %w", err)
	}
	tracer := otel.Tracer(service)
	hub := &live.Hub{Tracer: tracer}

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(noListing{web}))
	mux.Handle("GET /ws", hub)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		pctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := rdb.Ping(pctx).Err(); err != nil {
			http.Error(w, "redis unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		// Sockets are hijacked, so Shutdown does not reach them; they end with ctx.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return subscribe(gctx, rdb, hub, tracer) })
	g.Go(func() error {
		slog.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-gctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := srv.Shutdown(sctx)
		if werr := hub.Wait(sctx); werr != nil {
			slog.Warn("sockets still open at shutdown deadline", "clients", hub.Clients())
		}
		return err
	})
	return g.Wait()
}

// noListing serves files and directories that have an index.html, never a
// generated directory listing.
type noListing struct{ fs.FS }

func (n noListing) Open(name string) (fs.File, error) {
	f, err := n.FS.Open(name)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if st.IsDir() {
		if _, err := fs.Stat(n.FS, path.Join(name, "index.html")); err != nil {
			_ = f.Close()
			return nil, fs.ErrNotExist
		}
	}
	return f, nil
}

func subscribe(ctx context.Context, rdb *redis.Client, hub *live.Hub, tracer trace.Tracer) error {
	ps := rdb.PSubscribe(ctx, position.UpdatesPattern)
	defer ps.Close()
	if _, err := ps.Receive(ctx); err != nil {
		return fmt.Errorf("psubscribe %s: %w", position.UpdatesPattern, err)
	}
	slog.Info("subscribed", "pattern", position.UpdatesPattern)
	ch := ps.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case m, ok := <-ch:
			if !ok {
				return nil
			}
			var u position.Update
			if err := json.Unmarshal([]byte(m.Payload), &u); err != nil {
				slog.Warn("skipping undecodable update", "channel", m.Channel, "err", err)
				continue
			}
			msg, err := json.Marshal(message{DriverID: u.DriverID, Seq: u.Seq, Lat: u.Lat, Lon: u.Lon})
			if err != nil {
				continue
			}
			uctx := otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(u.Trace))
			uctx, span := tracer.Start(uctx, "live-gateway.broadcast", trace.WithSpanKind(trace.SpanKindConsumer),
				trace.WithAttributes(attribute.String("driver.id", u.DriverID), attribute.Int64("position.seq", u.Seq), attribute.Int("live.clients", hub.Clients())))
			hub.Broadcast(uctx, msg)
			span.End()
		}
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
