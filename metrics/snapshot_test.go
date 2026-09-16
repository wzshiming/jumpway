package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/netproc"
)

func TestSnapshotJSON(t *testing.T) {
	registry := NewRegistry()
	empty, err := json.Marshal(registry.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(empty), `"rules":[]`) {
		t.Fatalf("empty rules = %s", empty)
	}
	const raw = "ssh://user:secret@host:22"
	registry.Sync([]config.Rule{
		{Name: "empty"},
		{Name: "full", Listen: config.Listen{Way: []bridgeconfig.Node{{LB: []string{raw}}}}, Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{raw}}}}},
	})
	rule := registry.Rule("full")
	wrapped := rule.WrapDialer(rule.HopWrapper(Forward)(0, raw, bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("offline")
	})))
	if connected, err := wrapped.DialContext(context.Background(), "tcp", "example.com:443"); connected != nil || err == nil {
		t.Fatalf("expected failed dial, got (%v, %v)", connected, err)
	}
	data, err := json.Marshal(registry.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret") {
		t.Fatalf("snapshot leaked password: %s", data)
	}
	var snapshot map[string]interface{}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, snapshot, "since", "rules")
	if _, err := time.Parse(time.RFC3339, snapshot["since"].(string)); err != nil {
		t.Fatal(err)
	}
	rules := snapshot["rules"].([]interface{})
	plain := rules[0].(map[string]interface{})
	for _, key := range []string{"listen", "forward", "targets", "connections"} {
		values, ok := plain[key].([]interface{})
		if !ok || len(values) != 0 {
			t.Fatalf("empty %s = %#v, want []", key, plain[key])
		}
	}
	full := rules[1].(map[string]interface{})
	assertKeys(t, full, "name", "stats", "listen", "forward", "targets", "connections", "targets_evicted")
	hop := full["forward"].([]interface{})[0].(map[string]interface{})
	assertKeys(t, hop, "index", "parent_index", "stats", "urls")
	url := hop["urls"].([]interface{})[0].(map[string]interface{})
	assertKeys(t, url, "url", "stats")
	target := full["targets"].([]interface{})[0].(map[string]interface{})
	assertKeys(t, target, "address", "via", "stats")
	statKeys := []string{"up", "down", "rate_up", "rate_down", "peak_rate_up", "peak_rate_down", "active", "total", "dials", "dial_failures", "latency_ms", "avg_latency_ms"}
	assertKeys(t, plain["stats"].(map[string]interface{}), statKeys...)
	for _, object := range []map[string]interface{}{full, hop, url, target} {
		stats := object["stats"].(map[string]interface{})
		assertKeys(t, stats, append(statKeys, "last_active")...)
		if _, err := time.Parse(time.RFC3339Nano, stats["last_active"].(string)); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []interface{}{Snapshot{}, RuleStats{}, Hop{}, URLStats{}, Target{}, Stats{}, Connection{}, PathHop{}} {
		typeInfo := reflect.TypeOf(value)
		for _, field := range reflect.VisibleFields(typeInfo) {
			if field.IsExported() && (field.Type == reflect.TypeFor[time.Time]() || field.Type == reflect.TypeFor[time.Duration]()) {
				t.Fatalf("exported time field: %s.%s", typeInfo.Name(), field.Name)
			}
		}
	}
}

func TestSnapshotTransferJSON(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	inner, peer := net.Pipe()
	closeOnCleanup(t, peer)
	hopDialer := rule.HopWrapper(Forward)(0, "socks5://hop:1080", bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
		return inner, nil
	}))
	connected, err := rule.WrapDialer(hopDialer).DialContext(context.Background(), "tcp", "target:443")
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, connected)
	acceptedInner, client := net.Pipe()
	closeOnCleanup(t, client)
	accepted, err := rule.WrapListener(&pipeListener{connection: acceptedInner}).Accept()
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, accepted)
	setDeadlines(t, connected, peer, accepted, client)
	start := time.Now()
	registry.tick(start)
	transferBytes(t, client, accepted, 3)
	transferBytes(t, accepted, client, 5)
	transferBytes(t, connected, peer, 3)
	transferBytes(t, peer, connected, 5)
	registry.tick(start.Add(time.Second))
	data, err := json.Marshal(registry.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]interface{}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	full := snapshot["rules"].([]interface{})[0].(map[string]interface{})
	hop := full["forward"].([]interface{})[0].(map[string]interface{})
	url := hop["urls"].([]interface{})[0].(map[string]interface{})
	target := full["targets"].([]interface{})[0].(map[string]interface{})
	connection := full["connections"].([]interface{})[0].(map[string]interface{})
	assertKeys(t, connection, "id", "target", "via", "path", "started", "stats")
	for _, entity := range []struct {
		name   string
		object map[string]interface{}
		count  *counter
	}{
		{name: "rule", object: full, count: rule.count},
		{name: "hop", object: hop, count: hopDialer.(*dialer).count},
		{name: "url", object: url, count: hopDialer.(*dialer).count},
		{name: "target", object: target, count: connected.(*conn).count},
		{name: "connection", object: connection, count: connected.(*conn).extra},
	} {
		t.Run(entity.name, func(t *testing.T) {
			stats := entity.object["stats"].(map[string]interface{})
			for key, want := range map[string]float64{
				"up": 3, "down": 5, "rate_up": 3, "rate_down": 5, "peak_rate_up": 3, "peak_rate_down": 5,
			} {
				if stats[key] != want {
					t.Errorf("%s = %#v, want %v", key, stats[key], want)
				}
			}
			for key, timestamp := range map[string]int64{"last_up": entity.count.lastUp.Load(), "last_down": entity.count.lastDown.Load()} {
				if timestamp == 0 {
					t.Fatalf("%s was not recorded", key)
				}
				if want := time.Unix(0, timestamp).UTC().Format(time.RFC3339Nano); stats[key] != want {
					t.Errorf("%s = %#v, want %q", key, stats[key], want)
				}
			}
		})
	}
}

func TestSnapshotResolvesProcess(t *testing.T) {
	dial := func(rule *Rule, client net.Addr) {
		inner, peer := net.Pipe()
		closeOnCleanup(t, peer)
		ctx := context.Background()
		if client != nil {
			ctx = jumpway.WithClientAddr(ctx, client)
		}
		connected, err := rule.WrapDialer(bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
			return inner, nil
		})).DialContext(ctx, "tcp", "example.com:443")
		if err != nil {
			t.Fatal(err)
		}
		closeOnCleanup(t, connected)
	}
	client := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	want := &netproc.Process{PID: 4242, Name: "curl"}
	owners := map[netip.AddrPort]netproc.Process{netip.MustParseAddrPort("127.0.0.1:12345"): *want}
	t.Run("resolution", func(t *testing.T) {
		registry := NewRegistry()
		registry.lookup = nil
		registry.Sync([]config.Rule{
			{Name: "local"},
			{Name: "remote", Listen: config.Listen{Way: []bridgeconfig.Node{{LB: []string{"ssh://user:secret@entry:22"}}}}},
		})
		local := registry.Rule("local")
		dial(local, client)
		dial(local, &net.TCPAddr{IP: net.ParseIP("::1"), Port: 12346})
		dial(local, &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 1})
		dial(registry.Rule("remote"), &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12347})
		dial(local, nil)
		for _, rule := range registry.Snapshot().Rules {
			for _, connection := range rule.Connections {
				if connection.Process != nil {
					t.Fatalf("without lookup: connection = %+v", connection)
				}
			}
		}
		var calls atomic.Int32
		registry.lookup = func(ctx context.Context) (map[netip.AddrPort]netproc.Process, error) {
			calls.Add(1)
			if _, ok := ctx.Deadline(); !ok {
				t.Error("lookup context has no deadline")
			}
			return owners, nil
		}
		snapshot := registry.Snapshot()
		if calls.Load() != 1 || !reflect.DeepEqual(snapshot.Rules[0].Connections[0].Process, want) {
			t.Fatalf("first Snapshot: calls = %d, process = %+v, want %+v", calls.Load(), snapshot.Rules[0].Connections[0].Process, want)
		}
		for _, rule := range snapshot.Rules {
			for _, connection := range rule.Connections {
				if connection.ID != 1 && connection.Process != nil {
					t.Fatalf("unexpected process: connection = %+v", connection)
				}
			}
		}
		registry.Snapshot()
		if calls.Load() != 1 {
			t.Fatalf("second Snapshot: calls = %d, want 1", calls.Load())
		}
		dial(local, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12348})
		registry.Snapshot()
		if calls.Load() != 2 {
			t.Fatalf("new connection: calls = %d, want 2", calls.Load())
		}
		snapshot.Rules[0].Connections[0].Process.Name = "changed"
		snapshot = registry.Snapshot()
		if !reflect.DeepEqual(snapshot.Rules[0].Connections[0].Process, want) {
			t.Fatalf("snapshot shares process storage: %+v", snapshot.Rules[0].Connections[0].Process)
		}
		for _, index := range []int{0, 2} {
			data, err := json.Marshal(snapshot.Rules[0].Connections[index])
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]interface{}
			if err := json.Unmarshal(data, &object); err != nil {
				t.Fatal(err)
			}
			keys := []string{"id", "client", "target", "via", "path", "started", "stats"}
			if index == 0 {
				keys = append(keys, "process")
			}
			assertKeys(t, object, keys...)
		}
	})
	t.Run("non-candidates", func(t *testing.T) {
		registry := NewRegistry()
		registry.Sync([]config.Rule{
			{Name: "local"},
			{Name: "remote", Listen: config.Listen{Way: []bridgeconfig.Node{{LB: []string{"ssh://user:secret@entry:22"}}}}},
		})
		dial(registry.Rule("local"), &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 1})
		dial(registry.Rule("remote"), client)
		dial(registry.Rule("local"), nil)
		var calls atomic.Int32
		registry.lookup = func(context.Context) (map[netip.AddrPort]netproc.Process, error) {
			calls.Add(1)
			return owners, nil
		}
		if registry.Snapshot(); calls.Load() != 0 {
			t.Fatalf("non-candidates: calls = %d, want 0", calls.Load())
		}
	})
	t.Run("error", func(t *testing.T) {
		registry := NewRegistry()
		registry.Sync([]config.Rule{{Name: "local"}})
		dial(registry.Rule("local"), client)
		var calls atomic.Int32
		registry.lookup = func(context.Context) (map[netip.AddrPort]netproc.Process, error) {
			calls.Add(1)
			return nil, errors.New("boom")
		}
		for range 2 {
			connection := registry.Snapshot().Rules[0].Connections[0]
			if calls.Load() != 1 || connection.Process != nil {
				t.Fatalf("failed lookup: calls = %d, connection = %+v", calls.Load(), connection)
			}
		}
	})
	t.Run("concurrency", func(t *testing.T) {
		registry := NewRegistry()
		registry.Sync([]config.Rule{{Name: "local"}})
		dial(registry.Rule("local"), client)
		started := make(chan struct{}, 1)
		release := make(chan struct{})
		defer func() {
			select {
			case <-release:
			default:
				close(release)
			}
		}()
		var calls atomic.Int32
		registry.lookup = func(context.Context) (map[netip.AddrPort]netproc.Process, error) {
			calls.Add(1)
			started <- struct{}{}
			<-release
			return owners, nil
		}
		resolved := make(chan Snapshot, 1)
		go func() { resolved <- registry.Snapshot() }()
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("lookup did not start")
		}
		skipped := make(chan Snapshot, 1)
		go func() { skipped <- registry.Snapshot() }()
		select {
		case snapshot := <-skipped:
			if connection := snapshot.Rules[0].Connections[0]; connection.Process != nil {
				t.Fatalf("in-flight lookup: connection = %+v", connection)
			}
		case <-time.After(time.Second):
			t.Fatal("concurrent Snapshot blocked on lookup")
		}
		written := make(chan struct{})
		go func() { registry.Reset(); close(written) }()
		select {
		case <-written:
		case <-time.After(time.Second):
			t.Fatal("writer blocked on lookup")
		}
		close(release)
		select {
		case snapshot := <-resolved:
			if process := snapshot.Rules[0].Connections[0].Process; calls.Load() != 1 || !reflect.DeepEqual(process, want) {
				t.Fatalf("resolved lookup: calls = %d, process = %+v, want %+v", calls.Load(), process, want)
			}
		case <-time.After(time.Second):
			t.Fatal("resolving Snapshot did not finish")
		}
	})
}

func assertKeys(t *testing.T, object map[string]interface{}, expected ...string) {
	t.Helper()
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	expected = append([]string(nil), expected...)
	sort.Strings(expected)
	if !reflect.DeepEqual(keys, expected) {
		t.Fatalf("keys = %v, want %v", keys, expected)
	}
}

func TestSnapshotHopAggregation(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	first := rule.HopWrapper(Forward)(0, "ssh://first:22", &net.Dialer{}).(*dialer).count
	second := rule.HopWrapper(Forward)(0, "ssh://second:22", &net.Dialer{}).(*dialer).count
	start := time.Now()
	registry.tick(start)
	first.dial(2*time.Millisecond, start, nil)
	first.dial(4*time.Millisecond, start.Add(time.Second), nil)
	second.dial(12*time.Millisecond, start.Add(2*time.Second), nil)
	first.dial(50*time.Millisecond, start.Add(3*time.Second), errors.New("failed"))
	first.up.Store(20)
	first.down.Store(30)
	first.active.Store(1)
	first.total.Store(2)
	second.up.Store(60)
	second.down.Store(90)
	second.active.Store(2)
	second.total.Store(3)
	registry.tick(start.Add(time.Second))
	stats := registry.Snapshot().Rules[0].Forward[0].Stats
	if stats.Up != 80 || stats.Down != 120 || stats.RateUp != 80 || stats.RateDown != 120 || stats.Active != 3 || stats.Total != 5 {
		t.Fatalf("hop totals = %+v", stats)
	}
	if stats.Dials != 4 || stats.DialFailures != 1 || stats.LatencyMs != 12 || stats.AvgLatencyMs != 6 {
		t.Fatalf("hop dial measurements = %+v", stats)
	}
	if want := start.Add(3 * time.Second).UTC().Format(time.RFC3339Nano); stats.LastActive != want {
		t.Fatalf("hop LastActive = %q, want %q", stats.LastActive, want)
	}
}

func TestSnapshotDuplicateRedactedURLs(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"ssh://u:a@h:22", "ssh://u:b@h:22"}}}}}})
	rule := registry.Rule("rule")
	count := rule.HopWrapper(Forward)(0, "ssh://u:a@h:22", &net.Dialer{}).(*dialer).count
	count.dial(time.Millisecond, time.Now(), nil)
	count.total.Store(1)
	hop := registry.Snapshot().Rules[0].Forward[0]
	if len(hop.URLs) != 2 || hop.URLs[0].URL != "ssh://u:xxxxx@h:22" || hop.URLs[1].URL != hop.URLs[0].URL {
		t.Fatalf("urls = %+v, want both positions with the redacted URL", hop.URLs)
	}
	if hop.Stats.Dials != 1 || hop.Stats.Total != 1 {
		t.Fatalf("shared counter summed twice: %+v", hop.Stats)
	}
}

func TestSnapshotTargetOrder(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	for _, target := range []struct {
		address string
		via     string
		active  int64
		up      int64
		down    int64
	}{
		{address: "z:443", active: 2},
		{address: "a:443", active: 1, up: 3, down: 4},
		{address: "b:443", active: 1, up: 6, down: 2},
		{address: "same:443", via: "b", active: 1, up: 1},
		{address: "same:443", via: "a", active: 1, up: 1},
		{address: "idle:443", down: 100},
	} {
		count := rule.target(targetKey{address: target.address, via: target.via})
		count.active.Store(target.active)
		count.up.Store(target.up)
		count.down.Store(target.down)
	}
	var keys []targetKey
	for _, target := range registry.Snapshot().Rules[0].Targets {
		keys = append(keys, targetKey{address: target.Address, via: target.Via})
	}
	expected := []targetKey{{address: "z:443"}, {address: "b:443"}, {address: "a:443"}, {address: "same:443", via: "a"}, {address: "same:443", via: "b"}, {address: "idle:443"}}
	if !reflect.DeepEqual(keys, expected) {
		t.Fatalf("target order = %v, want %v", keys, expected)
	}
}

func TestSnapshotConnections(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "first"}, {Name: "idle"}, {Name: "second"}})
	for index := range 8 {
		dialRulePipe(t, registry.Rule("first"), "example.com:443")
		dialRulePipe(t, registry.Rule("second"), "other:443")
		if index == 0 {
			if got := registry.Snapshot().Rules[0].Connections[0].ID; got != 1 {
				t.Fatalf("first ID = %d, want 1", got)
			}
		}
	}
	for range 10 {
		snapshot := registry.Snapshot()
		seen := make(map[uint64]bool)
		for _, rule := range snapshot.Rules {
			var previous uint64
			if rule.Connections == nil {
				t.Fatalf("rule %q has null connections", rule.Name)
			}
			for _, connection := range rule.Connections {
				if connection.ID <= previous || seen[connection.ID] {
					t.Fatalf("connections not sorted with registry-wide IDs: %+v", snapshot.Rules)
				}
				seen[connection.ID] = true
				previous = connection.ID
			}
		}
		if len(seen) != 16 || len(snapshot.Rules[1].Connections) != 0 {
			t.Fatalf("connections assigned to wrong rules: %+v", snapshot.Rules)
		}
	}
	snapshot := registry.Snapshot()
	snapshot.Rules[0].Connections[0].Target = "changed"
	if registry.Snapshot().Rules[0].Connections[0].Target != "example.com:443" {
		t.Fatal("snapshot shares live connection storage")
	}
	connection := registry.Snapshot().Rules[0].Connections[0]
	for _, client := range []string{"", "127.0.0.1:12345"} {
		connection.Client = client
		data, err := json.Marshal(connection)
		if err != nil {
			t.Fatal(err)
		}
		var object map[string]interface{}
		if err := json.Unmarshal(data, &object); err != nil {
			t.Fatal(err)
		}
		keys := []string{"id", "target", "via", "path", "started", "stats"}
		if client != "" {
			keys = append(keys, "client")
		}
		assertKeys(t, object, keys...)
		if path, ok := object["path"].([]interface{}); !ok || len(path) != 0 {
			t.Fatalf("direct path = %#v, want []", object["path"])
		}
	}
}
