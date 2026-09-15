package metrics

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/wzshiming/bridge"
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
		{name: "listen chain", role: Listen, url: "ssh://user:secret@exit:22", via: "ssh://user:xxxxx@exit:22"},
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
			connected, err := wrapped.DialContext(ctx, "tcp", "example.com:443")
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
			} else {
				if target.Total != 1 || target.Active != 1 || target.Up != 3 || target.LatencyMs <= 0 || target.AvgLatencyMs != target.LatencyMs {
					t.Fatalf("successful target = %+v", target)
				}
				if err := connected.Close(); err != nil {
					t.Fatal(err)
				}
				if registry.Snapshot().Rules[0].Targets[0].Stats.Active != 0 {
					t.Fatal("target remained active after close")
				}
			}
			registry.Sync([]config.Rule{{Name: "rule"}})
			if retained := registry.Snapshot().Rules[0].Targets; len(retained) != 1 || retained[0].Stats.Dials != 1 {
				t.Fatal("reload dropped targets")
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
