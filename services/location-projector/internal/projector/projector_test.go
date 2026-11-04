package projector

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/Abdelwahab313/flotilla-stream/internal/position"
)

func TestApplyMiniredis(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	testNewestWins(t, rdb, "test-"+strconv.FormatInt(time.Now().UnixNano(), 36))
	testRefusedPointWritesNothing(t, rdb, "test-"+strconv.FormatInt(time.Now().UnixNano(), 36))
}

func report(fleet string, seq int64) position.Reported {
	return position.Reported{
		EventID:    "e" + strconv.FormatInt(seq, 10),
		FleetID:    fleet,
		DriverID:   "D-1",
		Seq:        seq,
		DeviceTime: time.Date(2026, 11, 4, 12, 0, int(seq), 0, time.UTC),
		IngestTime: time.Date(2026, 11, 4, 12, 0, int(seq), 0, time.UTC),
		Lat:        30 + float64(seq)/1000,
		Lon:        31 + float64(seq)/1000,
	}
}

// testNewestWins runs against any Redis: miniredis in unit tests, the compose
// Redis under the integration tag.
func testNewestWins(t *testing.T, rdb *redis.Client, fleet string) {
	ctx := context.Background()
	s := Store{RDB: rdb}
	t.Cleanup(func() { rdb.Del(context.Background(), DriverKey(fleet, "D-1"), GeoKey(fleet)) })

	sub := rdb.Subscribe(ctx, position.UpdatesChannel(fleet))
	t.Cleanup(func() { _ = sub.Close() })
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	updates := sub.Channel()

	steps := []struct {
		name     string
		seq      int64
		accepted bool
		wantSeq  int64
	}{
		{"first", 5, true, 5},
		{"newer", 6, true, 6},
		{"duplicate", 6, false, 6},
		{"older after newer", 4, false, 6},
		{"redelivered batch: 5", 5, false, 6},
		{"redelivered batch: 6", 6, false, 6},
		{"skip ahead", 9, true, 9},
		{"late arrival in the gap", 8, false, 9},
	}
	var published []int64
	for _, st := range steps {
		accepted, err := s.Apply(ctx, report(fleet, st.seq), map[string]string{"traceparent": "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"})
		if err != nil {
			t.Fatalf("%s: apply: %v", st.name, err)
		}
		if accepted != st.accepted {
			t.Errorf("%s: accepted = %v, want %v", st.name, accepted, st.accepted)
		}
		if accepted {
			published = append(published, st.seq)
		}
		h, err := rdb.HGetAll(ctx, DriverKey(fleet, "D-1")).Result()
		if err != nil {
			t.Fatalf("%s: hgetall: %v", st.name, err)
		}
		want := report(fleet, st.wantSeq)
		if h["seq"] != strconv.FormatInt(want.Seq, 10) || h["event_id"] != want.EventID ||
			h["lat"] != strconv.FormatFloat(want.Lat, 'f', -1, 64) || h["lon"] != strconv.FormatFloat(want.Lon, 'f', -1, 64) {
			t.Errorf("%s: stored %v, want seq %d", st.name, h, want.Seq)
		}
		pos, err := rdb.GeoPos(ctx, GeoKey(fleet), "D-1").Result()
		if err != nil || len(pos) != 1 || pos[0] == nil {
			t.Fatalf("%s: geopos: %v %v", st.name, pos, err)
		}
		if d := pos[0].Latitude - want.Lat; d > 1e-5 || d < -1e-5 {
			t.Errorf("%s: geo lat %v, want %v", st.name, pos[0].Latitude, want.Lat)
		}
	}

	for _, wantSeq := range published {
		select {
		case m := <-updates:
			var u position.Update
			if err := json.Unmarshal([]byte(m.Payload), &u); err != nil {
				t.Fatalf("decode update: %v", err)
			}
			if u.Seq != wantSeq || u.DriverID != "D-1" || u.Trace["traceparent"] == "" {
				t.Errorf("update = %+v, want seq %d with trace", u, wantSeq)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("missing update for seq %d", wantSeq)
		}
	}
	select {
	case m := <-updates:
		t.Errorf("unexpected extra update %s", m.Payload)
	case <-time.After(100 * time.Millisecond):
	}
}

func testRefusedPointWritesNothing(t *testing.T, rdb *redis.Client, fleet string) {
	ctx := context.Background()
	s := Store{RDB: rdb}
	t.Cleanup(func() { rdb.Del(context.Background(), DriverKey(fleet, "D-1"), GeoKey(fleet)) })

	if _, err := s.Apply(ctx, report(fleet, 1), nil); err != nil {
		t.Fatalf("apply seq 1: %v", err)
	}
	r := report(fleet, 2)
	r.Lat = 86
	if _, err := s.Apply(ctx, r, nil); err == nil {
		t.Fatal("apply at lat 86: want the geo index to refuse it")
	}
	if seq := rdb.HGet(ctx, DriverKey(fleet, "D-1"), "seq").Val(); seq != "1" {
		t.Errorf("hash seq = %q after a refused point, want 1", seq)
	}
	pos := rdb.GeoPos(ctx, GeoKey(fleet), "D-1").Val()
	if len(pos) != 1 || pos[0] == nil || pos[0].Latitude > 31 {
		t.Errorf("geo point = %+v after a refused point, want seq 1's", pos)
	}
}
