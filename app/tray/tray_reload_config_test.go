package tray

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/wzshiming/anyproxy/proxies/httpproxy"
	_ "github.com/wzshiming/anyproxy/proxies/socks4"
	_ "github.com/wzshiming/anyproxy/proxies/socks5"
	_ "github.com/wzshiming/anyproxy/proxies/sshproxy"
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/protocols/local"
	_ "github.com/wzshiming/bridge/protocols/socks5"
	"github.com/wzshiming/hostmatcher"
	"github.com/wzshiming/jumpway/app/web"
	"github.com/wzshiming/jumpway/app/web/services/configs"
	"github.com/wzshiming/jumpway/app/web/services/stats"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/metrics"
)

func TestReloadWebUIWithoutRules(test *testing.T) {
	app := newTestApp(test, &config.Config{})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	status := app.Status()
	if !status.Running || status.Address == "" || status.Error != "" || status.Rules == nil || len(status.Rules) != 0 {
		test.Fatalf("unexpected status: %+v", status)
	}
	transport := &http.Transport{}
	test.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second}
	response, err := client.Get("http://" + status.Address + "/apis/configs/status")
	if err != nil {
		test.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		test.Fatalf("status response: %s", response.Status)
	}
	var reported configs.Status
	if err := json.NewDecoder(response.Body).Decode(&reported); err != nil {
		test.Fatal(err)
	}
	if reported.Address != status.Address || !reported.Running || reported.Rules == nil || len(reported.Rules) != 0 {
		test.Fatalf("unexpected HTTP status: %+v", reported)
	}
}

func TestReloadAllRulesAndAuth(test *testing.T) {
	app := newTestApp(test, &config.Config{Rules: []config.Rule{
		{Name: "direct"},
		{Name: "auth", Listen: config.Listen{Username: "alice", Password: "test-secret"}},
		{Name: "disabled", Disabled: true},
	}})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	status := app.Status()
	if len(status.Rules) != 2 || status.Rules[0].Name != "direct" || status.Rules[1].Name != "auth" {
		test.Fatalf("enabled rules: %+v", status.Rules)
	}
	for _, rule := range status.Rules {
		if !rule.Running || rule.Remote || rule.Error != "" || rule.Attempt != 0 || rule.Address == status.Address {
			test.Fatalf("unexpected rule status: %+v", rule)
		}
	}
	if app.primaryAddress() != status.Rules[0].Address || app.webURL() != "http://"+status.Address {
		test.Fatal("menu helpers did not use their respective listeners")
	}
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, "target")
	}))
	test.Cleanup(target.Close)
	for _, scenario := range []struct {
		name    string
		address string
		user    *url.Userinfo
		code    int
	}{
		{name: "direct", address: status.Rules[0].Address, code: http.StatusOK},
		{name: "auth-required", address: status.Rules[1].Address, code: http.StatusProxyAuthRequired},
		{name: "auth-accepted", address: status.Rules[1].Address, user: url.UserPassword("alice", "test-secret"), code: http.StatusOK},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			proxyURL := &url.URL{Scheme: "http", Host: scenario.address, User: scenario.user}
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: time.Second}
			response, err := client.Get(target.URL)
			if err != nil {
				test.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != scenario.code {
				test.Fatalf("proxy status = %d, want %d", response.StatusCode, scenario.code)
			}
		})
	}
	listener := app.webListener
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	if app.webListener != listener || app.Status().Address != status.Address {
		test.Fatal("same configured Web UI address replaced the listener")
	}
	for _, rule := range status.Rules {
		assertListenerClosed(test, rule.Address)
	}
}

func TestReloadForwardRule(test *testing.T) {
	target := startEchoServer(test)
	app := newTestApp(test, &config.Config{Rules: []config.Rule{{
		Name:    "fwd",
		Listen:  config.Listen{Host: "127.0.0.1"},
		Forward: config.Forward{Host: "127.0.0.1", Port: uint32(target.Addr().(*net.TCPAddr).Port)},
	}}})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	status := app.Status()
	if len(status.Rules) != 1 || status.Rules[0].Name != "fwd" || !status.Rules[0].Running {
		test.Fatalf("unexpected forward status: %+v", status)
	}
	connection, err := net.DialTimeout("tcp", status.Rules[0].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		test.Fatal(err)
	}
	payload := "forward payload\n"
	if _, err := io.WriteString(connection, payload); err != nil {
		test.Fatal(err)
	}
	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(connection, reply); err != nil {
		test.Fatalf("read forwarded payload: %v", err)
	}
	if string(reply) != payload {
		test.Fatalf("forwarded payload = %q, want %q", reply, payload)
	}
	if status.Rules[0].Target != target.Addr().String() {
		test.Fatalf("forward target = %q, want %q", status.Rules[0].Target, target.Addr().String())
	}
}

func TestReloadForwardIgnoresNoProxy(test *testing.T) {
	previousNoProxy := chain.NoProxy
	chain.NoProxy = hostmatcher.NewMatcher([]string{"127.0.0.1"})
	test.Cleanup(func() { chain.NoProxy = previousNoProxy })
	target := startEchoServer(test)
	app := newTestApp(test, &config.Config{
		NoProxy: config.NoProxy{List: []string{"127.0.0.1"}},
		Rules: []config.Rule{{
			Name:   "fwd",
			Listen: config.Listen{Host: "127.0.0.1"},
			Forward: config.Forward{
				Host: "127.0.0.1",
				Port: uint32(target.Addr().(*net.TCPAddr).Port),
				Way:  []bridgeconfig.Node{{LB: []string{"socks5://127.0.0.1:1"}}},
			},
		}},
	})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	status := app.Status()
	if len(status.Rules) != 1 || !status.Rules[0].Running {
		test.Fatalf("unexpected forward status: %+v", status)
	}
	connection, err := net.DialTimeout("tcp", status.Rules[0].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		test.Fatal(err)
	}
	payload := "must not be echoed\n"
	if _, err := io.WriteString(connection, payload); err != nil {
		test.Fatal(err)
	}
	reply := make([]byte, len(payload))
	received, err := connection.Read(reply)
	if received != 0 || err == nil {
		test.Fatalf("no_proxy bypassed the forward chain: reply=%q, err=%v", reply[:received], err)
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		test.Fatalf("forward connection timed out instead of closing: %v", err)
	}
}

func TestReloadPreservesGenerationOnInvalidConfig(test *testing.T) {
	conf := &config.Config{Rules: []config.Rule{{Name: "direct"}}}
	app := newTestApp(test, conf)
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	before := app.Status()
	for _, contents := range []string{"web_ui: [", "rules:\n  - name: ''\n"} {
		if err := app.store.SaveRaw([]byte(contents)); err != nil {
			test.Fatal(err)
		}
		err := app.Reload()
		if err == nil {
			test.Fatal("invalid hand-edited config accepted")
		}
		after := app.Status()
		if !after.Running || after.Address != before.Address || after.Error != err.Error() || len(after.Rules) != 1 || after.Rules[0] != before.Rules[0] {
			test.Fatalf("invalid config replaced live generation: %+v", after)
		}
		if app.webErr != nil {
			test.Fatalf("load/validation error polluted webErr: %v", app.webErr)
		}
	}
	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil || app.Status().Error != "" {
		test.Fatalf("valid reload did not clear load error: %v, %+v", err, app.Status())
	}
}

func TestReloadBindFailuresRetryIndependently(test *testing.T) {
	occupied := occupyPort(test)
	port := uint32(occupied.Addr().(*net.TCPAddr).Port)
	app := newTestApp(test, &config.Config{Rules: []config.Rule{
		{Name: "occupied", Listen: config.Listen{Port: port}},
		{Name: "direct"},
	}})
	started := time.Now()
	err := app.Reload()
	if err == nil || !strings.Contains(err.Error(), `rule "occupied": `) {
		test.Fatalf("reload error = %v", err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		test.Fatalf("local failure took %s", elapsed)
	}
	status := app.Status()
	if !status.Running || status.Error != "" || status.Rules[0].Running || status.Rules[0].Attempt < 1 || status.Rules[0].Error == "" || !status.Rules[1].Running {
		test.Fatalf("bind failure status: %+v", status)
	}
	if app.primaryAddress() != status.Rules[1].Address {
		test.Fatal("failed first rule masked a running local rule")
	}
	waitForStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Attempt >= 2 })
	occupied.Close()
	status = waitForStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Running })
	if status.Rules[0].Attempt != 0 || status.Rules[0].Error != "" || app.primaryAddress() != occupied.Addr().String() {
		test.Fatalf("retry did not recover: %+v", status)
	}
}

func TestReloadReportsWebAndRuleErrors(test *testing.T) {
	webPort, rulePort := occupyPort(test), occupyPort(test)
	conf := &config.Config{
		WebUI: config.Address{Port: uint32(webPort.Addr().(*net.TCPAddr).Port)},
		Rules: []config.Rule{{Name: "occupied", Listen: config.Listen{Port: uint32(rulePort.Addr().(*net.TCPAddr).Port)}}, {Name: "direct"}},
	}
	app := newTestApp(test, conf)
	err := app.Reload()
	if err == nil || !strings.Contains(err.Error(), "web_ui: ") || !strings.Contains(err.Error(), `rule "occupied": `) {
		test.Fatalf("joined reload error = %v", err)
	}
	status := app.Status()
	if status.Running || status.Error == "" || !status.Rules[1].Running {
		test.Fatalf("web bind failure prevented rules: %+v", status)
	}
	if err := app.store.SaveRaw([]byte("rules: [")); err != nil {
		test.Fatal(err)
	}
	loadErr := app.Reload()
	if loadErr == nil || app.Status().Error != loadErr.Error() || app.webErr == nil {
		test.Fatal("load error did not take precedence over retained web listener error")
	}
	webPort.Close()
	rulePort.Close()
	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil || !app.Status().Running || app.Status().Error != "" {
		test.Fatalf("web listener did not recover: %v, %+v", err, app.Status())
	}
}

func TestReloadSharedDeadlineAndFirstEvent(test *testing.T) {
	previousChain := chain.Default
	chain.Default = chain.NewBridgeChain()
	test.Cleanup(func() { chain.Default = previousChain })
	bridger := &testListenBridger{listen: func(ctx context.Context, address string) (net.Listener, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	chain.Default.Register("slow", bridger)
	var dropped atomic.Int32
	chain.Default.Register("dropped", &testListenBridger{listen: func(ctx context.Context, address string) (net.Listener, error) {
		if dropped.Add(1) == 1 {
			return &droppedListener{}, nil
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}})
	occupied := occupyPort(test)
	app := newTestApp(test, &config.Config{Rules: []config.Rule{
		{Name: "slow-one", Listen: config.Listen{Way: []bridgeconfig.Node{{LB: []string{"slow://one"}}}}},
		{Name: "slow-two", Listen: config.Listen{Way: []bridgeconfig.Node{{LB: []string{"slow://two"}}}}},
		{Name: "occupied", Listen: config.Listen{Port: uint32(occupied.Addr().(*net.TCPAddr).Port)}},
		{Name: "dropped", Listen: config.Listen{Way: []bridgeconfig.Node{{LB: []string{"dropped://one"}}}}},
		{Name: "direct"},
	}})
	started := time.Now()
	err := app.Reload()
	elapsed := time.Since(started)
	if elapsed < 1900*time.Millisecond || elapsed > 2500*time.Millisecond {
		test.Fatalf("shared deadline took %s", elapsed)
	}
	if err == nil || !strings.HasPrefix(err.Error(), `rule "occupied": `) || strings.Contains(err.Error(), `rule "dropped"`) || strings.Contains(err.Error(), `rule "slow`) {
		test.Fatalf("first-attempt errors = %v", err)
	}
	status := app.Status()
	for _, rule := range status.Rules[:2] {
		if rule.Running || rule.Attempt != 0 || rule.Error != "" || !rule.Remote {
			test.Fatalf("unresolved rule: %+v", rule)
		}
	}
	if status.Rules[3].Running || status.Rules[3].Attempt != 1 || status.Rules[3].Error == "" || status.Rules[3].Address != "0.0.0.0:12345" || !status.Rules[4].Running {
		test.Fatalf("later events not reflected in status: %+v", status)
	}
	if err := app.store.Save(&config.Config{}); err != nil {
		test.Fatal(err)
	}
	started = time.Now()
	if err := app.Reload(); err != nil || time.Since(started) >= time.Second {
		test.Fatalf("canceling pending listens did not finish promptly: %v", err)
	}
}

func TestReloadReusesRemoteListenConfig(test *testing.T) {
	previousChain := chain.Default
	chain.Default = chain.NewBridgeChain()
	test.Cleanup(func() { chain.Default = previousChain })
	var attempts atomic.Int32
	bridger := &testListenBridger{listen: func(ctx context.Context, address string) (net.Listener, error) {
		if attempts.Add(1) == 1 {
			return nil, errors.New("temporary bind failure")
		}
		return local.LOCAL.Listen(ctx, "tcp", address)
	}}
	chain.Default.Register("retry", bridger)
	app := newTestApp(test, &config.Config{Rules: []config.Rule{
		{Name: "remote", Listen: config.Listen{Way: []bridgeconfig.Node{{LB: []string{"retry://remote"}}}}},
	}})
	if err := app.Reload(); err == nil || err.Error() != `rule "remote": temporary bind failure` {
		test.Fatalf("initial remote failure = %v", err)
	}
	status := waitForStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Running })
	if attempts.Load() != 2 || bridger.builds.Load() != 1 || !status.Rules[0].Remote || app.primaryAddress() != "" {
		test.Fatalf("remote listener was not reused: attempts=%d, builds=%d, status=%+v", attempts.Load(), bridger.builds.Load(), status)
	}
}

func TestReloadWebUIMovePreservesResponse(test *testing.T) {
	app := newTestApp(test, &config.Config{Rules: []config.Rule{{Name: "direct"}}})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	oldAddress := app.Status().Address
	available := occupyPort(test)
	newAddress := config.Address{Host: "127.0.0.1", Port: uint32(available.Addr().(*net.TCPAddr).Port)}
	available.Close()
	transport := &http.Transport{DisableKeepAlives: true}
	test.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	body, err := json.Marshal(newAddress)
	if err != nil {
		test.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPut, "http://"+oldAddress+"/apis/configs/web-ui", strings.NewReader(string(body)))
	if err != nil {
		test.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		test.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || app.Status().Address != newAddress.String() {
		test.Fatalf("move response/status: %s, %+v", response.Status, app.Status())
	}
	assertListenerClosed(test, oldAddress)
	response, err = client.Get(app.webURL() + "/apis/configs/status")
	if err != nil {
		test.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		test.Fatalf("new Web UI listener: %s", response.Status)
	}
}

func TestPrimaryAddressFallback(test *testing.T) {
	app := &App{rules: []*ruleState{
		{name: "remote", listenAddress: "0.0.0.0:10000", address: "0.0.0.0:10000", remote: true, running: true},
		{name: "forward", listenAddress: "0.0.0.0:10002", address: "127.0.0.1:20002", target: "127.0.0.1:5432", running: true},
		{name: "local", listenAddress: "0.0.0.0:10001", address: "127.0.0.1:20001"},
	}}
	if address := app.primaryAddress(); address != "127.0.0.1:10001" {
		test.Fatalf("fallback address = %q", address)
	}
	app.rules[2].running = true
	if address := app.primaryAddress(); address != "127.0.0.1:20001" {
		test.Fatalf("running proxy address = %q", address)
	}
	app.rules = app.rules[:2]
	if address := app.primaryAddress(); address != "" {
		test.Fatalf("remote or forward address used as local proxy: %q", address)
	}
	app.rules[1].running = false
	if address := app.primaryAddress(); address != "" {
		test.Fatalf("remote or forward address used as local fallback: %q", address)
	}
	app.updateStatus()
}

func newTestApp(test *testing.T, conf *config.Config) *App {
	test.Helper()
	store := config.NewStore(test.TempDir())
	if err := store.Save(conf); err != nil {
		test.Fatal(err)
	}
	app := &App{store: store, actions: make(chan func()), metrics: metrics.NewRegistry()}
	app.web = web.NewHandler(configs.NewConfigsService(store, app), stats.NewStatsService(app.metrics), http.NotFoundHandler())
	go func() {
		for action := range app.actions {
			action()
		}
	}()
	test.Cleanup(func() {
		app.Quit()
		app.wg.Wait()
		close(app.actions)
	})
	return app
}

func occupyPort(test *testing.T) net.Listener {
	test.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { listener.Close() })
	return listener
}

func startEchoServer(test *testing.T) net.Listener {
	test.Helper()
	listener := occupyPort(test)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		io.Copy(connection, connection)
	}()
	return listener
}

func assertListenerClosed(test *testing.T, address string) {
	test.Helper()
	connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
	if err == nil {
		connection.Close()
		test.Errorf("stale listener at %s", address)
	}
}

func waitForStatus(test *testing.T, app *App, ready func(configs.Status) bool) configs.Status {
	test.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		status := app.Status()
		if ready(status) {
			return status
		}
		select {
		case <-deadline.C:
			test.Fatalf("status did not converge: %+v", status)
		case <-ticker.C:
		}
	}
}

type testListenBridger struct {
	listen func(context.Context, string) (net.Listener, error)
	builds atomic.Int32
}

func (bridger *testListenBridger) Bridge(ctx context.Context, dialer bridge.Dialer, address string) (bridge.Dialer, error) {
	bridger.builds.Add(1)
	return &testListenDialer{Dialer: dialer, listen: bridger.listen}, nil
}

type testListenDialer struct {
	bridge.Dialer
	listen func(context.Context, string) (net.Listener, error)
}

func (dialer *testListenDialer) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return dialer.listen(ctx, address)
}

type droppedListener struct{}

func (*droppedListener) Accept() (net.Conn, error) {
	return nil, errors.New("listener dropped")
}

func (*droppedListener) Close() error {
	return nil
}

func (*droppedListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4zero, Port: 12345}
}
