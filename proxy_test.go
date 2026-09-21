package jumpway

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/jumpway/utils"
	"github.com/wzshiming/shadowsocks"
	"github.com/wzshiming/socks4"
	"github.com/wzshiming/socks5"
	"github.com/wzshiming/sshproxy"
)

var testShadowsocksUser = url.UserPassword("aes-256-gcm", "s3cr3t")

func TestRunProxyClientAddr(test *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, "proxy-ok")
	}))
	test.Cleanup(target.Close)
	for _, protocol := range []string{"http_connect", "http_get", "socks5", "socks4", "ssh", "ss"} {
		test.Run(protocol, func(test *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				test.Fatal(err)
			}
			clients := make(chan string, 1)
			dialer := bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
				clients <- ClientAddr(ctx)
				return local.LOCAL.DialContext(ctx, network, address)
			})
			done := make(chan error, 1)
			go func() { done <- RunProxy(ctx, listener, dialer, nil, testShadowsocksUser) }()
			test.Cleanup(func() {
				cancel()
				listener.Close()
				select {
				case err := <-done:
					if err == nil || !utils.IsClosedConnError(err) {
						test.Errorf("RunProxy() = %v, want closed connection error", err)
					}
				case <-time.After(time.Second):
					test.Error("RunProxy did not stop after closing its listener")
				}
			})
			addresses := make(chan string, 1)
			dialClient := func(ctx context.Context, network, address string) (net.Conn, error) {
				conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
				if err != nil {
					return nil, err
				}
				test.Cleanup(func() { conn.Close() })
				if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
					return nil, err
				}
				addresses <- conn.LocalAddr().String()
				return conn, nil
			}
			address := listener.Addr().String()
			var proxy bridge.Dialer
			switch protocol {
			case "http_connect":
				conn, err := dialClient(ctx, "tcp", address)
				if err != nil {
					test.Fatal(err)
				}
				if _, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target.Listener.Addr(), target.Listener.Addr()); err != nil {
					test.Fatal(err)
				}
				response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
				if err != nil {
					test.Fatal(err)
				}
				if response.StatusCode != http.StatusOK {
					test.Fatalf("CONNECT status = %d, want 200", response.StatusCode)
				}
			case "http_get":
				transport := &http.Transport{
					Proxy:             http.ProxyURL(&url.URL{Scheme: "http", Host: address}),
					DialContext:       dialClient,
					DisableKeepAlives: true,
				}
				test.Cleanup(transport.CloseIdleConnections)
				client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
				response, err := client.Get(target.URL)
				if err != nil {
					test.Fatal(err)
				}
				defer response.Body.Close()
				body, err := io.ReadAll(response.Body)
				if err != nil {
					test.Fatal(err)
				}
				if response.StatusCode != http.StatusOK || string(body) != "proxy-ok" {
					test.Fatalf("GET response = %d %q, want 200 proxy-ok", response.StatusCode, body)
				}
			case "socks5":
				client, err := socks5.NewDialer("socks5://" + address)
				if err != nil {
					test.Fatal(err)
				}
				client.ProxyDial = dialClient
				proxy = client
			case "socks4":
				client, err := socks4.NewDialer("socks4://" + address)
				if err != nil {
					test.Fatal(err)
				}
				client.ProxyDial = dialClient
				proxy = client
			case "ssh":
				client, err := sshproxy.NewDialer("ssh://" + address)
				if err != nil {
					test.Fatal(err)
				}
				test.Cleanup(func() { client.Close() })
				client.ProxyDial = dialClient
				proxy = client
			case "ss":
				client, err := shadowsocks.NewDialer("ss://" + testShadowsocksUser.String() + "@" + address)
				if err != nil {
					test.Fatal(err)
				}
				client.ProxyDial = dialClient
				proxy = client
			}
			if proxy != nil {
				conn, err := proxy.DialContext(ctx, "tcp", target.Listener.Addr().String())
				if err != nil {
					test.Fatal(err)
				}
				test.Cleanup(func() { conn.Close() })
			}
			var want string
			select {
			case want = <-addresses:
			case <-ctx.Done():
				test.Fatal("client did not connect to the proxy")
			}
			select {
			case got := <-clients:
				if got != want {
					test.Fatalf("ClientAddr() = %q, want %q", got, want)
				}
			case <-ctx.Done():
				test.Fatal("proxy did not dial the target")
			}
		})
	}
}

func TestRunProxyAuth(t *testing.T) {
	checkProxy := func(t *testing.T, scheme string, user, clientUser *url.Userinfo, wantStatus int) {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- RunProxy(ctx, listener, local.LOCAL, user, testShadowsocksUser) }()
		t.Cleanup(func() {
			cancel()
			listener.Close()
			select {
			case err := <-done:
				if err == nil || !utils.IsClosedConnError(err) {
					t.Errorf("RunProxy() = %v, want closed connection error", err)
				}
			case <-time.After(time.Second):
				t.Error("RunProxy did not stop after closing its listener")
			}
		})
		target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/hello" {
				t.Errorf("target path = %q, want /hello", request.URL.Path)
			}
			io.WriteString(writer, "proxy-ok")
		}))
		t.Cleanup(target.Close)
		proxyURL := &url.URL{Scheme: scheme, Host: listener.Addr().String(), User: clientUser}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
		t.Cleanup(transport.CloseIdleConnections)
		dialClient := func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			t.Cleanup(func() { conn.Close() })
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return nil, err
			}
			return conn, nil
		}
		switch scheme {
		case "socks5":
			proxy, err := socks5.NewDialer(proxyURL.String())
			if err != nil {
				t.Fatal(err)
			}
			proxy.ProxyDial = dialClient
			transport.Proxy = nil
			transport.DialContext = proxy.DialContext
		case "ssh":
			proxy, err := sshproxy.NewDialer(proxyURL.String())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { proxy.Close() })
			proxy.ProxyDial = dialClient
			transport.Proxy = nil
			transport.DialContext = proxy.DialContext
		case "ss":
			proxy, err := shadowsocks.NewDialer(proxyURL.String())
			if err != nil {
				t.Fatal(err)
			}
			proxy.ProxyDial = dialClient
			transport.Proxy = nil
			transport.DialContext = proxy.DialContext
		}
		client := &http.Client{Transport: transport, Timeout: time.Second}
		response, err := client.Get(target.URL + "/hello")
		if scheme != "http" && wantStatus != http.StatusOK {
			if err == nil {
				response.Body.Close()
				t.Fatal("request succeeded with invalid credentials")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != wantStatus {
			t.Fatalf("status = %d, want %d", response.StatusCode, wantStatus)
		}
		if wantStatus == http.StatusOK && string(body) != "proxy-ok" {
			t.Fatalf("body = %q, want proxy-ok", body)
		}
	}
	user := url.UserPassword("alice", "s3cr3t")
	for _, scheme := range []string{"http", "socks5", "ssh"} {
		t.Run(scheme, func(t *testing.T) {
			t.Run("missing_credentials", func(t *testing.T) {
				checkProxy(t, scheme, user, nil, http.StatusProxyAuthRequired)
			})
			t.Run("valid_credentials", func(t *testing.T) {
				checkProxy(t, scheme, user, user, http.StatusOK)
			})
			t.Run("invalid_credentials", func(t *testing.T) {
				checkProxy(t, scheme, user, url.UserPassword("alice", "wrong"), http.StatusProxyAuthRequired)
			})
			t.Run("without_auth", func(t *testing.T) {
				checkProxy(t, scheme, nil, nil, http.StatusOK)
			})
			t.Run("escaped_credentials", func(t *testing.T) {
				escaped := url.UserPassword("alice@example.com", "s3cr3t:@/?#%")
				checkProxy(t, scheme, escaped, escaped, http.StatusOK)
			})
			if scheme == "http" {
				t.Run("unknown_user_empty_password", func(t *testing.T) {
					checkProxy(t, scheme, user, url.UserPassword("mallory", ""), http.StatusProxyAuthRequired)
				})
			}
		})
	}
	// The Shadowsocks credentials are the cipher and the shared password; a dial only fails once the request is read.
	t.Run("ss", func(t *testing.T) {
		t.Run("valid_credentials", func(t *testing.T) {
			checkProxy(t, "ss", user, testShadowsocksUser, http.StatusOK)
		})
		t.Run("invalid_credentials", func(t *testing.T) {
			checkProxy(t, "ss", user, url.UserPassword("aes-256-gcm", "wrong"), http.StatusProxyAuthRequired)
		})
	})
}

type virtualTestAddr struct{}

func (virtualTestAddr) Network() string { return "virtual" }

func (virtualTestAddr) String() string { return "virtual://x" }

type virtualTestListener struct {
	conns chan net.Conn
	done  chan struct{}
}

func (l *virtualTestListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *virtualTestListener) Close() error {
	select {
	case <-l.done:
	default:
		close(l.done)
	}
	return nil
}

func (l *virtualTestListener) Addr() net.Addr { return virtualTestAddr{} }

func TestRunProxyVirtualListener(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, "proxy-ok")
	}))
	t.Cleanup(target.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	listener := &virtualTestListener{conns: make(chan net.Conn), done: make(chan struct{})}
	clients := make(chan string, 1)
	dialer := bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		clients <- ClientAddr(ctx)
		return local.LOCAL.DialContext(ctx, network, address)
	})
	done := make(chan error, 1)
	go func() { done <- RunProxy(ctx, listener, dialer, url.UserPassword("alice", "s3cr3t"), nil) }()
	t.Cleanup(func() {
		cancel()
		listener.Close()
		select {
		case err := <-done:
			if err == nil || !utils.IsClosedConnError(err) {
				t.Errorf("RunProxy() = %v, want closed connection error", err)
			}
		case <-time.After(time.Second):
			t.Error("RunProxy did not stop after closing its listener")
		}
	})
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close() })
	select {
	case listener.conns <- server:
	case <-ctx.Done():
		t.Fatal("RunProxy did not accept the virtual connection")
	}
	if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	credentials := base64.StdEncoding.EncodeToString([]byte("alice:s3cr3t"))
	if _, err := fmt.Fprintf(client, "CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: Basic %s\r\n\r\n", target.Listener.Addr(), target.Listener.Addr(), credentials); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(client), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d, want 200", response.StatusCode)
	}
	select {
	case got := <-clients:
		if want := server.RemoteAddr().String(); got != want {
			t.Fatalf("ClientAddr() = %q, want %q", got, want)
		}
	case <-ctx.Done():
		t.Fatal("proxy did not dial the target")
	}
}
