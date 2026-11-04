package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func serve(t *testing.T, frames ...string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := c.CloseRead(r.Context())
		for _, f := range frames {
			if err := c.Write(ctx, websocket.MessageText, []byte(f)); err != nil {
				return
			}
		}
		<-ctx.Done()
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/ws"
}

func TestProbe(t *testing.T) {
	tests := []struct {
		name    string
		frames  []string
		n       int
		driver  string
		wantErr string
	}{
		{"two rising", []string{`{"driver_id":"D-1","seq":1,"lat":30,"lon":31}`, `{"driver_id":"D-1","seq":2,"lat":30,"lon":31}`}, 2, "", ""},
		{"array frame", []string{`[{"driver_id":"D-1","seq":5,"lat":0,"lon":0},{"driver_id":"D-1","seq":9,"lat":0,"lon":0}]`}, 2, "", ""},
		{"other drivers ignored", []string{`{"driver_id":"D-2","seq":100,"lat":0,"lon":0}`, `{"driver_id":"D-1","seq":1,"lat":0,"lon":0}`, `{"driver_id":"D-2","seq":1,"lat":0,"lon":0}`, `{"driver_id":"D-1","seq":2,"lat":0,"lon":0}`}, 2, "D-1", ""},
		{"duplicate seq", []string{`{"driver_id":"D-1","seq":1,"lat":0,"lon":0}`, `{"driver_id":"D-1","seq":1,"lat":0,"lon":0}`}, 2, "", "seq went from 1 to 1"},
		{"seq backwards", []string{`{"driver_id":"D-1","seq":3,"lat":0,"lon":0}`, `{"driver_id":"D-1","seq":2,"lat":0,"lon":0}`}, 2, "", "seq went from 3 to 2"},
		{"too few before timeout", []string{`{"driver_id":"D-1","seq":1,"lat":0,"lon":0}`}, 2, "", "timed out with 1 of 2 rising messages for driver D-1"},
		{"nothing arrives", nil, 2, "", "timed out with no position messages"},
		{"not a position", []string{`{"hello":1}`}, 1, "", "missing driver_id or seq"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			err := probe(ctx, serve(t, tt.frames...), tt.n, tt.driver, io.Discard)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("probe: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestProbeDialFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := probe(ctx, "http://127.0.0.1:1/ws", 2, "", io.Discard); err == nil || !strings.Contains(err.Error(), "dial") {
		t.Fatalf("err = %v, want a dial error", err)
	}
}
