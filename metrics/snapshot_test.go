package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/jumpway/config"
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
	statKeys := []string{"up", "down", "rate_up", "rate_down", "active", "total", "dials", "dial_failures", "latency_ms", "avg_latency_ms"}
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
		keys := []string{"id", "target", "via", "path", "started", "up", "down", "rate_up", "rate_down"}
		if client != "" {
			keys = append(keys, "client")
		}
		assertKeys(t, object, keys...)
		if path, ok := object["path"].([]interface{}); !ok || len(path) != 0 {
			t.Fatalf("direct path = %#v, want []", object["path"])
		}
	}
}
