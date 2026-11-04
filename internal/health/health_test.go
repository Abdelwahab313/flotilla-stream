package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBackoffDoublesToCap(t *testing.T) {
	b := Backoff{Min: 500 * time.Millisecond, Max: 5 * time.Second}
	want := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for i, w := range want {
		if got := b.Next(); got != w {
			t.Fatalf("step %d = %v, want %v", i, got, w)
		}
	}
	b.Reset()
	if got := b.Next(); got != 500*time.Millisecond {
		t.Fatalf("after reset = %v, want 500ms", got)
	}
}

func TestWaitForRetriesUntilUp(t *testing.T) {
	calls := 0
	check := func(context.Context) error {
		calls++
		if calls < 4 {
			return errors.New("down")
		}
		return nil
	}
	if err := WaitFor(context.Background(), "dep", Backoff{Min: time.Millisecond, Max: 2 * time.Millisecond}, check); err != nil {
		t.Fatalf("WaitFor: %v", err)
	}
	if calls != 4 {
		t.Fatalf("calls = %d, want 4", calls)
	}
}

func TestWaitForStopsOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	err := WaitFor(ctx, "dep", Backoff{Min: time.Millisecond, Max: time.Millisecond}, func(context.Context) error { return errors.New("down") })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestBackoffZeroValueIsNotATightLoop(t *testing.T) {
	var b Backoff
	if got := b.Next(); got != Default.Min {
		t.Fatalf("zero Backoff first delay = %v, want %v", got, Default.Min)
	}
	neg := Backoff{Min: -time.Second}
	if got := neg.Next(); got <= 0 {
		t.Fatalf("negative Min gave %v", got)
	}
}

func get(h http.Handler) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Code
}

func TestReadinessOrderTimeoutAndDrain(t *testing.T) {
	up := true
	var order []string
	rd := &Readiness{Probes: []Probe{
		{"a", func(context.Context) error { order = append(order, "a"); return nil }},
		{"b", func(context.Context) error {
			order = append(order, "b")
			if up {
				return nil
			}
			return errors.New("down")
		}},
		{"c", func(ctx context.Context) error { order = append(order, "c"); return nil }},
	}}
	if got := get(rd); got != http.StatusOK {
		t.Fatalf("ready with deps up = %d", got)
	}
	if want := "abc"; strings.Join(order, "") != want {
		t.Fatalf("probe order %v, want %s", order, want)
	}
	up = false
	order = nil
	if got := get(rd); got != http.StatusServiceUnavailable {
		t.Fatalf("ready with b down = %d", got)
	}
	if strings.Join(order, "") != "ab" {
		t.Fatalf("probes after a failure ran: %v", order)
	}
	if got := get(http.HandlerFunc(Live)); got != http.StatusOK {
		t.Fatalf("live with a dep down = %d", got)
	}
	up = true
	rd.Drain()
	if got := get(rd); got != http.StatusServiceUnavailable {
		t.Fatalf("ready while draining = %d", got)
	}
}

func TestReadinessEachProbeHasItsOwnSecond(t *testing.T) {
	slow := func(ctx context.Context) error {
		select {
		case <-time.After(800 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	hung := func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	rd := &Readiness{Probes: []Probe{{"slow1", slow}, {"slow2", slow}}}
	if got := get(rd); got != http.StatusOK {
		t.Fatalf("two 0.8 s probes = %d, want 200 (each has its own budget)", got)
	}
	start := time.Now()
	rd = &Readiness{Probes: []Probe{{"hung", hung}}}
	if got := get(rd); got != http.StatusServiceUnavailable {
		t.Fatalf("hung probe = %d", got)
	}
	if d := time.Since(start); d > 1500*time.Millisecond {
		t.Fatalf("hung probe answered after %v, want about 1 s", d)
	}
}
