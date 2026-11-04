package ingest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Abdelwahab313/flotilla-stream/internal/position"
)

type fakeProducer struct {
	got []position.Reported
	err error
}

func (f *fakeProducer) Produce(_ context.Context, r position.Reported) error {
	if f.err != nil {
		return f.err
	}
	f.got = append(f.got, r)
	return nil
}

var fixedNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

func newHandler(p Producer) *Handler {
	return &Handler{Token: "test-token", Producer: p, Now: func() time.Time { return fixedNow }}
}

func post(h http.Handler, auth, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/positions", strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandlerStatus(t *testing.T) {
	const good = `{"fleet_id":"fleet-demo","driver_id":"D-118","seq":7,"device_ts":"2026-11-04T13:59:58+02:00","lat":30.0444,"lon":31.2357}`
	tests := []struct {
		name string
		auth string
		body string
		want int
	}{
		{"valid", "Bearer test-token", good, http.StatusAccepted},
		{"no token", "", good, http.StatusUnauthorized},
		{"wrong token", "Bearer nope", good, http.StatusUnauthorized},
		{"wrong scheme", "Basic test-token", good, http.StatusUnauthorized},
		{"not json", "Bearer test-token", `nope`, http.StatusBadRequest},
		{"unknown field", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1,"x":1}`, http.StatusBadRequest},
		{"missing fleet", "Bearer test-token", `{"driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1}`, http.StatusBadRequest},
		{"fleet with key separator", "Bearer test-token", `{"fleet_id":"f:x","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1}`, http.StatusBadRequest},
		{"missing driver", "Bearer test-token", `{"fleet_id":"f","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1}`, http.StatusBadRequest},
		{"driver with hash tag", "Bearer test-token", `{"fleet_id":"f","driver_id":"{d}","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1}`, http.StatusBadRequest},
		{"seq zero", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":0,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1}`, http.StatusBadRequest},
		{"seq negative", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":-3,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1}`, http.StatusBadRequest},
		{"seq above 2^53", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":9007199254740992,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1}`, http.StatusBadRequest},
		{"missing device_ts", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":1,"lat":1,"lon":1}`, http.StatusBadRequest},
		{"missing lat", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lon":1}`, http.StatusBadRequest},
		{"lat 90.1", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":90.1,"lon":1}`, http.StatusBadRequest},
		{"lat 86 above redis geo limit", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":86,"lon":1}`, http.StatusBadRequest},
		{"lat -85.06 below redis geo limit", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":-85.06,"lon":1}`, http.StatusBadRequest},
		{"lat at redis geo limit", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":85.05112878,"lon":1}`, http.StatusAccepted},
		{"two json values", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1}{"seq":2}`, http.StatusBadRequest},
		{"trailing garbage", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1} x`, http.StatusBadRequest},
		{"trailing whitespace", "Bearer test-token", "{\"fleet_id\":\"f\",\"driver_id\":\"d\",\"seq\":1,\"device_ts\":\"2026-11-04T12:00:00Z\",\"lat\":1,\"lon\":1}\n ", http.StatusAccepted},
		{"lon -180.1", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":-180.1}`, http.StatusBadRequest},
		{"lat 0 lon 0 edges", "Bearer test-token", `{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":0,"lon":180}`, http.StatusAccepted},
		{"body too large", "Bearer test-token", `{"fleet_id":"` + strings.Repeat("a", maxBody) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &fakeProducer{}
			rec := post(newHandler(p), tt.auth, tt.body)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.want, rec.Body.String())
			}
			if wantProduced := tt.want == http.StatusAccepted; (len(p.got) == 1) != wantProduced {
				t.Fatalf("produced %d records, want produced=%v", len(p.got), wantProduced)
			}
		})
	}
}

func TestHandlerNormalizes(t *testing.T) {
	p := &fakeProducer{}
	rec := post(newHandler(p), "Bearer test-token",
		`{"fleet_id":" fleet-demo ","driver_id":"D-118 ","seq":7,"device_ts":"2026-11-04T13:59:58+02:00","lat":30.0444,"lon":31.2357}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	got := p.got[0]
	if got.FleetID != "fleet-demo" || got.DriverID != "D-118" || got.Key() != "fleet-demo:D-118" {
		t.Errorf("ids not trimmed: %+v", got)
	}
	if want := time.Date(2026, 11, 4, 11, 59, 58, 0, time.UTC); !got.DeviceTime.Equal(want) || got.DeviceTime.Location() != time.UTC {
		t.Errorf("device time = %v, want %v in UTC", got.DeviceTime, want)
	}
	if !got.IngestTime.Equal(fixedNow) {
		t.Errorf("ingest time = %v, want %v", got.IngestTime, fixedNow)
	}
	if got.EventID != "fleet-demo:D-118:7" || got.SchemaVersion != position.SchemaVersion || got.Seq != 7 {
		t.Errorf("envelope incomplete: %+v", got)
	}
}

func TestEventIDIsStableAcrossRetries(t *testing.T) {
	p := &fakeProducer{}
	h := newHandler(p)
	body := func(seq string) string {
		return `{"fleet_id":"f","driver_id":"d","seq":` + seq + `,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1}`
	}
	for _, b := range []string{body("1"), body("1"), body("2")} {
		if rec := post(h, "Bearer test-token", b); rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d", rec.Code)
		}
	}
	if p.got[0].EventID != p.got[1].EventID {
		t.Errorf("retry changed event id: %q vs %q", p.got[0].EventID, p.got[1].EventID)
	}
	if p.got[0].EventID == p.got[2].EventID {
		t.Errorf("different seq shares event id %q", p.got[0].EventID)
	}
}

func TestHandlerProduceFailure(t *testing.T) {
	p := &fakeProducer{err: errors.New("broker down")}
	rec := post(newHandler(p), "Bearer test-token",
		`{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1}`)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("status = %d retry-after = %q, want 503 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func FuzzHandler(f *testing.F) {
	f.Add(`{"fleet_id":"f","driver_id":"d","seq":1,"device_ts":"2026-11-04T12:00:00Z","lat":1,"lon":1}`)
	f.Add(`{"seq":1e400}`)
	f.Add(`[]`)
	f.Fuzz(func(t *testing.T, body string) {
		p := &fakeProducer{}
		rec := post(newHandler(p), "Bearer test-token", body)
		switch rec.Code {
		case http.StatusAccepted:
			r := p.got[0]
			if err := r.Validate(); err != nil {
				t.Fatalf("accepted invalid report %+v: %v", r, err)
			}
		case http.StatusBadRequest, http.StatusRequestEntityTooLarge:
		default:
			t.Fatalf("unexpected status %d", rec.Code)
		}
	})
}
