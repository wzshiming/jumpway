package jumpway

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	_ "github.com/wzshiming/anyproxy/proxies/httpproxy"
	_ "github.com/wzshiming/anyproxy/proxies/socks4"
	_ "github.com/wzshiming/anyproxy/proxies/socks5"
	_ "github.com/wzshiming/anyproxy/proxies/sshproxy"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/jumpway/utils"
)

func TestRunProxyAuth(t *testing.T) {
	checkProxy := func(t *testing.T, user, clientUser *url.Userinfo, wantStatus int) {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- RunProxy(ctx, listener, local.LOCAL, user) }()
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
		proxyURL := &url.URL{Scheme: "http", Host: listener.Addr().String(), User: clientUser}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
		t.Cleanup(transport.CloseIdleConnections)
		client := &http.Client{Transport: transport, Timeout: time.Second}
		response, err := client.Get(target.URL + "/hello")
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
	t.Run("missing_credentials", func(t *testing.T) {
		checkProxy(t, user, nil, http.StatusProxyAuthRequired)
	})
	t.Run("valid_credentials", func(t *testing.T) {
		checkProxy(t, user, user, http.StatusOK)
	})
	t.Run("invalid_credentials", func(t *testing.T) {
		checkProxy(t, user, url.UserPassword("alice", "wrong"), http.StatusProxyAuthRequired)
	})
	t.Run("without_auth", func(t *testing.T) {
		checkProxy(t, nil, nil, http.StatusOK)
	})
	t.Run("escaped_credentials", func(t *testing.T) {
		escaped := url.UserPassword("alice@example.com", "s3cr3t:@/?#%")
		checkProxy(t, escaped, escaped, http.StatusOK)
	})
}
