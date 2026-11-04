// Command wsprobe asserts that a live gateway socket delivers moving positions:
// at least n messages with strictly rising seq for one driver within a timeout.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/coder/websocket"
)

type update struct {
	DriverID string  `json:"driver_id"`
	Seq      *int64  `json:"seq"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
}

func main() {
	url := flag.String("url", "ws://127.0.0.1:18080/ws", "live gateway socket (ws, wss, http or https)")
	n := flag.Int("n", 2, "messages with strictly rising seq required")
	driver := flag.String("driver", "", "driver id to follow (default: the first one seen)")
	timeout := flag.Duration("timeout", 30*time.Second, "give up after")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := probe(ctx, *url, *n, *driver, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "wsprobe: FAIL:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "wsprobe: OK")
}

func probe(ctx context.Context, url string, n int, driver string, out io.Writer) error {
	if n < 1 {
		return errors.New("-n must be at least 1")
	}
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", url, err)
	}
	defer c.CloseNow()

	var last int64
	got := 0
	for got < n {
		_, b, err := c.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				if driver == "" {
					return fmt.Errorf("timed out with no position messages (wanted %d)", n)
				}
				return fmt.Errorf("timed out with %d of %d rising messages for driver %s", got, n, driver)
			}
			return fmt.Errorf("read after %d of %d messages: %w", got, n, err)
		}
		ups, err := decode(b)
		if err != nil {
			return fmt.Errorf("decode %q: %w", b, err)
		}
		for _, u := range ups {
			if driver == "" {
				driver = u.DriverID
			}
			if u.DriverID != driver {
				continue
			}
			if got > 0 && *u.Seq <= last {
				return fmt.Errorf("driver %s seq went from %d to %d", driver, last, *u.Seq)
			}
			last = *u.Seq
			got++
			fmt.Fprintf(out, "%s driver=%s seq=%d lat=%.6f lon=%.6f\n", time.Now().Format("15:04:05.000"), u.DriverID, *u.Seq, u.Lat, u.Lon)
		}
	}
	_ = c.Close(websocket.StatusNormalClosure, "")
	return nil
}

func decode(b []byte) ([]update, error) {
	var ups []update
	if len(b) > 0 && b[0] == '[' {
		if err := json.Unmarshal(b, &ups); err != nil {
			return nil, err
		}
	} else {
		var u update
		if err := json.Unmarshal(b, &u); err != nil {
			return nil, err
		}
		ups = []update{u}
	}
	for _, u := range ups {
		if u.DriverID == "" || u.Seq == nil {
			return nil, errors.New("missing driver_id or seq")
		}
	}
	return ups, nil
}
