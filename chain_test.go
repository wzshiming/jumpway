package jumpway

import (
	"context"
	"errors"
	"io"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/sshproxy"
	"golang.org/x/crypto/ssh"
)

func TestNewChainDialerHopOrder(t *testing.T) {
	type hop struct {
		index int
		url   string
	}
	var mu sync.Mutex
	var calls []string
	var built []string
	var wrapped []hop
	ctx := context.Background()
	target := "target.example:443"
	base := bridge.DialFunc(func(callCtx context.Context, network, address string) (net.Conn, error) {
		if callCtx != ctx || network != "tcp" || address != target {
			t.Error("dial arguments were not forwarded to the base dialer")
		}
		mu.Lock()
		calls = append(calls, "base")
		mu.Unlock()
		client, server := net.Pipe()
		server.Close()
		return client, nil
	})
	if err := chain.Default.Register("testhop", bridge.BridgeFunc(func(ctx context.Context, dialer bridge.Dialer, address string) (bridge.Dialer, error) {
		mu.Lock()
		built = append(built, address)
		mu.Unlock()
		return bridge.DialFunc(func(ctx context.Context, network, target string) (net.Conn, error) {
			mu.Lock()
			calls = append(calls, address)
			mu.Unlock()
			return dialer.DialContext(ctx, network, target)
		}), nil
	})); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name      string
		way       []config.Node
		wrap      bool
		dials     int
		wantCalls []string
	}{
		{
			name: "two_hops",
			way: []config.Node{
				{LB: []string{"testhop://exit"}},
				{LB: []string{"testhop://entry"}},
			},
			wrap:      true,
			dials:     2,
			wantCalls: slices.Repeat([]string{"testhop://exit", "testhop://entry", "base"}, 2),
		},
		{
			name: "nil_wrapper",
			way: []config.Node{
				{LB: []string{"testhop://exit"}},
				{LB: []string{"testhop://entry"}},
			},
			dials:     2,
			wantCalls: slices.Repeat([]string{"testhop://exit", "testhop://entry", "base"}, 2),
		},
		{
			name:      "load_balancing",
			way:       []config.Node{{LB: []string{"testhop://a", "testhop://b"}}},
			wrap:      true,
			dials:     4,
			wantCalls: slices.Repeat([]string{"testhop://a", "base", "testhop://b", "base"}, 2),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			mu.Lock()
			calls, built, wrapped = nil, nil, nil
			mu.Unlock()
			var wrap HopWrapper
			if testCase.wrap {
				wrap = func(index int, url string, dialer bridge.Dialer) bridge.Dialer {
					mu.Lock()
					wrapped = append(wrapped, hop{index: index, url: url})
					mu.Unlock()
					return dialer
				}
			}
			dialer, err := NewChainDialer(ctx, base, testCase.way, wrap)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := dialer.(bridge.ListenConfig); !ok {
				t.Fatalf("non-empty chain %T does not implement bridge.ListenConfig", dialer)
			}
			mu.Lock()
			if len(wrapped) != 0 || len(calls) != 0 || len(built) != 0 {
				t.Errorf("chain construction built %v, wrapped %v and dialed %v, want lazy construction", built, wrapped, calls)
			}
			mu.Unlock()
			for range testCase.dials {
				conn, err := dialer.DialContext(ctx, "tcp", target)
				if err != nil {
					t.Fatal(err)
				}
				conn.Close()
			}
			var wantBuilt []string
			var wantWrapped []hop
			for index, node := range testCase.way {
				for _, url := range node.LB {
					wantBuilt = append(wantBuilt, url)
					if testCase.wrap {
						wantWrapped = append(wantWrapped, hop{index: index, url: url})
					}
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if !slices.Equal(built, wantBuilt) {
				t.Errorf("built hops = %v, want %v", built, wantBuilt)
			}
			if !slices.Equal(wrapped, wantWrapped) {
				t.Errorf("wrapped hops = %v, want %v", wrapped, wantWrapped)
			}
			if !slices.Equal(calls, testCase.wantCalls) {
				t.Errorf("dial order = %v, want %v", calls, testCase.wantCalls)
			}
		})
	}
}

func TestNewChainDialerEmpty(t *testing.T) {
	base := &net.Dialer{}
	for _, way := range [][]config.Node{nil, {}} {
		dialer, err := NewChainDialer(context.Background(), base, way, func(index int, url string, dialer bridge.Dialer) bridge.Dialer {
			t.Error("empty way invoked the hop wrapper")
			return dialer
		})
		if err != nil {
			t.Fatal(err)
		}
		if dialer != base {
			t.Fatalf("empty way returned %v, want the base dialer %v", dialer, base)
		}
	}
}

func TestNewChainDialerUnsupportedScheme(t *testing.T) {
	ctx := context.Background()
	way := []config.Node{{LB: []string{"testunsupported://proxy"}}}
	dialer, err := NewChainDialer(ctx, &net.Dialer{}, way, nil)
	if err != nil {
		t.Fatalf("unsupported scheme failed at build time: %v", err)
	}
	conn, err := dialer.DialContext(ctx, "tcp", "target.example:443")
	if conn != nil {
		conn.Close()
		t.Error("unsupported scheme returned a connection")
	}
	if err == nil || !strings.Contains(err.Error(), "testunsupported") {
		t.Fatalf("dial error = %v, want an error mentioning the unsupported scheme", err)
	}
}

func TestNewChainDialerSSHHopSurvivesRejectedDial(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		for {
			conn, err := echo.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				io.Copy(conn, conn)
			}(conn)
		}
	}()
	server, err := sshproxy.NewSimpleServer("ssh://u:p@127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server.Context = ctx
	if err := server.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	proxyURL := server.ProxyURL()
	dialer, err := NewChainDialer(ctx, local.LOCAL, []config.Node{{LB: []string{proxyURL}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := dialer.DialContext(ctx, "tcp", echo.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	roundTrip := func(message string) {
		t.Helper()
		if err := first.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			guard := time.AfterFunc(2*time.Second, func() { first.Close() })
			defer guard.Stop()
		}
		if _, err := io.WriteString(first, message); err != nil {
			t.Fatalf("first SSH tunnel write %q: %v", message, err)
		}
		reply := make([]byte, len(message))
		if _, err := io.ReadFull(first, reply); err != nil {
			t.Fatalf("first SSH tunnel read %q: %v", message, err)
		}
		if string(reply) != message {
			t.Fatalf("first SSH tunnel echoed %q, want %q", reply, message)
		}
	}
	roundTrip("before rejected dial\n")
	rejected, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:1")
	if rejected != nil {
		rejected.Close()
		t.Fatal("rejected dial returned a connection")
	}
	var channelError *ssh.OpenChannelError
	if !errors.As(err, &channelError) {
		t.Fatalf("rejected dial error = %v, want *ssh.OpenChannelError", err)
	}
	roundTrip("after rejected dial\n")
}
