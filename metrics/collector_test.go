package metrics

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/wzshiming/bridge"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/jumpway/config"
)

func TestCollector(t *testing.T) {
	registry := collectorRegistry(t)
	collector := NewCollector(registry)
	if problems, err := testutil.CollectAndLint(collector); err != nil || len(problems) != 0 {
		t.Fatalf("lint = %v, %v", problems, err)
	}
	const expected = `# HELP jumpway_hop_bytes_total Total number of bytes transferred in each direction.
# TYPE jumpway_hop_bytes_total counter
jumpway_hop_bytes_total{direction="down",hop="0",rule="a",url="ssh://u:xxxxx@h:22",way="forward"} 5
jumpway_hop_bytes_total{direction="up",hop="0",rule="a",url="ssh://u:xxxxx@h:22",way="forward"} 3
# HELP jumpway_hop_info Proxy URL and parent hop for each configured hop.
# TYPE jumpway_hop_info gauge
jumpway_hop_info{hop="0",parent="local",rule="a",url="ssh://u:xxxxx@h:22",way="forward"} 1
# HELP jumpway_hop_last_transfer_timestamp_seconds Unix timestamp of the last byte transferred in each direction.
# TYPE jumpway_hop_last_transfer_timestamp_seconds gauge
jumpway_hop_last_transfer_timestamp_seconds{direction="down",hop="0",rule="a",url="ssh://u:xxxxx@h:22",way="forward"} 100.75
jumpway_hop_last_transfer_timestamp_seconds{direction="up",hop="0",rule="a",url="ssh://u:xxxxx@h:22",way="forward"} 100.25
# HELP jumpway_hop_peak_bandwidth_bytes_per_second Highest bytes per second observed in a completed sampling window since the last reset.
# TYPE jumpway_hop_peak_bandwidth_bytes_per_second gauge
jumpway_hop_peak_bandwidth_bytes_per_second{direction="down",hop="0",rule="a",url="ssh://u:xxxxx@h:22",way="forward"} 5
jumpway_hop_peak_bandwidth_bytes_per_second{direction="up",hop="0",rule="a",url="ssh://u:xxxxx@h:22",way="forward"} 3
# HELP jumpway_rule_active_connections Number of currently active connections.
# TYPE jumpway_rule_active_connections gauge
jumpway_rule_active_connections{rule="a"} 1
jumpway_rule_active_connections{rule="b"} 0
# HELP jumpway_rule_bytes_total Total number of bytes transferred in each direction.
# TYPE jumpway_rule_bytes_total counter
jumpway_rule_bytes_total{direction="down",rule="a"} 5
jumpway_rule_bytes_total{direction="up",rule="a"} 3
jumpway_rule_bytes_total{direction="down",rule="b"} 0
jumpway_rule_bytes_total{direction="up",rule="b"} 0
# HELP jumpway_rule_dial_duration_seconds Duration of successful dials in seconds.
# TYPE jumpway_rule_dial_duration_seconds summary
jumpway_rule_dial_duration_seconds_sum{rule="a"} 0.012345678
jumpway_rule_dial_duration_seconds_count{rule="a"} 1
jumpway_rule_dial_duration_seconds_sum{rule="b"} 0
jumpway_rule_dial_duration_seconds_count{rule="b"} 0
# HELP jumpway_rule_last_transfer_timestamp_seconds Unix timestamp of the last byte transferred in each direction.
# TYPE jumpway_rule_last_transfer_timestamp_seconds gauge
jumpway_rule_last_transfer_timestamp_seconds{direction="down",rule="a"} 100.75
jumpway_rule_last_transfer_timestamp_seconds{direction="up",rule="a"} 100.25
# HELP jumpway_rule_peak_bandwidth_bytes_per_second Highest bytes per second observed in a completed sampling window since the last reset.
# TYPE jumpway_rule_peak_bandwidth_bytes_per_second gauge
jumpway_rule_peak_bandwidth_bytes_per_second{direction="down",rule="a"} 5
jumpway_rule_peak_bandwidth_bytes_per_second{direction="up",rule="a"} 3
jumpway_rule_peak_bandwidth_bytes_per_second{direction="down",rule="b"} 0
jumpway_rule_peak_bandwidth_bytes_per_second{direction="up",rule="b"} 0
# HELP jumpway_target_dials_total Total number of dial attempts.
# TYPE jumpway_target_dials_total counter
jumpway_target_dials_total{rule="a",target="example.com:443",via=""} 1
jumpway_target_dials_total{rule="a",target="example.com:443",via="ssh://h:22"} 2
# HELP jumpway_target_last_transfer_timestamp_seconds Unix timestamp of the last byte transferred in each direction.
# TYPE jumpway_target_last_transfer_timestamp_seconds gauge
jumpway_target_last_transfer_timestamp_seconds{direction="down",rule="a",target="example.com:443",via="ssh://h:22"} 100.75
jumpway_target_last_transfer_timestamp_seconds{direction="up",rule="a",target="example.com:443",via="ssh://h:22"} 100.25
# HELP jumpway_target_peak_bandwidth_bytes_per_second Highest bytes per second observed in a completed sampling window since the last reset.
# TYPE jumpway_target_peak_bandwidth_bytes_per_second gauge
jumpway_target_peak_bandwidth_bytes_per_second{direction="down",rule="a",target="example.com:443",via=""} 0
jumpway_target_peak_bandwidth_bytes_per_second{direction="up",rule="a",target="example.com:443",via=""} 0
jumpway_target_peak_bandwidth_bytes_per_second{direction="down",rule="a",target="example.com:443",via="ssh://h:22"} 5
jumpway_target_peak_bandwidth_bytes_per_second{direction="up",rule="a",target="example.com:443",via="ssh://h:22"} 3
`
	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected),
		"jumpway_rule_peak_bandwidth_bytes_per_second", "jumpway_rule_last_transfer_timestamp_seconds",
		"jumpway_hop_peak_bandwidth_bytes_per_second", "jumpway_hop_last_transfer_timestamp_seconds",
		"jumpway_target_peak_bandwidth_bytes_per_second", "jumpway_target_last_transfer_timestamp_seconds",
		"jumpway_rule_bytes_total", "jumpway_hop_bytes_total", "jumpway_hop_info",
		"jumpway_target_dials_total", "jumpway_rule_active_connections", "jumpway_rule_dial_duration_seconds"); err != nil {
		t.Fatal(err)
	}
}

func TestCollectorTransferStats(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	counts := []*counter{
		rule.count,
		rule.HopWrapper(Forward)(0, "socks5://hop:1080", &net.Dialer{}).(*dialer).count,
		rule.target(targetKey{address: "target:443"}),
	}
	start := time.Unix(100, 0)
	registry.tick(start)
	for _, count := range counts {
		count.addBytes(true, 100, start.Add(250*time.Millisecond).UnixNano())
	}
	registry.tick(start.Add(time.Second))
	for _, count := range counts {
		count.addBytes(true, 20, start.Add(1250*time.Millisecond).UnixNano())
	}
	registry.tick(start.Add(2 * time.Second))
	registry.tick(start.Add(3 * time.Second))
	collector := NewCollector(registry)
	if problems, err := testutil.CollectAndLint(collector); err != nil || len(problems) != 0 {
		t.Fatalf("lint = %v, %v", problems, err)
	}
	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	wanted := make(map[string]bool)
	for _, level := range []string{"rule", "hop", "target"} {
		wanted["jumpway_"+level+"_peak_bandwidth_bytes_per_second"] = false
		wanted["jumpway_"+level+"_last_transfer_timestamp_seconds"] = false
	}
	for _, family := range families {
		name := family.GetName()
		if _, exists := wanted[name]; !exists {
			continue
		}
		wanted[name] = true
		peak := strings.HasSuffix(name, "peak_bandwidth_bytes_per_second")
		wantSamples := 1
		if peak {
			wantSamples = 2
		}
		if family.GetType().String() != "GAUGE" || len(family.Metric) != wantSamples {
			t.Fatalf("%s = %v, want %d gauges", name, family, wantSamples)
		}
		for _, metric := range family.Metric {
			var direction string
			for _, label := range metric.Label {
				if label.GetName() == "direction" {
					direction = label.GetValue()
				}
			}
			if direction != "up" && direction != "down" {
				t.Fatalf("%s has no valid direction: %v", name, metric)
			}
			want := 101.25
			if peak {
				want = 100
				if direction == "down" {
					want = 0
				}
			} else if direction != "up" {
				t.Fatalf("%s emitted an untouched direction: %v", name, metric)
			}
			if got := metric.GetGauge().GetValue(); got != want {
				t.Errorf("%s direction %s = %g, want %g", name, direction, got, want)
			}
		}
	}
	for name, found := range wanted {
		if !found {
			t.Errorf("missing metric family %s", name)
		}
	}
	registry.Reset()
	families, err = reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if strings.HasSuffix(family.GetName(), "last_transfer_timestamp_seconds") {
			t.Errorf("reset retained transfer family %s", family.GetName())
		}
		if strings.HasSuffix(family.GetName(), "peak_bandwidth_bytes_per_second") {
			for _, metric := range family.Metric {
				if metric.GetGauge().GetValue() != 0 {
					t.Errorf("reset retained peak: %v", metric)
				}
			}
		}
	}
}

func TestCollectorHandler(t *testing.T) {
	registry := collectorRegistry(t)
	handler := NewHandler(registry)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	body := response.Body.String()
	for _, text := range []string{"jumpway_rule_bytes_total{", "go_goroutines", "ssh://u:xxxxx@h:22"} {
		if !strings.Contains(body, text) {
			t.Errorf("response does not contain %q", text)
		}
	}
	if strings.Contains(body, "secret") {
		t.Fatal("response leaked a proxy password")
	}
	t.Run("process", func(t *testing.T) {
		switch runtime.GOOS {
		case "darwin", "windows":
		case "js", "wasip1", "ios":
			t.Skip("process collector is unsupported on this platform")
		default:
			if _, err := os.Stat("/proc"); err != nil {
				t.Skip("process collector requires procfs on this platform")
			}
		}
		if !strings.Contains(body, "process_") {
			t.Fatal("response has no process metrics")
		}
	})
	empty := httptest.NewRecorder()
	NewHandler(NewRegistry()).ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if empty.Code != http.StatusOK || strings.Contains(empty.Body.String(), "jumpway_rule_bytes_total{") {
		t.Fatalf("independent handler = %d: %s", empty.Code, empty.Body.String())
	}
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if strings.HasPrefix(family.GetName(), "jumpway_") {
			t.Fatalf("collector registered globally: %s", family.GetName())
		}
	}
}

func TestCollectorGather(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := reg.Register(NewCollector(collectorRegistry(t))); err != nil {
		t.Fatal(err)
	}
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 36 {
		t.Fatalf("metric families = %d, want 36", len(families))
	}
	type sample struct {
		labels                                   map[string]string
		up, down, total, active, dials, failures float64
		duration, activity                       float64
	}
	expected := map[string][]sample{
		"rule": {
			{labels: map[string]string{"rule": "a"}, up: 3, down: 5, total: 1, active: 1, dials: 3, failures: 2, duration: 0.012345678, activity: 100},
			{labels: map[string]string{"rule": "b"}},
		},
		"hop": {
			{labels: map[string]string{"rule": "a", "way": "forward", "hop": "0", "url": "ssh://u:xxxxx@h:22"}, up: 3, down: 5, total: 1, active: 1, dials: 2, failures: 1, duration: 0.012345678, activity: 100},
		},
		"target": {
			{labels: map[string]string{"rule": "a", "target": "example.com:443", "via": "ssh://h:22"}, up: 3, down: 5, total: 1, active: 1, dials: 2, failures: 1, duration: 0.012345678, activity: 100},
			{labels: map[string]string{"rule": "a", "target": "example.com:443", "via": ""}, dials: 1, failures: 1, activity: 100},
		},
	}
	checked := 0
	for _, family := range families {
		level, suffix, _ := strings.Cut(strings.TrimPrefix(family.GetName(), "jumpway_"), "_")
		for _, metric := range family.Metric {
			checked++
			labels := make(map[string]string)
			for _, label := range metric.Label {
				labels[label.GetName()] = label.GetValue()
			}
			if family.GetName() == "jumpway_hop_info" {
				if family.GetType().String() != "GAUGE" || metric.GetGauge().GetValue() != 1 || labels["parent"] != "local" {
					t.Fatalf("hop info = %v", metric)
				}
				continue
			}
			if family.GetName() == "jumpway_stats_reset_timestamp_seconds" {
				if family.GetType().String() != "GAUGE" || len(labels) != 0 || metric.GetGauge().GetValue() != 100 {
					t.Fatalf("reset timestamp = %v", metric)
				}
				continue
			}
			direction := labels["direction"]
			delete(labels, "direction")
			var want *sample
			for _, candidate := range expected[level] {
				if reflect.DeepEqual(labels, candidate.labels) {
					want = &candidate
					break
				}
			}
			if want == nil {
				t.Fatalf("unexpected labels for %s: %v", family.GetName(), labels)
			}
			kind, value := "GAUGE", float64(0)
			switch suffix {
			case "bytes_total", "bandwidth_bytes_per_second", "peak_bandwidth_bytes_per_second", "last_transfer_timestamp_seconds":
				if direction == "up" {
					value = want.up
				} else if direction == "down" {
					value = want.down
				} else {
					t.Fatalf("unexpected direction %q", direction)
				}
				if suffix == "bytes_total" {
					kind = "COUNTER"
				}
				if suffix == "last_transfer_timestamp_seconds" {
					if value == 0 {
						t.Fatal("last transfer emitted for an untouched direction")
					}
					value = 100.25
					if direction == "down" {
						value = 100.75
					}
				}
			case "connections_total":
				kind, value = "COUNTER", want.total
			case "active_connections":
				value = want.active
			case "dials_total":
				kind, value = "COUNTER", want.dials
			case "dial_failures_total":
				kind, value = "COUNTER", want.failures
			case "dial_duration_seconds":
				kind, value = "SUMMARY", want.duration
				if summary := metric.GetSummary(); summary.GetSampleCount() != uint64(want.dials-want.failures) || len(summary.Quantile) != 0 {
					t.Fatalf("summary = %v", summary)
				}
			case "last_dial_duration_seconds":
				if want.dials == want.failures {
					t.Fatal("last dial duration emitted without a successful dial")
				}
				value = want.duration
			case "last_activity_timestamp_seconds":
				if want.activity == 0 {
					t.Fatal("last activity emitted for an idle counter")
				}
				value = want.activity
			case "targets_evicted_total":
				kind = "COUNTER"
			default:
				t.Fatalf("unexpected metric family %s", family.GetName())
			}
			if suffix != "bytes_total" && suffix != "bandwidth_bytes_per_second" && suffix != "peak_bandwidth_bytes_per_second" && suffix != "last_transfer_timestamp_seconds" && direction != "" {
				t.Fatalf("unexpected direction label on %s", family.GetName())
			}
			got := metric.GetGauge().GetValue() + metric.GetCounter().GetValue() + metric.GetSummary().GetSampleSum()
			if family.GetType().String() != kind || got != value {
				t.Errorf("%s %v = %s %g, want %s %g", family.GetName(), labels, family.GetType(), got, kind, value)
			}
		}
	}
	if checked != 72 {
		t.Fatalf("metrics = %d, want 72", checked)
	}
}

func TestCollectorRemovesSeries(t *testing.T) {
	registry := collectorRegistry(t)
	reg := prometheus.NewRegistry()
	reg.MustRegister(NewCollector(registry))
	if _, err := reg.Gather(); err != nil {
		t.Fatal(err)
	}
	if err := registry.Disconnect(registry.Snapshot().Rules[0].Connections[0].ID); err != nil {
		t.Fatal(err)
	}
	registry.Reset()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		name := family.GetName()
		if strings.HasPrefix(name, "jumpway_target_") || strings.Contains(name, "last_dial_duration") || strings.Contains(name, "last_activity") || strings.Contains(name, "last_transfer") {
			t.Errorf("stale metric family after reset: %s", name)
		}
	}
	registry.Sync([]config.Rule{{Name: "a"}, {Name: "b"}})
	families, err = reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if strings.HasPrefix(family.GetName(), "jumpway_hop_") {
			t.Errorf("removed hop retained: %s", family.GetName())
		}
	}
	registry.Sync([]config.Rule{{Name: "b"}})
	families, err = reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() == "rule" && label.GetValue() == "a" {
					t.Errorf("removed rule retained in %s", family.GetName())
				}
			}
		}
	}
}

func TestCollectorHopParents(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "a"}})
	for _, role := range []Role{Listen, Forward} {
		for index := range 2 {
			registry.Rule("a").HopWrapper(role)(index, "ssh://hop"+strconv.Itoa(index)+":22", &net.Dialer{})
		}
	}
	const expected = `# HELP jumpway_hop_info Proxy URL and parent hop for each configured hop.
# TYPE jumpway_hop_info gauge
jumpway_hop_info{hop="0",parent="1",rule="a",url="ssh://hop0:22",way="forward"} 1
jumpway_hop_info{hop="0",parent="1",rule="a",url="ssh://hop0:22",way="listen"} 1
jumpway_hop_info{hop="1",parent="local",rule="a",url="ssh://hop1:22",way="forward"} 1
jumpway_hop_info{hop="1",parent="local",rule="a",url="ssh://hop1:22",way="listen"} 1
`
	if err := testutil.CollectAndCompare(NewCollector(registry), strings.NewReader(expected), "jumpway_hop_info"); err != nil {
		t.Fatal(err)
	}
}

func TestCollectorDuplicateURLs(t *testing.T) {
	registry := collectorRegistry(t)
	registry.Sync([]config.Rule{
		{Name: "a", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{
			"ssh://u:secret@h:22", "ssh://u:secret@h:22", "ssh://u:other@h:22",
		}}}}},
		{Name: "b"},
	})
	const expected = `# HELP jumpway_hop_bytes_total Total number of bytes transferred in each direction.
# TYPE jumpway_hop_bytes_total counter
jumpway_hop_bytes_total{direction="down",hop="0",rule="a",url="ssh://u:xxxxx@h:22",way="forward"} 5
jumpway_hop_bytes_total{direction="up",hop="0",rule="a",url="ssh://u:xxxxx@h:22",way="forward"} 3
# HELP jumpway_hop_info Proxy URL and parent hop for each configured hop.
# TYPE jumpway_hop_info gauge
jumpway_hop_info{hop="0",parent="local",rule="a",url="ssh://u:xxxxx@h:22",way="forward"} 1
`
	if err := testutil.CollectAndCompare(NewCollector(registry), strings.NewReader(expected), "jumpway_hop_bytes_total", "jumpway_hop_info"); err != nil {
		t.Fatal(err)
	}
}

func TestCollectorConcurrentReset(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "a"}})
	rule := registry.Rule("a")
	inner, peer := net.Pipe()
	closeOnCleanup(t, inner)
	closeOnCleanup(t, peer)
	connected, err := rule.WrapDialer(rule.HopWrapper(Forward)(0, "ssh://u:secret@h:22",
		bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
			return inner, nil
		}))).DialContext(context.Background(), "tcp", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, connected)
	accepted, err := rule.WrapListener(&pipeListener{connection: peer}).Accept()
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, accepted)
	setDeadlines(t, connected, accepted)
	failing := rule.WrapDialer(rule.HopWrapper(Forward)(0, "ssh://u:secret@h:22",
		bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
			return nil, net.ErrClosed
		})))
	reg := prometheus.NewRegistry()
	reg.MustRegister(NewCollector(registry))
	start := make(chan struct{})
	trafficDone := make(chan struct{})
	resetDone := make(chan struct{})
	var group sync.WaitGroup
	var writes, scrapes, resets atomic.Int64
	group.Go(func() {
		<-start
		if _, err := io.Copy(io.Discard, accepted); err != nil {
			t.Error(err)
		}
	})
	group.Go(func() {
		defer close(trafficDone)
		defer connected.Close()
		<-start
		for range 200 {
			if _, err := connected.Write([]byte("traffic")); err != nil {
				t.Error(err)
				return
			}
			writes.Add(1)
			if connection, err := failing.DialContext(context.Background(), "tcp", "example.com:443"); connection != nil || !errors.Is(err, net.ErrClosed) {
				t.Errorf("failed dial = (%v, %v)", connection, err)
				return
			}
		}
	})
	group.Go(func() {
		defer close(resetDone)
		<-start
		for {
			registry.Reset()
			registry.tick(time.Now().Add(time.Second))
			resets.Add(1)
			select {
			case <-trafficDone:
				return
			default:
			}
		}
	})
	for range 2 {
		group.Go(func() {
			<-start
			for {
				families, err := reg.Gather()
				if err != nil {
					t.Error(err)
					return
				}
				for _, family := range families {
					for _, metric := range family.Metric {
						if strings.HasSuffix(family.GetName(), "active_connections") && metric.GetGauge().GetValue() < 0 {
							t.Errorf("negative active connections: %v", metric)
						}
						if strings.HasSuffix(family.GetName(), "dial_duration_seconds") && metric.GetSummary().GetSampleCount() > 1 {
							t.Errorf("incoherent successful dial count: %v", metric)
						}
					}
				}
				scrapes.Add(1)
				select {
				case <-resetDone:
					return
				default:
				}
			}
		})
	}
	close(start)
	group.Wait()
	if writes.Load() != 200 || scrapes.Load() == 0 || resets.Load() == 0 {
		t.Fatalf("writes = %d, scrapes = %d, resets = %d", writes.Load(), scrapes.Load(), resets.Load())
	}
}

func collectorRegistry(t *testing.T) *Registry {
	t.Helper()
	const raw = "ssh://u:secret@h:22"
	registry := NewRegistry()
	registry.Sync([]config.Rule{
		{Name: "a", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{raw}}}}},
		{Name: "b"},
	})
	rule := registry.Rule("a")
	inner, target := net.Pipe()
	closeOnCleanup(t, inner)
	closeOnCleanup(t, target)
	var dialErr error
	base := bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
		if dialErr != nil {
			return nil, dialErr
		}
		return inner, nil
	})
	dialer := rule.WrapDialer(rule.HopWrapper(Forward)(0, raw, base))
	connected, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, connected)
	incoming, client := net.Pipe()
	closeOnCleanup(t, incoming)
	closeOnCleanup(t, client)
	accepted, err := rule.WrapListener(&pipeListener{connection: incoming}).Accept()
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, accepted)
	setDeadlines(t, accepted, client, connected, target)
	start := time.Unix(100, 0)
	registry.tick(start)
	done := make(chan error, 1)
	go func() {
		_, err := io.CopyN(connected, accepted, 3)
		if err == nil {
			_, err = io.CopyN(accepted, connected, 5)
		}
		done <- err
	}()
	if _, err := client.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(target, make([]byte, 3)); err != nil {
		t.Fatal(err)
	}
	if _, err := target.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(client, make([]byte, 5)); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	dialErr = net.ErrClosed
	for _, failing := range []bridge.Dialer{dialer, rule.WrapDialer(base)} {
		if connection, err := failing.DialContext(context.Background(), "tcp", "example.com:443"); connection != nil || !errors.Is(err, dialErr) {
			t.Fatalf("failed dial = (%v, %v)", connection, err)
		}
	}
	registry.tick(start.Add(time.Second))
	registry.mu.Lock()
	registry.since = start
	registry.eachCounter(func(count *counter) {
		if count.lastDial.Load() != 0 {
			count.latency.Store(12345678)
			count.latencySum.Store(12345678)
			count.lastDial.Store(start.UnixNano())
		}
		if count.lastActive.Load() != 0 {
			count.lastActive.Store(start.UnixNano())
		}
		if count.lastUp.Load() != 0 {
			count.lastUp.Store(start.Add(250 * time.Millisecond).UnixNano())
		}
		if count.lastDown.Load() != 0 {
			count.lastDown.Store(start.Add(750 * time.Millisecond).UnixNano())
		}
	})
	registry.mu.Unlock()
	return registry
}
