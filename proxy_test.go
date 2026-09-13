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

func TestRunProxyWebAndProxy(t *testing.T) {
	for _, name := range []string{"with_web", "without_web"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			var web http.Handler
			if name == "with_web" {
				web = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					if request.URL.Path != "/x" {
						t.Errorf("web path = %q, want /x", request.URL.Path)
					}
					io.WriteString(writer, "web-ok")
				})
			}
			done := make(chan error, 1)
			go func() { done <- RunProxy(ctx, listener, local.LOCAL, web) }()
			t.Cleanup(func() {
				cancel()
				listener.Close()
				select {
				case err := <-done:
					if err != nil && !utils.IsClosedConnError(err) {
						t.Errorf("RunProxy() = %v", err)
					}
				case <-time.After(5 * time.Second):
					t.Error("RunProxy did not stop after closing its listener")
				}
			})
			checkBody := func(client *http.Client, address, want string) {
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
			proxyURL := &url.URL{Scheme: "http", Host: listener.Addr().String()}
			if web != nil {
				transport := &http.Transport{DisableKeepAlives: true}
				t.Cleanup(transport.CloseIdleConnections)
				checkBody(&http.Client{Transport: transport, Timeout: 5 * time.Second}, proxyURL.String()+"/x", "web-ok")
			}
			target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/hello" {
					t.Errorf("target path = %q, want /hello", request.URL.Path)
				}
				io.WriteString(writer, "proxy-ok")
			}))
			t.Cleanup(target.Close)
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
			t.Cleanup(transport.CloseIdleConnections)
			checkBody(&http.Client{Transport: transport, Timeout: 5 * time.Second}, target.URL+"/hello", "proxy-ok")
		})
	}
}
