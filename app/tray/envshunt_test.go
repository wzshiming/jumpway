package tray

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/bridge/chain"
	bridgeconfig "github.com/wzshiming/bridge/config"
	_ "github.com/wzshiming/bridge/protocols/connect"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/hostmatcher"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/metrics"
)

// envShuntElsewhere matches neither the hop nor the target.
const envShuntElsewhere = "192.0.2.1"

func TestEnvShuntDialsThroughBase(test *testing.T) {
	for _, scenario := range []struct {
		name, noProxy, onlyProxy string
		direct                   bool
	}{
		{name: "no_proxy-hit", noProxy: "127.0.0.1", direct: true},
		{name: "no_proxy-miss", noProxy: envShuntElsewhere},
		{name: "only_proxy-miss", onlyProxy: envShuntElsewhere, direct: true},
		{name: "no_proxy-empty"},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			hop, target := occupyPort(test), occupyPort(test)
			envShuntSetenv(test, scenario.noProxy, scenario.onlyProxy)
			dialed := envShuntDial(test, hop.Addr().String(), target.Addr().String())
			want := "tcp " + hop.Addr().String()
			if scenario.direct {
				want = "tcp " + target.Addr().String()
			}
			if !slices.Equal(dialed, []string{want}) {
				test.Fatalf("policy.base dials = %q, want only %q", dialed, want)
			}
			envShuntAssertIdle(test, hop)
			envShuntAssertIdle(test, target)
		})
	}
}

// envShuntSetenv sets NO_PROXY and ONLY_PROXY (unset when empty) in both spellings, and the chain matchers bridge derives from them once at init.
func envShuntSetenv(test *testing.T, noProxy, onlyProxy string) {
	test.Helper()
	for _, name := range []string{"no_proxy", "NO_PROXY"} {
		test.Setenv(name, noProxy)
	}
	for _, name := range []string{"only_proxy", "ONLY_PROXY"} {
		test.Setenv(name, onlyProxy)
		if onlyProxy == "" {
			os.Unsetenv(name)
		}
	}
	previousNoProxy, previousOnlyProxy := chain.NoProxy, chain.OnlyProxy
	test.Cleanup(func() { chain.NoProxy, chain.OnlyProxy = previousNoProxy, previousOnlyProxy })
	chain.NoProxy, chain.OnlyProxy = envShuntMatcher(noProxy), envShuntMatcher(onlyProxy)
}

func envShuntMatcher(list string) hostmatcher.Matcher {
	if list == "" {
		return nil
	}
	return hostmatcher.NewMatcher(strings.Split(list, ","))
}

// envShuntDial builds reload's dialer for a proxy rule with one http hop over a recording base, dials target with it, and returns the base's dials.
func envShuntDial(test *testing.T, hop, target string) []string {
	test.Helper()
	rule := config.Rule{Name: "envshunt", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"http://" + hop}}}}}
	registry := metrics.NewRegistry()
	registry.Sync([]config.Rule{rule}, "", nil)
	recorder := &envShuntBase{}
	base := *local.LOCAL
	base.Dialer = recorder
	dialer, err := (&App{}).buildDialer(context.Background(), rule, registry.Rule(rule.Name), exit{base: &base})
	if err != nil {
		test.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), ttWait)
	defer cancel()
	connection, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		test.Fatalf("dial %s through the rule: %v", target, err)
	}
	connection.Close()
	return recorder.recorded()
}

// envShuntBase records each dial and returns a pipe whose far end grants an HTTP CONNECT, so no dial leaves the process.
type envShuntBase struct {
	mu     sync.Mutex
	dialed []string
}

func (base *envShuntBase) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	base.mu.Lock()
	base.dialed = append(base.dialed, network+" "+address)
	base.mu.Unlock()
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		if request, err := http.ReadRequest(bufio.NewReader(server)); err == nil && request.Method == http.MethodConnect {
			io.WriteString(server, "HTTP/1.1 200 Connection established\r\n\r\n")
		}
	}()
	return client, nil
}

func (base *envShuntBase) recorded() []string {
	base.mu.Lock()
	defer base.mu.Unlock()
	return slices.Clone(base.dialed)
}

// envShuntAssertIdle fails when listener has a connection waiting, which only a dial around the base can leave.
func envShuntAssertIdle(test *testing.T, listener net.Listener) {
	test.Helper()
	if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		test.Fatal(err)
	}
	if connection, err := listener.Accept(); err == nil {
		connection.Close()
		test.Fatalf("%s accepted a connection, want every dial through policy.base", listener.Addr())
	}
}
