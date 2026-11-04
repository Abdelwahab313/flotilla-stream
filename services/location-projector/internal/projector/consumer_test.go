package projector

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/Abdelwahab313/flotilla-stream/internal/position"
)

const testGroup = "projector-test"

type failingStore struct {
	Store
	failSeq int64
	calls   int
}

func (f *failingStore) Apply(ctx context.Context, r position.Reported, tc map[string]string) (bool, error) {
	f.calls++
	if r.Seq == f.failSeq {
		return false, errors.New("redis down")
	}
	return f.Store.Apply(ctx, r, tc)
}

type notifyStore struct {
	Store
	doneSeq int64
	done    chan struct{}
}

func (n *notifyStore) Apply(ctx context.Context, r position.Reported, tc map[string]string) (bool, error) {
	ok, err := n.Store.Apply(ctx, r, tc)
	if err == nil && r.Seq == n.doneSeq {
		close(n.done)
	}
	return ok, err
}

func groupClient(t *testing.T, addrs []string) *kgo.Client {
	t.Helper()
	kc, err := kgo.NewClient(
		kgo.SeedBrokers(addrs...),
		kgo.ConsumerGroup(testGroup),
		kgo.ConsumeTopics(position.Topic),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.FetchIsolationLevel(kgo.ReadCommitted()),
	)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return kc
}

func closeWithin(t *testing.T, kc *kgo.Client, d time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() { kc.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("client Close blocked for %v (rebalance not released?)", d)
	}
}

func TestConsumerFailureLeavesBatchUncommittedAndRerunConverges(t *testing.T) {
	ctx := context.Background()
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(1, position.Topic))
	if err != nil {
		t.Fatalf("kfake: %v", err)
	}
	t.Cleanup(cluster.Close)
	addrs := cluster.ListenAddrs()

	producer, err := kgo.NewClient(kgo.SeedBrokers(addrs...), kgo.DefaultProduceTopic(position.Topic))
	if err != nil {
		t.Fatalf("producer: %v", err)
	}
	defer producer.Close()
	for seq := int64(1); seq <= 5; seq++ {
		v, _ := json.Marshal(report("fleet-t", seq))
		if err := producer.ProduceSync(ctx, &kgo.Record{Key: []byte("fleet-t:D-1"), Value: v}).FirstErr(); err != nil {
			t.Fatalf("produce: %v", err)
		}
	}

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	sub := rdb.Subscribe(ctx, position.UpdatesChannel("fleet-t"))
	t.Cleanup(func() { _ = sub.Close() })
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	updates := sub.Channel()
	tracer := noop.NewTracerProvider().Tracer("test")

	failing := &failingStore{Store: Store{RDB: rdb}, failSeq: 3}
	kc := groupClient(t, addrs)
	c := &Consumer{Client: kc, Store: failing, Tracer: tracer, RetryDelays: []time.Duration{time.Millisecond, time.Millisecond}}
	runErr := make(chan error, 1)
	go func() { runErr <- c.Run(ctx) }()
	select {
	case err := <-runErr:
		if err == nil {
			t.Fatal("Run returned nil, want the apply error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after the store kept failing")
	}
	closeWithin(t, kc, 10*time.Second)
	if want := 2 + 3; failing.calls != want {
		t.Errorf("store calls = %d, want %d (2 applied, record 3 tried 3 times)", failing.calls, want)
	}

	adm := kadm.NewClient(producer)
	offs, err := adm.FetchOffsets(ctx, testGroup)
	if err != nil {
		t.Fatalf("fetch offsets: %v", err)
	}
	if o, ok := offs.Lookup(position.Topic, 0); ok && o.At > 2 {
		t.Fatalf("committed offset = %d, want at most 2 (record 3 failed and must be redelivered)", o.At)
	}

	working := &notifyStore{Store: Store{RDB: rdb}, doneSeq: 5, done: make(chan struct{})}
	kc2 := groupClient(t, addrs)
	rctx, cancel := context.WithCancel(ctx)
	c2 := &Consumer{Client: kc2, Store: working, Tracer: tracer}
	go func() { runErr <- c2.Run(rctx) }()
	select {
	case <-working.done:
	case <-time.After(10 * time.Second):
		t.Fatal("rerun did not reach seq 5")
	}
	cancel()
	if err := <-runErr; err != nil {
		t.Fatalf("rerun: %v", err)
	}
	closeWithin(t, kc2, 10*time.Second)

	if seq := mr.HGet(DriverKey("fleet-t", "D-1"), "seq"); seq != "5" {
		t.Errorf("stored seq = %q, want 5", seq)
	}
	if got, want := drain(t, updates), []int64{1, 2, 3, 4, 5}; !slices.Equal(got, want) {
		t.Fatalf("published %v, want exactly one update per accepted seq %v", got, want)
	}
}

func drain(t *testing.T, updates <-chan *redis.Message) []int64 {
	t.Helper()
	var seqs []int64
	for {
		select {
		case m := <-updates:
			var u position.Update
			if err := json.Unmarshal([]byte(m.Payload), &u); err != nil {
				t.Fatalf("decode update: %v", err)
			}
			seqs = append(seqs, u.Seq)
		case <-time.After(200 * time.Millisecond):
			return seqs
		}
	}
}

func TestConsumerSkipsInvalidRecords(t *testing.T) {
	ctx := context.Background()
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(1, position.Topic))
	if err != nil {
		t.Fatalf("kfake: %v", err)
	}
	t.Cleanup(cluster.Close)
	addrs := cluster.ListenAddrs()
	producer, err := kgo.NewClient(kgo.SeedBrokers(addrs...), kgo.DefaultProduceTopic(position.Topic))
	if err != nil {
		t.Fatalf("producer: %v", err)
	}
	defer producer.Close()

	bad := report("fleet-t", 1)
	bad.Lat = 86
	empty := report("", 2)
	empty.DriverID = ""
	good := report("fleet-t", 3)
	var values [][]byte
	for _, r := range []position.Reported{bad, empty} {
		v, _ := json.Marshal(r)
		values = append(values, v)
	}
	values = append(values, []byte("not json"))
	v, _ := json.Marshal(good)
	values = append(values, v)
	for _, v := range values {
		if err := producer.ProduceSync(ctx, &kgo.Record{Value: v}).FirstErr(); err != nil {
			t.Fatalf("produce: %v", err)
		}
	}

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := &notifyStore{Store: Store{RDB: rdb}, doneSeq: 3, done: make(chan struct{})}
	kc := groupClient(t, addrs)
	rctx, cancel := context.WithCancel(ctx)
	c := &Consumer{Client: kc, Store: store, Tracer: noop.NewTracerProvider().Tracer("test")}
	runErr := make(chan error, 1)
	go func() { runErr <- c.Run(rctx) }()
	select {
	case <-store.done:
	case <-time.After(10 * time.Second):
		t.Fatal("valid record after invalid ones was not applied")
	}
	cancel()
	if err := <-runErr; err != nil {
		t.Fatalf("run: %v", err)
	}
	closeWithin(t, kc, 10*time.Second)

	if c.skipped != 3 {
		t.Errorf("skipped = %d, want 3", c.skipped)
	}
	keys := mr.Keys()
	for _, k := range keys {
		if k != DriverKey("fleet-t", "D-1") && k != GeoKey("fleet-t") {
			t.Errorf("unexpected key written: %q", k)
		}
	}
	if seq := mr.HGet(DriverKey("fleet-t", "D-1"), "seq"); seq != "3" {
		t.Errorf("stored seq = %q, want 3", seq)
	}
}
