package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
)

func TestHopDialer(t *testing.T) {
	inner, peer := net.Pipe()
	closeOnCleanup(t, peer)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failure := errors.New("dial failed")
	var dialErr error
	count := &counter{}
	wrapped := &dialer{count: count, inner: bridge.DialFunc(func(callCtx context.Context, network, address string) (net.Conn, error) {
		if callCtx != ctx || network != "tcp" || address != "example.com:443" {
			t.Fatal("dial arguments were not forwarded")
		}
		if dialErr != nil {
			return nil, dialErr
		}
		return inner, nil
	})}
	connected, err := wrapped.DialContext(ctx, "tcp", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, connected)
	if count.dials.Load() != 1 || count.dialFailures.Load() != 0 || count.latency.Load() <= 0 {
		t.Fatal("successful dial not measured")
	}
	if count.latencySum.Load() != count.latency.Load() {
		t.Fatal("successful latency not included in sum")
	}
	if measured, ok := connected.(*conn); !ok || measured.accepted {
		t.Fatal("dialed connection has wrong direction")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	latency := count.latency.Load()
	dialErr = failure
	connected, err = wrapped.DialContext(ctx, "tcp", "example.com:443")
	if connected != nil || !errors.Is(err, failure) {
		t.Fatalf("failed dial = (%v, %v)", connected, err)
	}
	if count.dials.Load() != 2 || count.dialFailures.Load() != 1 {
		t.Fatal("failed attempt not counted")
	}
	if count.latency.Load() != latency || count.latencySum.Load() != latency {
		t.Fatal("failed attempt changed successful dial latency")
	}
	if count.active.Load() != 0 || count.total.Load() != 1 {
		t.Fatal("failed attempt changed connection counts")
	}
}

type listenDialer struct {
	bridge.Dialer
	bridge.ListenConfig
}

func TestHopDialerListen(t *testing.T) {
	for _, test := range []struct {
		name      string
		supported bool
		listenErr error
	}{
		{name: "success", supported: true},
		{name: "failure", supported: true, listenErr: net.ErrClosed},
		{name: "unsupported"},
	} {
		t.Run(test.name, func(t *testing.T) {
			inner, peer := net.Pipe()
			closeOnCleanup(t, inner)
			closeOnCleanup(t, peer)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var base bridge.Dialer = bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("Listen called DialContext")
				return nil, errors.ErrUnsupported
			})
			if test.supported {
				base = &listenDialer{Dialer: base, ListenConfig: bridge.ListenConfigFunc(func(callCtx context.Context, network, address string) (net.Listener, error) {
					if callCtx != ctx || network != "tcp" || address != "127.0.0.1:0" {
						t.Fatal("listen arguments were not forwarded")
					}
					if test.listenErr != nil {
						return nil, test.listenErr
					}
					return &pipeListener{connection: inner}, nil
				})}
			}
			count := &counter{}
			var wrapped bridge.Dialer = &dialer{inner: base, count: count}
			config, ok := wrapped.(bridge.ListenConfig)
			if !ok {
				t.Fatal("hop dialer lost ListenConfig")
			}
			listening, err := config.Listen(ctx, "tcp", "127.0.0.1:0")
			if !test.supported {
				if listening != nil || err == nil || !errors.Is(err, errors.ErrUnsupported) {
					t.Fatalf("unsupported Listen = (%v, %v)", listening, err)
				}
				return
			}
			if test.listenErr != nil {
				if listening != nil || !errors.Is(err, test.listenErr) {
					t.Fatalf("failed Listen = (%v, %v)", listening, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			closeOnCleanup(t, listening)
			accepted, err := listening.Accept()
			if err != nil {
				t.Fatal(err)
			}
			closeOnCleanup(t, accepted)
			if count.active.Load() != 1 || count.total.Load() != 1 || count.dials.Load() != 0 {
				t.Fatal("remote accept not counted independently of dials")
			}
			setDeadlines(t, accepted, peer)
			done := make(chan error, 1)
			go func() {
				_, err := peer.Write([]byte("up"))
				done <- err
			}()
			if size, err := accepted.Read(make([]byte, 2)); err != nil || size != 2 {
				t.Fatalf("accepted Read = (%d, %v)", size, err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if count.up.Load() != 2 || count.down.Load() != 0 {
				t.Fatal("remote accept has wrong byte direction")
			}
		})
	}
}

func TestRuleDialerTargets(t *testing.T) {
	failure := errors.New("dial failed")
	for _, test := range []struct {
		name string
		role Role
		via  string
		url  string
		err  error
	}{
		{name: "forward chain", role: Forward, url: "ssh://user:secret@exit:22", via: "ssh://user:xxxxx@exit:22"},
		{name: "listen chain", role: Listen, url: "ssh://user:secret@exit:22"},
		{name: "direct"},
		{name: "failed direct", err: failure},
		{name: "failed chain", role: Forward, url: "ssh://user:secret@exit:22", via: "ssh://user:xxxxx@exit:22", err: failure},
		{name: "invalid URL", role: Forward, url: "ssh://user:secret@%zz", via: "<invalid url>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			registry := NewRegistry()
			registry.Sync([]config.Rule{{Name: "rule"}})
			rule := registry.Rule("rule")
			inner, peer := net.Pipe()
			closeOnCleanup(t, inner)
			closeOnCleanup(t, peer)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
			ctx = jumpway.WithClientAddr(ctx, client)
			var base bridge.Dialer = bridge.DialFunc(func(callCtx context.Context, network, address string) (net.Conn, error) {
				if callCtx.Done() != ctx.Done() || network != "tcp" || address != "example.com:443" {
					t.Fatal("outer dialer did not preserve context and arguments")
				}
				if test.err != nil {
					return nil, test.err
				}
				return inner, nil
			})
			if test.url != "" {
				wrap := rule.HopWrapper(test.role)
				base = wrap(1, "socks5://entry:1080", base)
				base = wrap(0, test.url, base)
			}
			wrapped := rule.WrapDialer(base)
			before := time.Now()
			connected, err := wrapped.DialContext(ctx, "tcp", "example.com:443")
			after := time.Now()
			if !errors.Is(err, test.err) {
				t.Fatalf("DialContext = %v, want %v", err, test.err)
			}
			if connected != nil {
				closeOnCleanup(t, connected)
				setDeadlines(t, connected, peer)
				done := make(chan error, 1)
				go func() {
					_, err := io.ReadFull(peer, make([]byte, 3))
					done <- err
				}()
				if _, err := connected.Write([]byte("abc")); err != nil {
					t.Fatal(err)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			}
			snapshot := registry.Snapshot().Rules[0]
			if len(snapshot.Targets) != 1 || snapshot.Targets[0].Address != "example.com:443" || snapshot.Targets[0].Via != test.via {
				t.Fatalf("target trace = %+v, want via %q", snapshot.Targets, test.via)
			}
			target := snapshot.Targets[0].Stats
			if snapshot.Stats.Dials != 1 || target.Dials != 1 || snapshot.Stats.LatencyMs != target.LatencyMs {
				t.Fatal("rule and target dial measurements differ")
			}
			if snapshot.Stats.Total != 0 || snapshot.Stats.Active != 0 || snapshot.Stats.Up != 0 || snapshot.Stats.Down != 0 {
				t.Fatal("outer dialer counted rule bytes or connections")
			}
			if test.err != nil {
				if snapshot.Stats.DialFailures != 1 || target.DialFailures != 1 || target.Total != 0 || target.LatencyMs != 0 {
					t.Fatalf("failed target = %+v", target)
				}
				if snapshot.Connections == nil || len(snapshot.Connections) != 0 {
					t.Fatalf("failed dial connections = %+v, want empty list", snapshot.Connections)
				}
			} else {
				if target.Total != 1 || target.Active != 1 || target.Up != 3 || target.LatencyMs <= 0 || target.AvgLatencyMs != target.LatencyMs {
					t.Fatalf("successful target = %+v", target)
				}
				if len(snapshot.Connections) != 1 {
					t.Fatalf("connections = %+v, want one live connection", snapshot.Connections)
				}
				connection := snapshot.Connections[0]
				if connection.ID != 1 || connection.Client != client.String() || connection.Target != "example.com:443" || connection.Via != test.via {
					t.Fatalf("connection = %+v, want client %q and via %q", connection, client.String(), test.via)
				}
				data, err := json.Marshal(connection)
				if err != nil {
					t.Fatal(err)
				}
				var body struct {
					Path []map[string]any `json:"path"`
				}
				if err := json.Unmarshal(data, &body); err != nil {
					t.Fatal(err)
				}
				path := []map[string]any{}
				if test.role == Forward && test.url != "" {
					path = append(path,
						map[string]any{"index": float64(0), "url": test.via, "dialed": true},
						map[string]any{"index": float64(1), "url": "socks5://entry:1080", "dialed": true},
					)
				}
				if !reflect.DeepEqual(body.Path, path) {
					t.Fatalf("connection path = %+v, want %+v; JSON: %s", body.Path, path, data)
				}
				started, err := time.Parse(time.RFC3339Nano, connection.Started)
				if err != nil || started.Before(before) || started.After(after) || connection.Started != started.UTC().Format(time.RFC3339Nano) {
					t.Fatalf("connection started = %q, want UTC dial time between %v and %v", connection.Started, before, after)
				}
				if err := connected.Close(); err != nil {
					t.Fatal(err)
				}
				if registry.Snapshot().Rules[0].Targets[0].Stats.Active != 0 {
					t.Fatal("target remained active after close")
				}
				if connections := registry.Snapshot().Rules[0].Connections; connections == nil || len(connections) != 0 {
					t.Fatalf("connections after close = %+v, want empty list", connections)
				}
			}
			registry.Sync([]config.Rule{{Name: "rule"}})
			if retained := registry.Snapshot().Rules[0].Targets; len(retained) != 1 || retained[0].Stats.Dials != 1 {
				t.Fatal("reload dropped targets")
			}
		})
	}
}

func TestTargetViaUnderFailover(t *testing.T) {
	for _, test := range []struct {
		name    string
		goodErr error
	}{
		{name: "second URL connects"},
		{name: "every URL fails", goodErr: errors.New("offline")},
	} {
		t.Run(test.name, func(t *testing.T) {
			registry := NewRegistry()
			registry.Sync([]config.Rule{{Name: "rule"}})
			rule := registry.Rule("rule")
			wrap := rule.HopWrapper(Forward)
			entry := wrap(1, "socks5://entry:1080", bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
				if test.goodErr != nil {
					return nil, test.goodErr
				}
				inner, peer := net.Pipe()
				closeOnCleanup(t, inner)
				closeOnCleanup(t, peer)
				return inner, nil
			}))
			bad := wrap(0, "ssh://user:secret@bad:22", bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
				return nil, errors.New("bad exit")
			}))
			good := wrap(0, "ssh://user:secret@good:22", entry)
			// Mimics bridge's backoffManager: the exit node tries its next LB URL inside one dial.
			node := bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
				if connected, err := bad.DialContext(ctx, network, address); err == nil {
					return connected, nil
				}
				return good.DialContext(ctx, network, address)
			})
			connected, err := rule.WrapDialer(node).DialContext(context.Background(), "tcp", "example.com:443")
			if !errors.Is(err, test.goodErr) {
				t.Fatalf("DialContext = %v, want %v", err, test.goodErr)
			}
			if connected != nil {
				closeOnCleanup(t, connected)
			}
			snapshot := registry.Snapshot().Rules[0]
			targets := snapshot.Targets
			if len(targets) != 1 || targets[0].Via != "ssh://user:xxxxx@good:22" {
				t.Fatalf("targets = %+v, want via the URL that was tried last", targets)
			}
			if test.goodErr == nil {
				path := []PathHop{
					{Index: 0, URL: "ssh://user:xxxxx@good:22", Dialed: true},
					{Index: 1, URL: "socks5://entry:1080", Dialed: true},
				}
				if len(snapshot.Connections) != 1 || !reflect.DeepEqual(snapshot.Connections[0].Path, path) {
					t.Fatalf("connections = %+v, want path %+v", snapshot.Connections, path)
				}
			}
		})
	}
}

func TestConnectionPathCachedTransport(t *testing.T) {
	const exit = "socks5://exit:1080"
	const entry = "ssh://user:secret@entry:22"
	const redacted = "ssh://user:xxxxx@entry:22"
	for _, test := range []struct {
		name string
		urls []string
		url  string
	}{
		{name: "single URL", urls: []string{entry}, url: redacted},
		{name: "multiple URLs", urls: []string{entry, "ssh://other:22"}},
		{name: "duplicate URLs", urls: []string{entry, entry}, url: redacted},
		{name: "duplicate redacted URLs", urls: []string{entry, "ssh://user:other-password@entry:22"}, url: redacted},
		{name: "changed URL", urls: []string{"ssh://other:22"}, url: "ssh://other:22"},
		{name: "no URLs"},
	} {
		t.Run(test.name, func(t *testing.T) {
			registry := NewRegistry()
			configured := config.Rule{Name: "rule", Forward: config.Forward{Way: []bridgeconfig.Node{
				{LB: []string{exit}},
				{LB: []string{entry}},
			}}}
			registry.Sync([]config.Rule{configured})
			rule := registry.Rule("rule")
			wrap := rule.HopWrapper(Forward)
			calls := 0
			inner := wrap(1, entry, bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
				calls++
				connected, peer := net.Pipe()
				closeOnCleanup(t, connected)
				closeOnCleanup(t, peer)
				return connected, nil
			}))
			var cached net.Conn
			outer := wrap(0, exit, bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
				if cached == nil {
					connected, err := inner.DialContext(ctx, network, address)
					if err != nil {
						return nil, err
					}
					cached = connected
				}
				return cached, nil
			}))
			wrapped := rule.WrapDialer(outer)
			first, err := wrapped.DialContext(context.Background(), "tcp", "example.com:443")
			if err != nil {
				t.Fatal(err)
			}
			closeOnCleanup(t, first)
			path := []PathHop{
				{Index: 0, URL: exit, Dialed: true},
				{Index: 1, URL: redacted, Dialed: true},
			}
			before := registry.Snapshot().Rules[0].Connections
			if len(before) != 1 || !reflect.DeepEqual(before[0].Path, path) {
				t.Fatalf("first connections = %+v, want path %+v", before, path)
			}
			configured.Forward.Way[1].LB = test.urls
			registry.Sync([]config.Rule{configured})
			second, err := wrapped.DialContext(context.Background(), "tcp", "other.example:443")
			if err != nil {
				t.Fatal(err)
			}
			closeOnCleanup(t, second)
			if calls != 1 {
				t.Fatalf("entry dial calls = %d, want one reused transport", calls)
			}
			connections := registry.Snapshot().Rules[0].Connections
			if len(connections) != 2 || !reflect.DeepEqual(connections[0].Path, path) {
				t.Fatalf("connections = %+v, want first path retained as %+v", connections, path)
			}
			path[1] = PathHop{Index: 1, URL: test.url, Dialed: false}
			if connections[1].Via != exit || !reflect.DeepEqual(connections[1].Path, path) {
				t.Fatalf("second connection = %+v, want via %q and path %+v", connections[1], exit, path)
			}
			connections[1].Path[1].URL = "changed"
			if !reflect.DeepEqual(registry.Snapshot().Rules[0].Connections[1].Path, path) {
				t.Fatal("snapshot shares live path storage")
			}
		})
	}
}

func TestTargetViaSeparatesSameAddress(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	base := bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("offline")
	})
	for _, raw := range []string{"ssh://user:secret@exit-a:22", "ssh://user:secret@exit-b:22", ""} {
		var inner bridge.Dialer = base
		if raw != "" {
			inner = rule.HopWrapper(Forward)(0, raw, inner)
		}
		wrapped := rule.WrapDialer(inner)
		for range 2 {
			if connected, err := wrapped.DialContext(context.Background(), "tcp", "same:443"); connected != nil || err == nil {
				t.Fatalf("expected failed dial, got (%v, %v)", connected, err)
			}
		}
	}
	if targets := registry.Snapshot().Rules[0].Targets; len(targets) != 3 {
		t.Fatalf("target count = %d, want 3", len(targets))
	} else {
		for _, target := range targets {
			if target.Stats.Dials != 2 || target.Stats.DialFailures != 2 {
				t.Fatalf("target attempts = %+v", target)
			}
		}
	}
}

func TestLiveConnectionTraffic(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}})
	rule := registry.Rule("rule")
	first, firstPeer := dialRulePipe(t, rule, "example.com:443")
	second, secondPeer := dialRulePipe(t, rule, "example.com:443")
	start := time.Now()
	registry.tick(start)
	transferBytes(t, first, firstPeer, 3)
	transferBytes(t, firstPeer, first, 5)
	transferBytes(t, second, secondPeer, 7)
	transferBytes(t, secondPeer, second, 11)
	registry.tick(start.Add(time.Second))
	snapshot := registry.Snapshot().Rules[0]
	if len(snapshot.Connections) != 2 {
		t.Fatalf("connections = %+v, want two connections", snapshot.Connections)
	}
	for index, want := range []struct{ up, down int64 }{{3, 5}, {7, 11}} {
		connection := snapshot.Connections[index]
		if connection.Stats.Up != want.up || connection.Stats.Down != want.down || connection.Stats.RateUp != want.up || connection.Stats.RateDown != want.down {
			t.Fatalf("connection %d = %+v, want up/rate_up %d and down/rate_down %d", index, connection, want.up, want.down)
		}
		if connection.Client != "" {
			t.Fatalf("client = %q, want absent address", connection.Client)
		}
	}
	if len(snapshot.Targets) != 1 {
		t.Fatalf("targets = %+v, want one aggregate", snapshot.Targets)
	}
	if stats := snapshot.Targets[0].Stats; stats.Up != 10 || stats.Down != 16 || stats.RateUp != 10 || stats.RateDown != 16 || stats.Active != 2 || stats.Total != 2 {
		t.Fatalf("target stats = %+v, want the sum of both connections", stats)
	}
	registry.tick(start.Add(2 * time.Second))
	for _, connection := range registry.Snapshot().Rules[0].Connections {
		if connection.Stats.RateUp != 0 || connection.Stats.RateDown != 0 {
			t.Fatalf("idle connection rates = %+v, want zero", connection)
		}
	}
}

func dialRulePipe(t *testing.T, rule *Rule, address string) (net.Conn, net.Conn) {
	t.Helper()
	inner, peer := net.Pipe()
	closeOnCleanup(t, inner)
	closeOnCleanup(t, peer)
	wrapped := rule.WrapDialer(bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
		return inner, nil
	}))
	connected, err := wrapped.DialContext(context.Background(), "tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, connected)
	setDeadlines(t, connected, peer)
	return connected, peer
}
