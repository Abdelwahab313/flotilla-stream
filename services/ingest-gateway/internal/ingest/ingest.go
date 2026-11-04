package ingest

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Abdelwahab313/flotilla-stream/internal/position"
)

const maxBody = 4 << 10

type Producer interface {
	Produce(ctx context.Context, r position.Reported) error
}

// DefaultProduceTimeout is shorter than a device's request timeout, so the 503
// and Retry-After reach a client that is still waiting.
const DefaultProduceTimeout = 3 * time.Second

type Handler struct {
	Token          string
	Producer       Producer
	Now            func() time.Time
	ProduceTimeout time.Duration
}

type report struct {
	FleetID  string    `json:"fleet_id"`
	DriverID string    `json:"driver_id"`
	Seq      int64     `json:"seq"`
	DeviceTS time.Time `json:"device_ts"`
	Lat      *float64  `json:"lat"`
	Lon      *float64  `json:"lon"`
}

var errTrailingData = errors.New("trailing data after the report")

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	in, err := decode(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	ev, err := normalize(in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ev.IngestTime = h.Now().UTC()
	timeout := h.ProduceTimeout
	if timeout <= 0 {
		timeout = DefaultProduceTimeout
	}
	pctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	if err := h.Producer.Produce(pctx, ev); err != nil {
		slog.ErrorContext(r.Context(), "produce failed", "driver_id", ev.DriverID, "err", err)
		w.Header().Set("Retry-After", "1")
		http.Error(w, "log unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"event_id": ev.EventID})
}

func (h *Handler) authorized(r *http.Request) bool {
	got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return ok && subtle.ConstantTimeCompare([]byte(got), []byte(h.Token)) == 1
}

func decode(body io.Reader) (report, error) {
	var in report
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return in, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = errTrailingData
		}
		return in, err
	}
	return in, nil
}

func normalize(in report) (position.Reported, error) {
	if in.Lat == nil {
		return position.Reported{}, errors.New("lat: required")
	}
	if in.Lon == nil {
		return position.Reported{}, errors.New("lon: required")
	}
	ev := position.Reported{
		SchemaVersion: position.SchemaVersion,
		FleetID:       strings.TrimSpace(in.FleetID),
		DriverID:      strings.TrimSpace(in.DriverID),
		Seq:           in.Seq,
		DeviceTime:    in.DeviceTS.UTC(),
		Lat:           *in.Lat,
		Lon:           *in.Lon,
	}
	if err := ev.Validate(); err != nil {
		return position.Reported{}, err
	}
	ev.EventID = position.EventID(ev.FleetID, ev.DriverID, ev.Seq)
	return ev, nil
}
