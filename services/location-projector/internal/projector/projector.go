package projector

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Abdelwahab313/flotilla-stream/internal/position"
)

// GEOADD runs before HSET because Redis does not roll a script back: if the
// geo index refuses the point, nothing has been written yet.
// KEYS: driver hash, fleet geo set. ARGV: seq, lat, lon, device_ts, ingest_ts, event_id, driver_id, channel, update.
var applyScript = redis.NewScript(`
local cur = redis.call('HGET', KEYS[1], 'seq')
if cur and tonumber(cur) >= tonumber(ARGV[1]) then
  return 0
end
redis.call('GEOADD', KEYS[2], ARGV[3], ARGV[2], ARGV[7])
redis.call('HSET', KEYS[1], 'seq', ARGV[1], 'lat', ARGV[2], 'lon', ARGV[3], 'device_ts', ARGV[4], 'ingest_ts', ARGV[5], 'event_id', ARGV[6])
redis.call('PUBLISH', ARGV[8], ARGV[9])
return 1
`)

func DriverKey(fleetID, driverID string) string { return "fleet:{" + fleetID + "}:driver:" + driverID }

func GeoKey(fleetID string) string { return "fleet:{" + fleetID + "}:drivers:geo" }

type Store struct {
	RDB redis.Scripter
}

// Apply projects one position into the driver's latest state.
//
// Idempotency rule: the newest device sequence per driver wins. One Lua script
// compares the stored seq and, only if the incoming seq is strictly greater,
// writes the hash, the geo index and publishes the update, atomically. A
// duplicate or an older redelivery changes nothing and publishes nothing, so
// applying a batch twice converges to the same state. Device session (decision 4)
// is not part of the comparison until R1.
func (s Store) Apply(ctx context.Context, r position.Reported, trace map[string]string) (bool, error) {
	update, err := json.Marshal(position.Update{DriverID: r.DriverID, Seq: r.Seq, Lat: r.Lat, Lon: r.Lon, Trace: trace})
	if err != nil {
		return false, fmt.Errorf("encode update %s: %w", r.DriverID, err)
	}
	n, err := applyScript.Run(ctx, s.RDB,
		[]string{DriverKey(r.FleetID, r.DriverID), GeoKey(r.FleetID)},
		strconv.FormatInt(r.Seq, 10),
		strconv.FormatFloat(r.Lat, 'f', -1, 64),
		strconv.FormatFloat(r.Lon, 'f', -1, 64),
		r.DeviceTime.UTC().Format(time.RFC3339Nano),
		r.IngestTime.UTC().Format(time.RFC3339Nano),
		r.EventID,
		r.DriverID,
		position.UpdatesChannel(r.FleetID),
		update,
	).Int()
	if err != nil {
		return false, fmt.Errorf("project %s: %w", r.DriverID, err)
	}
	return n == 1, nil
}
