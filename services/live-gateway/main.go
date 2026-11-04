package main

import (
	"context"
	"embed"
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
	"golang.org/x/sync/errgroup"

	"github.com/Abdelwahab313/flotilla-stream/internal/health"
	"github.com/Abdelwahab313/flotilla-stream/internal/telemetry"
	"github.com/Abdelwahab313/flotilla-stream/services/live-gateway/internal/live"
)

const service = "live-gateway"

//go:embed web
var webFS embed.FS

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

	addr := getenv("LIVE_ADDR", "127.0.0.1:8080")
	drainDelay, err := time.ParseDuration(getenv("SHUTDOWN_DRAIN_DELAY", "5s"))
	if err != nil || drainDelay < 0 {
		return fmt.Errorf("SHUTDOWN_DRAIN_DELAY: want a duration like 5s, got %q", os.Getenv("SHUTDOWN_DRAIN_DELAY"))
	}
	rdb := redis.NewClient(&redis.Options{Addr: getenv("REDIS_ADDR", "localhost:6379")})
	defer rdb.Close()

	web, err := fs.Sub(webFS, "web")
	if err != nil {
		return fmt.Errorf("embedded page: %w", err)
	}
	tracer := otel.Tracer(service)
	hub := &live.Hub{Tracer: tracer}
	sub := &live.Subscriber{RDB: rdb, Hub: hub, Tracer: tracer, Backoff: health.Default, Idle: 15 * time.Second}
	ready := &health.Readiness{Probes: []health.Probe{{Name: "redis subscription", Check: sub.Ready}}}

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(noListing{web}))
	mux.Handle("GET /ws", hub)
	mux.HandleFunc("GET /healthz", health.Live)
	mux.Handle("GET /readyz", ready)

	// Sockets and the subscription keep serving through the drain delay, so
	// clients keep moving dots until the endpoint is gone; then sockets get
	// GoingAway. Sockets are hijacked, so Shutdown does not reach them.
	serveCtx, stopServing := context.WithCancel(context.Background())
	defer stopServing()
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return serveCtx },
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return sub.Run(serveCtx) })
	g.Go(func() error {
		slog.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-gctx.Done()
		ready.Drain()
		slog.Info("draining", "delay", drainDelay.String())
		time.Sleep(drainDelay)
		stopServing()
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

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
