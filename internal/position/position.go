package position

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

const (
	Topic         = "position.reported"
	SchemaVersion = 1

	// MaxSeq keeps seq exact in Lua (double) and in the browser (Number).
	MaxSeq = 1<<53 - 1
	// MaxLat is the Redis geo index limit (Web Mercator), tighter than ±90.
	MaxLat = 85.05112878
	MaxLon = 180.0
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Validate checks what every writer of driver state relies on: ids that are
// safe inside Redis keys, a positive exact seq, and a point Redis can index.
func (r Reported) Validate() error {
	switch {
	case !idPattern.MatchString(r.FleetID):
		return errors.New("fleet_id: required, [A-Za-z0-9_-]{1,64}")
	case !idPattern.MatchString(r.DriverID):
		return errors.New("driver_id: required, [A-Za-z0-9_-]{1,64}")
	case r.Seq <= 0 || r.Seq > MaxSeq:
		return fmt.Errorf("seq: must be in 1..%d", int64(MaxSeq))
	case r.DeviceTime.IsZero():
		return errors.New("device_ts: required")
	case !(r.Lat >= -MaxLat && r.Lat <= MaxLat):
		return fmt.Errorf("lat: required, -%v..%v", MaxLat, MaxLat)
	case !(r.Lon >= -MaxLon && r.Lon <= MaxLon):
		return fmt.Errorf("lon: required, -%v..%v", MaxLon, MaxLon)
	}
	return nil
}

// EventID is derived from what identifies a report, so a device retry after an
// acked produce carries the same id.
func EventID(fleetID, driverID string, seq int64) string {
	return fmt.Sprintf("%s:%s:%d", fleetID, driverID, seq)
}

// Reported is the `position.reported` event as written to the log.
type Reported struct {
	EventID       string    `json:"event_id"`
	SchemaVersion int       `json:"schema_version"`
	FleetID       string    `json:"fleet_id"`
	DriverID      string    `json:"driver_id"`
	Seq           int64     `json:"seq"`
	DeviceTime    time.Time `json:"device_ts"`
	IngestTime    time.Time `json:"ingest_ts"`
	Lat           float64   `json:"lat"`
	Lon           float64   `json:"lon"`
}

// Key is the tenancy-scoped partition key.
func (r Reported) Key() string { return r.FleetID + ":" + r.DriverID }

// Update is what the location projector publishes to the live gateway after it
// accepts a position. Trace carries the W3C trace context.
type Update struct {
	DriverID string            `json:"driver_id"`
	Seq      int64             `json:"seq"`
	Lat      float64           `json:"lat"`
	Lon      float64           `json:"lon"`
	Trace    map[string]string `json:"trace,omitempty"`
}

// UpdatesChannel is the Redis pub/sub channel for a fleet's accepted updates.
func UpdatesChannel(fleetID string) string { return "fleet:{" + fleetID + "}:positions" }

const UpdatesPattern = "fleet:*:positions"
