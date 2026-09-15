package jumpway

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/protocols/local"
	_ "github.com/wzshiming/bridge/protocols/socks5"
	_ "github.com/wzshiming/bridge/protocols/ssh"
	"github.com/wzshiming/hostmatcher"
	_ "github.com/wzshiming/sshd/directtcp"
	_ "github.com/wzshiming/sshd/tcpforward"
	"github.com/wzshiming/sshproxy"
)

func TestNewListenConfigLocal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	listenConfig, err := NewListenConfig(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if listenConfig != local.LOCAL {
		t.Fatalf("NewListenConfig(nil) = %v, want local.LOCAL", listenConfig)
	}
	listener, err := listenConfig.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}

func TestNewListenConfigIgnoresDialRouting(t *testing.T) {
	originalNoProxy, originalOnlyProxy := chain.NoProxy, chain.OnlyProxy
	t.Cleanup(func() {
		chain.NoProxy, chain.OnlyProxy = originalNoProxy, originalOnlyProxy
	})
	chain.NoProxy = hostmatcher.NewMatcher([]string{"*"})
	chain.OnlyProxy = hostmatcher.NewMatcher([]string{"*"})
	listenConfig, err := NewListenConfig(context.Background(), []config.Node{{LB: []string{"ssh://u:p@127.0.0.1:1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if listenConfig == nil {
		t.Fatal("NewListenConfig returned a nil listener with environment routing enabled")
	}
	if chain.Default.DialerFunc == nil {
		t.Fatal("NewListenConfig changed the default chain's dial routing")
	}
}

func TestNewListenConfigCannotListen(t *testing.T) {
	for _, scheme := range []string{"socks4", "socks4a", "socks5", "socks5h"} {
		t.Run(scheme, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			listenConfig, err := NewListenConfig(ctx, []config.Node{{LB: []string{scheme + "://127.0.0.1:1"}}})
			if err == nil {
				listener, listenErr := listenConfig.Listen(ctx, "tcp", "127.0.0.1:0")
				if listener != nil {
					listener.Close()
				}
				err = listenErr
			}
			if err == nil || !strings.Contains(err.Error(), "listen") {
				t.Fatalf("non-listening hop error = %v, want an error containing listen", err)
			}
		})
	}
	t.Run("socks_transit_hop", func(t *testing.T) {
		way := []config.Node{
			{LB: []string{"ssh://u:p@127.0.0.1:1"}},
			{LB: []string{"socks5://127.0.0.1:1"}},
		}
		if _, err := NewListenConfig(context.Background(), way); err != nil {
			t.Fatalf("SOCKS transit hop should not prevent constructing an SSH listen chain: %v", err)
		}
	})
}

func TestNewListenConfigSSHRemoteBind(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server, err := sshproxy.NewSimpleServer("ssh://u:p@127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server.Context = ctx
	var binds atomic.Int32
	server.ProxyListen = func(callCtx context.Context, network, address string) (net.Listener, error) {
		_, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		listener, err := local.LOCAL.Listen(callCtx, network, net.JoinHostPort("127.0.0.1", port))
		if err == nil {
			binds.Add(1)
		}
		return listener, err
	}
	if err := server.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	listenConfig, err := NewListenConfig(ctx, []config.Node{{LB: []string{server.ProxyURL()}}})
	if err != nil {
		t.Fatal(err)
	}
	loopbackAddress := func(address net.Addr) string {
		t.Helper()
		_, port, err := net.SplitHostPort(address.String())
		if err != nil {
			t.Fatal(err)
		}
		return net.JoinHostPort("127.0.0.1", port)
	}
	checkResponse := func(t *testing.T, client *http.Client, address, want string) {
		t.Helper()
		response, err := client.Get(address)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK || string(body) != want {
			t.Fatalf("GET %s = %d %q, want 200 %q", address, response.StatusCode, body, want)
		}
	}
	t.Run("http", func(t *testing.T) {
		listener, err := listenConfig.Listen(ctx, "tcp", "0.0.0.0:0")
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			done <- http.Serve(listener, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				io.WriteString(writer, "remote-ok")
			}))
		}()
		t.Cleanup(func() {
			listener.Close()
			select {
			case err := <-done:
				if err == nil {
					t.Error("http.Serve returned nil after its listener closed")
				}
			case <-time.After(time.Second):
				t.Error("http.Serve did not stop after its remote listener closed")
			}
		})
		transport := &http.Transport{DisableKeepAlives: true}
		t.Cleanup(transport.CloseIdleConnections)
		client := &http.Client{Transport: transport, Timeout: time.Second}
		checkResponse(t, client, "http://"+loopbackAddress(listener.Addr()), "remote-ok")
	})
	t.Run("proxy", func(t *testing.T) {
		proxyCtx, proxyCancel := context.WithCancel(ctx)
		defer proxyCancel()
		events := make(chan Event, 4)
		done := make(chan error, 1)
		go func() {
			done <- Serve(proxyCtx, func(callCtx context.Context) (net.Listener, error) {
				return listenConfig.Listen(callCtx, "tcp", "0.0.0.0:0")
			}, func(callCtx context.Context, listener net.Listener) error {
				return RunProxy(callCtx, listener, local.LOCAL, nil)
			}, func(event Event) {
				select {
				case events <- event:
				case <-proxyCtx.Done():
				}
			})
		}()
		t.Cleanup(func() {
			proxyCancel()
			select {
			case err := <-done:
				if err != proxyCtx.Err() || !errors.Is(err, context.Canceled) {
					t.Errorf("Serve() = %v, want context cancellation", err)
				}
			case <-time.After(time.Second):
				t.Error("Serve did not stop after canceling its remote listener")
			}
		})
		var address string
		select {
		case event := <-events:
			if event.Err != nil || event.Addr == nil {
				t.Fatalf("first remote proxy event = %v, want listening address", event)
			}
			address = loopbackAddress(event.Addr)
		case <-time.After(time.Second):
			t.Fatal("Serve did not report a remotely bound proxy address")
		}
		target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			io.WriteString(writer, "remote-proxy-ok")
		}))
		t.Cleanup(target.Close)
		proxyURL := &url.URL{Scheme: "http", Host: address}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
		t.Cleanup(transport.CloseIdleConnections)
		client := &http.Client{Transport: transport, Timeout: time.Second}
		checkResponse(t, client, target.URL, "remote-proxy-ok")
	})
	if count := binds.Load(); count != 2 {
		t.Errorf("SSH server binds = %d, want 2", count)
	}
}

type closableListener struct {
	net.Listener
	mu    sync.Mutex
	conns []net.Conn
}

func (l *closableListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil {
		l.mu.Lock()
		l.conns = append(l.conns, conn)
		l.mu.Unlock()
	}
	return conn, err
}

func (l *closableListener) closeConns() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, conn := range l.conns {
		conn.Close()
	}
}

// A remote listener must report the loss of its SSH connection and close without hanging (x/crypto < 0.55 spun forever).
func TestSSHRemoteListenerSurvivesConnectionLoss(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepting := &closableListener{Listener: raw}
	server, err := sshproxy.NewSimpleServer("ssh://u:p@" + raw.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	server.Listener = accepting
	server.Context = ctx
	server.ProxyListen = func(callCtx context.Context, network, address string) (net.Listener, error) {
		_, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		return local.LOCAL.Listen(callCtx, network, net.JoinHostPort("127.0.0.1", port))
	}
	if err := server.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	listenConfig, err := NewListenConfig(ctx, []config.Node{{LB: []string{server.ProxyURL()}}})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := listenConfig.Listen(ctx, "tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan error, 1)
	go func() {
		_, err := listener.Accept()
		accepted <- err
	}()
	accepting.closeConns()
	select {
	case err := <-accepted:
		if err == nil {
			t.Fatal("Accept returned a connection after the SSH connection was closed")
		}
	case <-ctx.Done():
		t.Fatal("Accept did not return after the SSH connection was closed")
	}
	closed := make(chan struct{})
	go func() {
		listener.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-ctx.Done():
		t.Fatal("Close hung after the SSH connection was closed")
	}
}
