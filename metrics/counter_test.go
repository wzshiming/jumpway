package metrics

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzshiming/jumpway/config"
)

func TestTick(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	hopCount := rule.HopWrapper(Forward)(0, "socks5://hop:1080", &net.Dialer{}).(*dialer).count
	idle := rule.HopWrapper(Listen)(0, "ssh://idle:22", &net.Dialer{}).(*dialer).count
	targetCount := rule.target(targetKey{address: "target:443"})
	start := time.Unix(100, 0)
	registry.tick(start)
	for _, test := range []struct {
		name  string
		count *counter
		up    int64
		down  int64
	}{
		{name: "rule", count: rule.count, up: 120, down: 80},
		{name: "hop", count: hopCount, up: 60, down: 40},
		{name: "target", count: targetCount, up: 30, down: 20},
		{name: "idle", count: idle},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.count.up.Add(test.up)
			test.count.down.Add(test.down)
			registry.tick(start.Add(time.Second))
			stats := test.count.snapshot().stats
			if stats.RateUp != test.up || stats.RateDown != test.down {
				t.Fatalf("1s rates = (%d, %d), want (%d, %d)", stats.RateUp, stats.RateDown, test.up, test.down)
			}
			test.count.up.Add(test.up)
			test.count.down.Add(test.down)
			registry.tick(start.Add(3 * time.Second))
			stats = test.count.snapshot().stats
			if stats.RateUp != test.up/2 || stats.RateDown != test.down/2 {
				t.Fatalf("2s rates = (%d, %d), want (%d, %d)", stats.RateUp, stats.RateDown, test.up/2, test.down/2)
			}
			registry.tick(start.Add(4 * time.Second))
			stats = test.count.snapshot().stats
			if stats.RateUp != 0 || stats.RateDown != 0 {
				t.Fatalf("idle rates = (%d, %d)", stats.RateUp, stats.RateDown)
			}
			start = start.Add(4 * time.Second)
		})
	}
}

func TestCounterPeakRate(t *testing.T) {
	start := time.Unix(100, 0)
	count := newCounter(start)
	count.up.Add(100)
	count.down.Add(80)
	if count.peakUp.Load() != 0 || count.peakDown.Load() != 0 {
		t.Fatal("incomplete window raised peaks")
	}
	for index, window := range []struct {
		up   int64
		down int64
	}{
		{up: 100, down: 80},
		{up: 20, down: 10},
		{},
	} {
		if index > 0 {
			count.up.Add(window.up)
			count.down.Add(window.down)
		}
		count.tick(start.Add(time.Duration(index+1) * time.Second))
		if up, down := count.rateUp.Load(), count.rateDown.Load(); up != window.up || down != window.down {
			t.Fatalf("window %d rates = (%d, %d), want (%d, %d)", index, up, down, window.up, window.down)
		}
		if up, down := count.peakUp.Load(), count.peakDown.Load(); up != 100 || down != 80 {
			t.Fatalf("window %d peaks = (%d, %d), want (100, 80)", index, up, down)
		}
	}
	count.reset(start.Add(3 * time.Second))
	if count.peakUp.Load() != 0 || count.peakDown.Load() != 0 {
		t.Fatal("reset retained peaks")
	}
	count.up.Add(7)
	count.down.Add(11)
	count.tick(start.Add(4 * time.Second))
	if up, down := count.peakUp.Load(), count.peakDown.Load(); up != 7 || down != 11 {
		t.Fatalf("post-reset peaks = (%d, %d), want (7, 11)", up, down)
	}
}

func TestCounterLastTransfer(t *testing.T) {
	start := time.Unix(100, 123)
	count := newCounter(start)
	lastUp := start.Add(2 * time.Second).UnixNano()
	lastDown := start.Add(time.Second).UnixNano()
	count.addBytes(true, 100, lastUp)
	count.addBytes(false, 80, lastDown)
	count.addBytes(true, 20, start.UnixNano())
	count.addBytes(false, 10, start.UnixNano())
	if count.up.Load() != 120 || count.down.Load() != 90 {
		t.Fatal("byte totals did not include out-of-order transfers")
	}
	if count.lastUp.Load() != lastUp || count.lastDown.Load() != lastDown || count.lastActive.Load() != lastUp {
		t.Fatalf("last transfers = (%d, %d), activity = %d, want (%d, %d), %d", count.lastUp.Load(), count.lastDown.Load(), count.lastActive.Load(), lastUp, lastDown, lastUp)
	}
	count.reset(start.Add(3 * time.Second))
	if count.lastUp.Load() != 0 || count.lastDown.Load() != 0 {
		t.Fatal("reset retained transfer timestamps")
	}
}

func TestReset(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	hopCount := rule.HopWrapper(Forward)(0, "ssh://hop:22", &net.Dialer{}).(*dialer).count
	targetCount := rule.target(targetKey{address: "target:443"})
	for _, count := range []*counter{rule.count, hopCount, targetCount} {
		count.up.Store(20)
		count.down.Store(40)
		count.active.Store(1)
		count.total.Store(2)
		count.dial(5*time.Millisecond, time.Now(), nil)
	}
	rule.targetsEvicted = 3
	registry.since = time.Unix(1, 0)
	registry.tick(time.Now().Add(time.Second))
	before := time.Now()
	registry.Reset()
	if registry.since.Before(before) || registry.since.After(time.Now()) {
		t.Fatal("reset did not update since")
	}
	if registry.Rule("rule") != rule {
		t.Fatal("reset replaced rule handle")
	}
	snapshot := registry.Snapshot().Rules[0]
	if snapshot.Stats != (Stats{}) || snapshot.Forward[0].Stats != (Stats{}) || snapshot.Forward[0].URLs[0].Stats != (Stats{}) {
		t.Fatalf("reset counters = %+v", snapshot)
	}
	if len(snapshot.Targets) != 0 || snapshot.Targets == nil || snapshot.TargetsEvicted != 0 {
		t.Fatal("reset did not clear target state")
	}
	if count := rule.HopWrapper(Forward)(0, "ssh://hop:22", &net.Dialer{}).(*dialer).count; count != hopCount {
		t.Fatal("reset detached existing hop wrappers")
	}
	rule.count.up.Add(100)
	hopCount.down.Add(80)
	registry.tick(registry.lastTick.Add(time.Second))
	snapshot = registry.Snapshot().Rules[0]
	if snapshot.Stats.RateUp != 100 || snapshot.Forward[0].Stats.RateDown != 80 {
		t.Fatal("reset retained previous rate samples")
	}
}

func TestResetWithOpenConnections(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	oldInner, oldPeer := net.Pipe()
	closeOnCleanup(t, oldPeer)
	old, err := rule.WrapListener(&pipeListener{connection: oldInner}).Accept()
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, old)
	registry.Reset()
	if registry.Snapshot().Rules[0].Stats.Active != 0 {
		t.Fatal("reset did not zero active count")
	}
	newInner, newPeer := net.Pipe()
	closeOnCleanup(t, newPeer)
	current, err := rule.WrapListener(&pipeListener{connection: newInner}).Accept()
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, current)
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	if registry.Snapshot().Rules[0].Stats.Active != 1 {
		t.Fatal("old close decremented a post-reset connection")
	}
	if err := current.Close(); err != nil {
		t.Fatal(err)
	}
	if registry.Snapshot().Rules[0].Stats.Active != 0 {
		t.Fatal("new close did not return active count to zero")
	}
}

func TestRunCanceled(t *testing.T) {
	registry := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		registry.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

func TestConcurrentTraffic(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	const workers = 8
	const writes = 100
	const payload = "measured payload"
	start := make(chan struct{})
	done := make(chan struct{})
	var group sync.WaitGroup
	for range workers {
		inner, peer := net.Pipe()
		closeOnCleanup(t, peer)
		wrapped, err := rule.WrapListener(&pipeListener{connection: inner}).Accept()
		if err != nil {
			t.Fatal(err)
		}
		closeOnCleanup(t, wrapped)
		setDeadlines(t, wrapped, peer)
		group.Go(func() {
			<-start
			if _, err := io.Copy(io.Discard, peer); err != nil {
				t.Error(err)
			}
		})
		group.Go(func() {
			<-start
			for range writes {
				if _, err := wrapped.Write([]byte(payload)); err != nil {
					t.Error(err)
					break
				}
			}
			if err := wrapped.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	var samples atomic.Int64
	observerDone := make(chan struct{})
	go func() {
		defer close(observerDone)
		<-start
		now := time.Now()
		for {
			registry.tick(now)
			registry.Snapshot()
			samples.Add(1)
			now = now.Add(time.Second)
			select {
			case <-done:
				return
			default:
			}
		}
	}()
	close(start)
	group.Wait()
	close(done)
	<-observerDone
	stats := registry.Snapshot().Rules[0].Stats
	if stats.Up != 0 || stats.Down != int64(workers*writes*len(payload)) || stats.Active != 0 || stats.Total != workers || samples.Load() == 0 {
		t.Fatalf("concurrent traffic = %+v, samples = %d", stats, samples.Load())
	}
}

func TestCounterLatestEvent(t *testing.T) {
	count := &counter{}
	older := time.Unix(100, 0)
	newer := older.Add(time.Second)
	count.dial(2*time.Millisecond, newer, nil)
	count.dial(8*time.Millisecond, older, nil)
	stats := count.snapshot().stats
	if stats.LatencyMs != 2 || stats.AvgLatencyMs != 5 {
		t.Fatalf("out-of-order dial latency = %+v", stats)
	}
	if want := newer.UTC().Format(time.RFC3339Nano); stats.LastActive != want {
		t.Fatalf("out-of-order activity = %q, want %q", stats.LastActive, want)
	}
}
