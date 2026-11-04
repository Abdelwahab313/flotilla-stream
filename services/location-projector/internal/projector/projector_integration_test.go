//go:build integration

package projector

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestApplyRealRedis(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("redis at %s: %v (start it with docker compose up -d redis)", addr, err)
	}
	testNewestWins(t, rdb, "test-"+strconv.FormatInt(time.Now().UnixNano(), 36))
	testRefusedPointWritesNothing(t, rdb, "test-"+strconv.FormatInt(time.Now().UnixNano(), 36))
}
