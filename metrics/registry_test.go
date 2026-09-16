package metrics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/jumpway/config"
)

func TestRegistrySync(t *testing.T) {
	registry := NewRegistry()
	if registry.Rule("unknown") != nil {
		t.Fatal("unknown rule should be nil")
	}
	const kept = "ssh://user:secret@exit:22"
	const removed = "socks5://removed:1080"
	const entry = "socks5://entry:1080"
	rules := []config.Rule{
		{Name: "first", Listen: config.Listen{Way: []bridgeconfig.Node{{LB: []string{kept}}}}, Forward: config.Forward{Way: []bridgeconfig.Node{
			{LB: []string{removed, kept}},
			{LB: []string{"socks5://middle:1080"}},
			{LB: []string{entry}},
		}}},
		{Name: "second"},
	}
	registry.Sync(rules)
	first := registry.Rule("first")
	first.count.total.Store(7)
	first.count.up.Store(50)
	forward := first.HopWrapper(Forward)(0, kept, &net.Dialer{}).(*dialer).count
	forward.total.Store(3)
	forward.up.Store(12)
	listening := first.HopWrapper(Listen)(0, kept, &net.Dialer{}).(*dialer).count
	listening.up.Store(4)
	initial := registry.Snapshot()
	if len(initial.Rules) != 2 || initial.Rules[0].Name != "first" || initial.Rules[1].Name != "second" {
		t.Fatalf("wrong config order: %+v", initial.Rules)
	}
	for index, parent := range []int{1, 2, -1} {
		hop := initial.Rules[0].Forward[index]
		if hop.Index != index || hop.ParentIndex != parent {
			t.Fatalf("hop %d = %+v, want parent %d", index, hop, parent)
		}
	}
	if got := initial.Rules[0].Forward[0].URLs; got[0].URL != removed || got[1].URL != "ssh://user:xxxxx@exit:22" {
		t.Fatalf("URL order/redaction = %+v", got)
	}
	if initial.Rules[0].Listen[0].Stats.Up != 4 || initial.Rules[0].Forward[0].Stats.Up != 12 {
		t.Fatal("listen and forward counters were mixed")
	}
	rules[0].Forward.Way = []bridgeconfig.Node{
		{LB: []string{"ssh://user:new-password@exit:22", "socks5://new:1080"}},
		{LB: []string{entry}},
	}
	registry.Sync([]config.Rule{rules[1], rules[0]})
	if registry.Rule("first") != first {
		t.Fatal("rule pointer was not retained")
	}
	updated := registry.Snapshot()
	if updated.Rules[0].Name != "second" || updated.Rules[1].Name != "first" {
		t.Fatal("reload did not update rule order")
	}
	stats := updated.Rules[1]
	if stats.Stats.Total != 7 || stats.Stats.Up != 50 {
		t.Fatal("rule counters were lost")
	}
	if len(stats.Forward) != 2 || len(stats.Forward[0].URLs) != 2 || stats.Forward[0].URLs[0].Stats.Total != 3 {
		t.Fatalf("matching URL was not retained: %+v", stats.Forward)
	}
	if stats.Forward[0].URLs[1].Stats != (Stats{}) || stats.Forward[1].URLs[0].Stats != (Stats{}) {
		t.Fatal("new or moved URL inherited counters")
	}
	if len(initial.Rules[0].Forward) != 3 || initial.Rules[0].Forward[0].URLs[0].URL != removed {
		t.Fatal("reload mutated an existing snapshot")
	}
	rules[0].Name = "renamed"
	registry.Sync([]config.Rule{rules[0]})
	if registry.Rule("first") != nil || registry.Rule("second") != nil {
		t.Fatal("removed rules remain in registry")
	}
	if got := registry.Snapshot().Rules[0]; got.Name != "renamed" || got.Stats != (Stats{}) {
		t.Fatalf("renamed rule inherited counters: %+v", got)
	}
}

// Counters created or reset late in a window must not extrapolate their first rate (and peak) from a partial window.
func TestRegistryCountersMeasureFromLastTick(t *testing.T) {
	const url = "ssh://u:secret@first:22"
	registry := NewRegistry()
	start := time.Unix(100, 0)
	registry.tick(start)
	registry.Sync([]config.Rule{{Name: "rule", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{url}}}}}})
	rule := registry.Rule("rule")
	registry.mu.Lock()
	target := rule.target(targetKey{address: "example.com:443"})
	registry.mu.Unlock()
	hop := rule.HopWrapper(Forward)(0, url, &net.Dialer{}).(*dialer).count
	counters := map[string]*counter{"rule": rule.count, "target": target, "hop": hop}
	stats := func() map[string]Stats {
		snapshot := registry.Snapshot().Rules[0]
		return map[string]Stats{"rule": snapshot.Stats, "target": snapshot.Targets[0].Stats, "hop": snapshot.Forward[0].Stats}
	}
	for _, count := range counters {
		count.addBytes(true, 100, start.Add(900*time.Millisecond).UnixNano())
	}
	registry.tick(start.Add(time.Second))
	for name, current := range stats() {
		if current.RateUp != 100 || current.PeakRateUp != 100 {
			t.Fatalf("%s after creation: rate/peak = (%d, %d), want (100, 100)", name, current.RateUp, current.PeakRateUp)
		}
	}
	registry.Reset()
	registry.mu.Lock()
	target = rule.target(targetKey{address: "example.com:443"})
	registry.mu.Unlock()
	counters["target"] = target
	for _, count := range counters {
		count.addBytes(false, 50, start.Add(1900*time.Millisecond).UnixNano())
	}
	registry.tick(start.Add(2 * time.Second))
	for name, current := range stats() {
		if current.RateDown != 50 || current.PeakRateDown != 50 || current.PeakRateUp != 0 {
			t.Fatalf("%s after reset: rate/peak down = (%d, %d), peak up = %d, want (50, 50, 0)", name, current.RateDown, current.PeakRateDown, current.PeakRateUp)
		}
	}
}

func TestRegistryHopPeak(t *testing.T) {
	for role, wayName := range []string{"listen", "forward"} {
		for _, test := range []struct {
			name       string
			secondURL  string
			sameWindow bool
			peakUp     int64
			peakDown   int64
		}{
			{name: "different windows", secondURL: "ssh://u:secret@second:22", peakUp: 100, peakDown: 80},
			{name: "same window", secondURL: "ssh://u:secret@second:22", sameWindow: true, peakUp: 200, peakDown: 160},
			{name: "duplicate redacted URLs", secondURL: "ssh://u:other@first:22", peakUp: 100, peakDown: 80},
		} {
			t.Run(wayName+"/"+test.name, func(t *testing.T) {
				const firstURL = "ssh://u:secret@first:22"
				way := []bridgeconfig.Node{{LB: []string{firstURL, test.secondURL}}}
				rules := []config.Rule{{Name: "rule"}}
				if Role(role) == Listen {
					rules[0].Listen.Way = way
				} else {
					rules[0].Forward.Way = way
				}
				registry := NewRegistry()
				registry.Sync(rules)
				rule := registry.Rule("rule")
				first := rule.HopWrapper(Role(role))(0, firstURL, &net.Dialer{}).(*dialer).count
				second := rule.HopWrapper(Role(role))(0, test.secondURL, &net.Dialer{}).(*dialer).count
				start := time.Unix(100, 0)
				registry.tick(start)
				first.addBytes(true, 100, start.Add(100*time.Millisecond).UnixNano())
				first.addBytes(false, 80, start.Add(900*time.Millisecond).UnixNano())
				if !test.sameWindow {
					registry.tick(start.Add(time.Second))
				}
				if first != second {
					second.addBytes(true, 100, start.Add(800*time.Millisecond).UnixNano())
					second.addBytes(false, 80, start.Add(200*time.Millisecond).UnixNano())
				}
				end := start.Add(time.Second)
				if !test.sameWindow {
					end = end.Add(time.Second)
				}
				registry.tick(end)
				getHop := func() Hop {
					snapshot := registry.Snapshot().Rules[0]
					if Role(role) == Listen {
						return snapshot.Listen[0]
					}
					return snapshot.Forward[0]
				}
				assertPeak := func() {
					t.Helper()
					stats := getHop().Stats
					if stats.PeakRateUp != test.peakUp || stats.PeakRateDown != test.peakDown {
						t.Fatalf("hop peaks = (%d, %d), want (%d, %d)", stats.PeakRateUp, stats.PeakRateDown, test.peakUp, test.peakDown)
					}
				}
				assertPeak()
				for _, url := range getHop().URLs {
					if url.Stats.PeakRateUp != 100 || url.Stats.PeakRateDown != 80 {
						t.Fatalf("URL peaks = %+v, want (100, 80)", url)
					}
				}
				stats := getHop().Stats
				if want := time.Unix(0, max(first.lastUp.Load(), second.lastUp.Load())).UTC().Format(time.RFC3339Nano); stats.LastUp != want {
					t.Fatalf("hop last up = %q, want %q", stats.LastUp, want)
				}
				if want := time.Unix(0, max(first.lastDown.Load(), second.lastDown.Load())).UTC().Format(time.RFC3339Nano); stats.LastDown != want {
					t.Fatalf("hop last down = %q, want %q", stats.LastDown, want)
				}
				registry.tick(end.Add(time.Second))
				assertPeak()
				if stats := getHop().Stats; stats.RateUp != 0 || stats.RateDown != 0 {
					t.Fatalf("idle hop rates = %+v", stats)
				}
				registry.Sync(rules)
				assertPeak()
				way[0].LB = []string{firstURL, "ssh://replacement:22"}
				registry.Sync(rules)
				assertPeak()
				if getHop().URLs[0].Stats.PeakRateUp != 100 || getHop().URLs[1].Stats != (Stats{}) {
					t.Fatal("sync failed to retain matching URL counters or clear replacement counters")
				}
				registry.Reset()
				if current := getHop(); current.Stats != (Stats{}) || current.URLs[0].Stats != (Stats{}) || current.URLs[1].Stats != (Stats{}) {
					t.Fatalf("reset retained hop or URL statistics: %+v", current)
				}
			})
		}
	}
}

func TestRuleWrapListener(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	inner, peer := net.Pipe()
	closeOnCleanup(t, peer)
	wrapped := registry.Rule("rule").WrapListener(&pipeListener{connection: inner})
	accepted, err := wrapped.Accept()
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, accepted)
	stats := registry.Snapshot().Rules[0].Stats
	if stats.Total != 1 || stats.Active != 1 || stats.LastActive == "" {
		t.Fatalf("rule accept = %+v", stats)
	}
	if err := accepted.Close(); err != nil {
		t.Fatal(err)
	}
	if registry.Snapshot().Rules[0].Stats.Active != 0 {
		t.Fatal("rule accept remains active")
	}
}

func TestHopWrapperLazy(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	const raw = "ssh://user:secret@host:22"
	first := rule.HopWrapper(Forward)(2, raw, &net.Dialer{}).(*dialer)
	second := rule.HopWrapper(Forward)(2, raw, &net.Dialer{}).(*dialer)
	if first.count != second.count {
		t.Fatal("same URL created separate counters")
	}
	first.count.up.Store(7)
	rule.HopWrapper(Listen)(0, "ssh://user:secret@%zz", &net.Dialer{})
	snapshot := registry.Snapshot().Rules[0]
	if len(snapshot.Forward) != 3 || snapshot.Forward[2].Stats.Up != 7 || snapshot.Forward[2].URLs[0].URL != "ssh://user:xxxxx@host:22" {
		t.Fatalf("lazy hop = %+v", snapshot.Forward)
	}
	if snapshot.Forward[0].URLs == nil || snapshot.Forward[1].URLs == nil || snapshot.Listen[0].URLs[0].URL != "<invalid url>" {
		t.Fatal("empty hop or invalid URL is not normalized")
	}
	before := registry.Snapshot()
	snapshot.Forward[2].URLs[0].URL = "changed"
	if !reflect.DeepEqual(before, registry.Snapshot()) {
		t.Fatal("snapshot shares storage with registry")
	}
}

func TestTargetCap(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	inner, peer := net.Pipe()
	closeOnCleanup(t, peer)
	wrapped := rule.WrapDialer(bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
		return inner, nil
	}))
	active, err := wrapped.DialContext(context.Background(), "tcp", "active:443")
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, active)
	rule.targets[targetKey{address: "active:443"}].lastActive.Store(1)
	failing := rule.WrapDialer(bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("offline")
	}))
	for index := range MaxTargets + 2 {
		address := fmt.Sprintf("target-%04d:443", index)
		if connected, err := failing.DialContext(context.Background(), "tcp", address); connected != nil || err == nil {
			t.Fatalf("expected failed dial, got (%v, %v)", connected, err)
		}
		rule.targets[targetKey{address: address}].lastActive.Store(int64(index + 2))
	}
	snapshot := registry.Snapshot().Rules[0]
	if len(snapshot.Targets) != MaxTargets || snapshot.TargetsEvicted != 3 {
		t.Fatalf("targets = %d, evicted = %d", len(snapshot.Targets), snapshot.TargetsEvicted)
	}
	if snapshot.Targets[0].Address != "active:443" || snapshot.Targets[0].Stats.Active != 1 {
		t.Fatal("oldest active target did not survive")
	}
	for index := range 3 {
		if _, exists := rule.targets[targetKey{address: fmt.Sprintf("target-%04d:443", index)}]; exists {
			t.Fatalf("old idle target %d was not evicted", index)
		}
	}
	for index := 3; index < MaxTargets+2; index++ {
		if _, exists := rule.targets[targetKey{address: fmt.Sprintf("target-%04d:443", index)}]; !exists {
			t.Fatalf("newer idle target %d was evicted", index)
		}
	}
	for key, count := range rule.targets {
		count.active.Store(1)
		if key.address != "active:443" {
			count.lastActive.Store(time.Now().UnixNano())
		}
	}
	if connected, err := failing.DialContext(context.Background(), "tcp", "all-active-new:443"); connected != nil || err == nil {
		t.Fatalf("expected failed dial, got (%v, %v)", connected, err)
	}
	if _, exists := rule.targets[targetKey{address: "active:443"}]; exists || rule.targetsEvicted != 4 {
		t.Fatal("oldest target was not evicted when every entry was active")
	}
}

func TestRegistryDisconnect(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	connected, peer := dialRulePipe(t, registry.Rule("rule"), "example.com:443")
	id := registry.Snapshot().Rules[0].Connections[0].ID
	done := make(chan error, 1)
	go func() { done <- registry.Disconnect(id) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Disconnect blocked while closing the connection")
	}
	if size, err := peer.Read(make([]byte, 1)); size != 0 || err != io.EOF {
		t.Fatalf("peer Read = (%d, %v), want EOF", size, err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	snapshot := registry.Snapshot().Rules[0]
	if len(snapshot.Connections) != 0 || snapshot.Targets[0].Stats.Active != 0 || snapshot.Targets[0].Stats.Total != 1 {
		t.Fatalf("after Disconnect = %+v", snapshot)
	}
	for _, unknown := range []uint64{0, id, 42} {
		if err := registry.Disconnect(unknown); err == nil || err.Error() != fmt.Sprintf("connection %d not found", unknown) {
			t.Fatalf("Disconnect(%d) = %v, want not found", unknown, err)
		}
	}
}

func TestRegistryResetKeepsConnections(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	connected, peer := dialRulePipe(t, rule, "example.com:443")
	start := time.Now()
	registry.tick(start)
	transferBytes(t, connected, peer, 3)
	transferBytes(t, peer, connected, 5)
	registry.tick(start.Add(time.Second))
	before := registry.Snapshot().Rules[0].Connections
	other, _ := dialRulePipe(t, rule, "other.example.com:443")
	if err := other.Close(); err != nil {
		t.Fatal(err)
	}
	resetAt := time.Now()
	registry.Reset()
	snapshot := registry.Snapshot().Rules[0]
	if len(snapshot.Connections) != 1 {
		t.Fatalf("reset connections = %+v, want one connection", snapshot.Connections)
	}
	connection := snapshot.Connections[0]
	if connection.ID != before[0].ID || connection.Client != before[0].Client ||
		connection.Target != before[0].Target || connection.Via != before[0].Via ||
		!reflect.DeepEqual(connection.Path, before[0].Path) {
		t.Fatalf("reset connection = %+v, want identity and path from %+v", connection, before[0])
	}
	if connection.Stats != (Stats{}) {
		t.Fatalf("reset connection stats = %+v, want empty counters", connection.Stats)
	}
	started, err := time.Parse(time.RFC3339Nano, connection.Started)
	if err != nil {
		t.Fatal(err)
	}
	if started.Before(resetAt) {
		t.Fatalf("reset connection started = %v, want at or after %v", started, resetAt)
	}
	if len(snapshot.Targets) != 1 || snapshot.Targets[0].Address != "example.com:443" || snapshot.Targets[0].Stats != (Stats{}) {
		t.Fatalf("reset targets = %+v, want only the live target with empty counters", snapshot.Targets)
	}
	if snapshot.Stats != (Stats{}) {
		t.Fatalf("reset aggregates = %+v, want empty counters", snapshot)
	}
	transferBytes(t, connected, peer, 7)
	transferBytes(t, peer, connected, 11)
	registry.tick(start.Add(2 * time.Second))
	snapshot = registry.Snapshot().Rules[0]
	connection = snapshot.Connections[0]
	if connection.Stats.Up != 7 || connection.Stats.Down != 11 || connection.Stats.RateUp != 7 || connection.Stats.RateDown != 11 {
		t.Fatalf("post-reset traffic = %+v, want only post-reset bytes and latest rates", connection)
	}
	if snapshot.Targets[0].Stats.Up != 7 || snapshot.Targets[0].Stats.Down != 11 {
		t.Fatalf("post-reset target traffic = %+v, want only post-reset bytes", snapshot.Targets[0])
	}
	dialRulePipe(t, rule, "example.com:443")
	snapshot = registry.Snapshot().Rules[0]
	if len(snapshot.Connections) != 2 || snapshot.Connections[1].ID <= before[0].ID {
		t.Fatalf("post-reset IDs = %+v, want increasing IDs", snapshot.Connections)
	}
	if err := registry.Disconnect(before[0].ID); err != nil {
		t.Fatal(err)
	}
	snapshot = registry.Snapshot().Rules[0]
	if len(snapshot.Connections) != 1 || snapshot.Targets[0].Stats.Active != 1 || snapshot.Targets[0].Stats.Total != 1 {
		t.Fatalf("old connection close changed new target = %+v", snapshot)
	}
}

func TestRegistrySyncDropsConnections(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "removed"}, {Name: "kept"}})
	removed, peer := dialRulePipe(t, registry.Rule("removed"), "example.com:443")
	dialRulePipe(t, registry.Rule("kept"), "example.com:443")
	before := registry.Snapshot()
	id := before.Rules[0].Connections[0].ID
	registry.Sync([]config.Rule{{Name: "kept"}})
	snapshot := registry.Snapshot()
	if len(snapshot.Rules) != 1 || !reflect.DeepEqual(snapshot.Rules[0].Connections, before.Rules[1].Connections) {
		t.Fatalf("reload connections = %+v, want only retained rule", snapshot.Rules)
	}
	if err := registry.Disconnect(id); err == nil || err.Error() != fmt.Sprintf("connection %d not found", id) {
		t.Fatalf("removed rule Disconnect = %v, want not found", err)
	}
	transferBytes(t, removed, peer, 3)
	transferBytes(t, peer, removed, 5)
	registry.Sync([]config.Rule{{Name: "removed"}, {Name: "kept"}})
	dialRulePipe(t, registry.Rule("removed"), "example.com:443")
	snapshot = registry.Snapshot()
	if len(snapshot.Rules[0].Connections) != 1 || snapshot.Rules[0].Connections[0].ID <= before.Rules[1].Connections[0].ID {
		t.Fatalf("re-added rule connections = %+v, want a new ID", snapshot.Rules[0].Connections)
	}
	if err := removed.Close(); err != nil {
		t.Fatal(err)
	}
	if len(registry.Snapshot().Rules[0].Connections) != 1 {
		t.Fatal("closing a removed connection deleted the new rule's connection")
	}
}

func TestRegistrySyncDuringDial(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	inner, peer := net.Pipe()
	closeOnCleanup(t, inner)
	closeOnCleanup(t, peer)
	started := make(chan struct{})
	resume := make(chan struct{})
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(resume) }) })
	dialer := rule.WrapDialer(bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
		close(started)
		<-resume
		return inner, nil
	}))
	done := make(chan net.Conn, 1)
	go func() {
		connected, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")
		if err != nil {
			t.Error(err)
		}
		done <- connected
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("dial did not start")
	}
	registry.Sync(nil)
	registry.Sync([]config.Rule{{Name: "rule"}})
	release.Do(func() { close(resume) })
	var connected net.Conn
	select {
	case connected = <-done:
	case <-time.After(time.Second):
		t.Fatal("dial did not finish")
	}
	if connected == nil {
		t.Fatal("dial returned no connection")
	}
	closeOnCleanup(t, connected)
	setDeadlines(t, connected, peer)
	transferBytes(t, connected, peer, 3)
	registry.mu.RLock()
	count := len(registry.live)
	registry.mu.RUnlock()
	if count != 0 || len(registry.Snapshot().Rules[0].Connections) != 0 {
		t.Fatal("completed dial re-registered a removed rule")
	}
}

func TestRegistryConcurrentConnections(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	dialer := registry.Rule("rule").WrapDialer(bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
		inner, peer := net.Pipe()
		peer.Close()
		return inner, nil
	}))
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			<-start
			for range 50 {
				connected, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")
				if err != nil {
					t.Error(err)
					return
				}
				for _, connection := range registry.Snapshot().Rules[0].Connections {
					if err := registry.Disconnect(connection.ID); err != nil && err.Error() != fmt.Sprintf("connection %d not found", connection.ID) {
						t.Error(err)
					}
				}
				if err := connected.Close(); err != nil {
					t.Error(err)
				}
			}
		})
	}
	workers.Go(func() {
		<-start
		for range 100 {
			registry.tick(time.Now())
			registry.Snapshot()
			registry.Reset()
			registry.Sync([]config.Rule{{Name: "rule"}})
		}
	})
	close(start)
	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent connection operations did not finish")
	}
	if connections := registry.Snapshot().Rules[0].Connections; len(connections) != 0 {
		t.Fatalf("connections leaked after concurrent closes: %+v", connections)
	}
}
