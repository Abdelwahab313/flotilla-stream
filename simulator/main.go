package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/Abdelwahab313/flotilla-stream/internal/telemetry"
)

const (
	service      = "simulator"
	centreLat    = 30.0444
	centreLon    = 31.2357
	radiusDeg    = 0.004
	stepsPerLap  = 120
	reportPeriod = time.Second
)

type report struct {
	FleetID  string    `json:"fleet_id"`
	DriverID string    `json:"driver_id"`
	Seq      int64     `json:"seq"`
	DeviceTS time.Time `json:"device_ts"`
	Lat      float64   `json:"lat"`
	Lon      float64   `json:"lon"`
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
	defer shutdownTracing()

	url := getenv("INGEST_URL", "http://127.0.0.1:8081/v1/positions")
	token := getenv("INGEST_TOKEN", "dev-token")
	fleet := getenv("FLEET_ID", "fleet-demo")
	driver := getenv("DRIVER_ID", "D-118")

	hc := &http.Client{Timeout: 5 * time.Second, Transport: otelhttp.NewTransport(http.DefaultTransport)}
	tracer := otel.Tracer(service)

	// Until device sessions arrive (decision 4, R1), the sequence starts at boot
	// time so a restarted simulator is not held behind its previous run's seq.
	seq := time.Now().UnixMilli()
	slog.Info("reporting", "url", url, "fleet_id", fleet, "driver_id", driver, "first_seq", seq)

	tick := time.NewTicker(reportPeriod)
	defer tick.Stop()
	for step := 0; ; step++ {
		angle := 2 * math.Pi * float64(step%stepsPerLap) / stepsPerLap
		r := report{
			FleetID:  fleet,
			DriverID: driver,
			Seq:      seq,
			DeviceTS: time.Now().UTC(),
			Lat:      centreLat + radiusDeg*math.Sin(angle),
			Lon:      centreLon + radiusDeg*math.Cos(angle)/math.Cos(centreLat*math.Pi/180),
		}
		if err := send(ctx, hc, tracer, url, token, r); err != nil {
			slog.Warn("report failed", "seq", r.Seq, "err", err)
		}
		seq++
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

func send(ctx context.Context, hc *http.Client, tracer trace.Tracer, url, token string, r report) error {
	ctx, span := tracer.Start(ctx, "simulator.report", trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("driver.id", r.DriverID), attribute.Int64("position.seq", r.Seq)))
	defer span.End()

	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		span.SetStatus(codes.Error, "post")
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		span.SetStatus(codes.Error, resp.Status)
		return fmt.Errorf("ingest answered %s", resp.Status)
	}
	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
