package health

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Check func(context.Context) error

// Backoff doubles from Min to Max. A zero or negative Min uses Default's.
type Backoff struct {
	Min, Max time.Duration
	next     time.Duration
}

var Default = Backoff{Min: 500 * time.Millisecond, Max: 5 * time.Second}

func (b *Backoff) Next() time.Duration {
	lo := b.Min
	if lo <= 0 {
		lo = Default.Min
	}
	hi := max(b.Max, lo)
	if b.next < lo {
		b.next = lo
	}
	d := min(b.next, hi)
	b.next = min(d*2, hi)
	return d
}

func (b *Backoff) Reset() { b.next = 0 }

// Sleep waits d or until ctx ends; it reports whether the full wait elapsed.
func Sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// WaitFor runs check until it succeeds or ctx ends, backing off between tries.
func WaitFor(ctx context.Context, name string, b Backoff, check Check) error {
	for {
		cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := check(cctx)
		cancel()
		if err == nil {
			slog.Info("dependency reachable", "dependency", name)
			return nil
		}
		d := b.Next()
		slog.Warn("waiting for dependency", "dependency", name, "retry_in", d.String(), "err", err)
		if !Sleep(ctx, d) {
			return ctx.Err()
		}
	}
}

// Live answers 200 while the process serves HTTP; it checks no dependency.
func Live(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

type Probe struct {
	Name  string
	Check Check
}

const probeTimeout = time.Second

var errDraining = errors.New("shutting down")

// Readiness answers 200 only when every probe passes, in order, each within
// its own second, and 503 once Drain is called.
type Readiness struct {
	Probes []Probe

	draining atomic.Bool
	mu       sync.Mutex
	failing  string
}

func (rd *Readiness) Drain() { rd.draining.Store(true) }

func (rd *Readiness) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if rd.draining.Load() {
		rd.report("draining", errDraining)
		http.Error(w, "draining", http.StatusServiceUnavailable)
		return
	}
	for _, p := range rd.Probes {
		ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
		err := p.Check(ctx)
		cancel()
		if err != nil {
			rd.report(p.Name, err)
			http.Error(w, p.Name+" not ready", http.StatusServiceUnavailable)
			return
		}
	}
	rd.report("", nil)
	w.WriteHeader(http.StatusOK)
}

// report logs only when the failing probe changes, so a probe every few
// seconds does not repeat the same line.
func (rd *Readiness) report(name string, err error) {
	rd.mu.Lock()
	defer rd.mu.Unlock()
	if name == rd.failing {
		return
	}
	rd.failing = name
	if err != nil {
		slog.Warn("not ready", "dependency", name, "err", err)
	} else {
		slog.Info("ready")
	}
}
