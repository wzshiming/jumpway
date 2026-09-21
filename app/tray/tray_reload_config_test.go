package tray

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/metrics"
	"github.com/wzshiming/shadowsocks"
	"github.com/wzshiming/socks4"
	"github.com/wzshiming/socks5"
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
		{Name: "auth", Listen: config.Listen{Username: "alice", Password: "test-secret", Cipher: "aes-256-gcm"}},
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
		ss      bool
		code    int
	}{
		{name: "direct", address: status.Rules[0].Address, code: http.StatusOK},
		{name: "auth-required", address: status.Rules[1].Address, code: http.StatusProxyAuthRequired},
		{name: "auth-accepted", address: status.Rules[1].Address, user: url.UserPassword("alice", "test-secret"), code: http.StatusOK},
		{name: "ss-accepted", address: status.Rules[1].Address, ss: true, code: http.StatusOK},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			proxyURL := &url.URL{Scheme: "http", Host: scenario.address, User: scenario.user}
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
			if scenario.ss {
				proxy, err := shadowsocks.NewDialer("ss://aes-256-gcm:test-secret@" + scenario.address)
				if err != nil {
					test.Fatal(err)
				}
				transport.Proxy, transport.DialContext = nil, proxy.DialContext
			}
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
	for index, rule := range app.Status().Rules {
		if !rule.Running || rule.Address != status.Rules[index].Address {
			test.Fatalf("unchanged rule listener replaced: before=%+v, after=%+v", status.Rules[index], rule)
		}
	}
}

func TestReloadCountsProxyTraffic(test *testing.T) {
	app := newTestApp(test, &config.Config{Rules: []config.Rule{{Name: "direct"}}})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, "target")
	}))
	test.Cleanup(target.Close)
	proxyURL := &url.URL{Scheme: "http", Host: app.Status().Rules[0].Address}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
	test.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second}
	response, err := client.Get(target.URL)
	if err != nil {
		test.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		test.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || string(body) != "target" {
		test.Fatalf("proxy response = %s, %q", response.Status, body)
	}
	snapshot := waitForSnapshot(test, app, func(snapshot metrics.Snapshot) bool {
		if len(snapshot.Rules) != 1 {
			return false
		}
		rule := snapshot.Rules[0]
		return rule.Stats.Total == 1 && rule.Stats.Up > 0 && rule.Stats.Down > 0 && rule.Stats.Active == 0 &&
			len(rule.Targets) == 1 && rule.Targets[0].Stats.Active == 0
	})
	rule := snapshot.Rules[0]
	if rule.Name != "direct" || rule.Stats.Dials != 1 || rule.Stats.DialFailures != 0 || rule.Stats.LatencyMs <= 0 {
		test.Fatalf("unexpected proxy statistics: %+v", rule)
	}
	entry := rule.Targets[0]
	if entry.Address != target.Listener.Addr().String() || entry.Via != "" || entry.Stats.Total != 1 {
		test.Fatalf("unexpected target statistics: %+v", entry)
	}
	if rule.Listen == nil || len(rule.Listen) != 0 || rule.Forward == nil || len(rule.Forward) != 0 {
		test.Fatalf("direct rule has nonempty or null ways: %+v", rule)
	}
	waitForSnapshot(test, app, func(snapshot metrics.Snapshot) bool {
		rule := snapshot.Rules[0]
		return rule.Stats.RateUp > 0 && rule.Stats.RateDown > 0 &&
			rule.Targets[0].Stats.RateUp > 0 && rule.Targets[0].Stats.RateDown > 0
	})
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

func TestReloadKeepsUnchangedRuleConnections(test *testing.T) {
	target := startEchoServer(test)
	conf := &config.Config{Rules: []config.Rule{
		{
			Name:    "a",
			Listen:  config.Listen{Host: "127.0.0.1"},
			Forward: config.Forward{Host: "127.0.0.1", Port: uint32(target.Addr().(*net.TCPAddr).Port)},
		},
		{Name: "b"},
	}}
	app := newTestApp(test, conf)
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	before := app.Status()
	connection, err := net.DialTimeout("tcp", before.Rules[0].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	reader := bufio.NewReader(connection)
	assertTunnelEcho(test, connection, reader, "before reload\n")
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	assertTunnelEcho(test, connection, reader, "after identical reload\n")
	if address := app.Status().Rules[0].Address; address != before.Rules[0].Address {
		test.Fatalf("identical reload changed a's address: got %q, want %q", address, before.Rules[0].Address)
	}
	available := occupyPort(test)
	conf.Rules[1].Listen.Port = uint32(available.Addr().(*net.TCPAddr).Port)
	if err := available.Close(); err != nil {
		test.Fatal(err)
	}
	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	assertTunnelEcho(test, connection, reader, "after changing b\n")
	after := app.Status()
	if len(after.Rules) != 2 || after.Rules[0].Name != "a" || after.Rules[1].Name != "b" {
		test.Fatalf("rule order after reload: %+v", after.Rules)
	}
	if !after.Rules[0].Running || after.Rules[0].Address != before.Rules[0].Address {
		test.Fatalf("changing b replaced a's listener: before=%+v, after=%+v", before.Rules[0], after.Rules[0])
	}
	assertListenerClosed(test, before.Rules[1].Address)
	if !after.Rules[1].Running || after.Rules[1].Address != available.Addr().String() {
		test.Fatalf("changed rule listener: %+v", after.Rules[1])
	}
	proxy, err := net.DialTimeout("tcp", after.Rules[1].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer proxy.Close()
	if snapshot := app.metrics.Snapshot(); len(snapshot.Rules[0].Connections) != 1 {
		test.Fatalf("unchanged rule lost its live connection: %+v", snapshot.Rules[0])
	}
}

func TestReloadRestartsChangedRule(test *testing.T) {
	firstTarget, secondTarget := startEchoServer(test), startEchoServer(test)
	available := occupyPort(test)
	conf := &config.Config{Rules: []config.Rule{{
		Name:    "a",
		Listen:  config.Listen{Host: "127.0.0.1", Port: uint32(available.Addr().(*net.TCPAddr).Port)},
		Forward: config.Forward{Host: "127.0.0.1", Port: uint32(firstTarget.Addr().(*net.TCPAddr).Port)},
	}}}
	if err := available.Close(); err != nil {
		test.Fatal(err)
	}
	app := newTestApp(test, conf)
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	connection, err := net.DialTimeout("tcp", app.Status().Rules[0].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	assertTunnelEcho(test, connection, connection, "first target\n")
	conf.Rules[0].Forward.Port = uint32(secondTarget.Addr().(*net.TCPAddr).Port)
	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	assertTunnelClosed(test, connection)
	status := app.Status()
	if status.Rules[0].Target != secondTarget.Addr().String() || status.Rules[0].Address != available.Addr().String() {
		test.Fatalf("changed forward target/listener: %+v", status.Rules[0])
	}
	replacement, err := net.DialTimeout("tcp", status.Rules[0].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer replacement.Close()
	assertTunnelEcho(test, replacement, replacement, "second target\n")
}

func TestReloadNoProxyChangeRestartsProxyRulesOnly(test *testing.T) {
	target := startMultiEchoServer(test)
	conf := &config.Config{Rules: []config.Rule{
		{
			Name:    "a",
			Listen:  config.Listen{Host: "127.0.0.1"},
			Forward: config.Forward{Host: "127.0.0.1", Port: uint32(target.Addr().(*net.TCPAddr).Port)},
		},
		{Name: "p"},
	}}
	app := newTestApp(test, conf)
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	before := app.Status()
	forward, err := net.DialTimeout("tcp", before.Rules[0].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer forward.Close()
	assertTunnelEcho(test, forward, forward, "forward before no_proxy change\n")
	proxy, err := net.DialTimeout("tcp", before.Rules[1].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer proxy.Close()
	if err := proxy.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		test.Fatal(err)
	}
	address := target.Addr().String()
	if _, err := io.WriteString(proxy, "CONNECT "+address+" HTTP/1.1\r\nHost: "+address+"\r\n\r\n"); err != nil {
		test.Fatal(err)
	}
	reader := bufio.NewReader(proxy)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		test.Fatal(err)
	}
	defer response.Body.Close()
	if response.Proto != "HTTP/1.1" || response.StatusCode != http.StatusOK {
		test.Fatalf("CONNECT response = %s %s, want HTTP/1.1 200", response.Proto, response.Status)
	}
	assertTunnelEcho(test, proxy, reader, "proxy before no_proxy change\n")
	conf.NoProxy.List = []string{"example.invalid"}
	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	assertTunnelClosed(test, proxy)
	assertTunnelEcho(test, forward, forward, "forward after no_proxy change\n")
	if address := app.Status().Rules[0].Address; address != before.Rules[0].Address {
		test.Fatalf("no_proxy changed forward address: got %q, want %q", address, before.Rules[0].Address)
	}
}

func TestReloadRestartsNotRunningRule(test *testing.T) {
	occupied := occupyPort(test)
	app := newTestApp(test, &config.Config{Rules: []config.Rule{
		{Name: "occupied", Listen: config.Listen{Port: uint32(occupied.Addr().(*net.TCPAddr).Port)}},
		{Name: "direct"},
	}})
	if err := app.Reload(); err == nil || !strings.Contains(err.Error(), `rule "occupied": `) {
		test.Fatalf("initial reload error = %v", err)
	}
	if status := app.Status(); status.Rules[0].Running || !status.Rules[1].Running {
		test.Fatalf("initial bind failure status: %+v", status)
	}
	if err := occupied.Close(); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	if status := app.Status(); !status.Rules[0].Running || status.Rules[0].Error != "" {
		test.Fatalf("reload did not restart failed rule: %+v", status)
	}
}

func TestReloadCountsForwardTraffic(test *testing.T) {
	target := startEchoServer(test)
	app := newTestApp(test, &config.Config{Rules: []config.Rule{{
		Name:    "fwd",
		Listen:  config.Listen{Host: "127.0.0.1"},
		Forward: config.Forward{Host: "127.0.0.1", Port: uint32(target.Addr().(*net.TCPAddr).Port)},
	}}})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	connection, err := net.DialTimeout("tcp", app.Status().Rules[0].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		test.Fatal(err)
	}
	payload := "counted forward payload\n"
	if _, err := io.WriteString(connection, payload); err != nil {
		test.Fatal(err)
	}
	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(connection, reply); err != nil {
		test.Fatal(err)
	}
	if string(reply) != payload {
		test.Fatalf("forwarded payload = %q, want %q", reply, payload)
	}
	snapshot := waitForSnapshot(test, app, func(snapshot metrics.Snapshot) bool {
		if len(snapshot.Rules) != 1 || len(snapshot.Rules[0].Targets) != 1 {
			return false
		}
		entry := snapshot.Rules[0].Targets[0]
		return entry.Stats.Up == int64(len(payload)) && entry.Stats.Down == int64(len(payload))
	})
	rule := snapshot.Rules[0]
	entry := rule.Targets[0]
	if rule.Stats.Total != 1 || rule.Stats.Active != 1 || entry.Address != target.Addr().String() ||
		entry.Via != "" || entry.Stats.Total != 1 || entry.Stats.Active != 1 {
		test.Fatalf("unexpected forward statistics: %+v", rule)
	}
	if err := connection.Close(); err != nil {
		test.Fatal(err)
	}
	waitForSnapshot(test, app, func(snapshot metrics.Snapshot) bool {
		rule := snapshot.Rules[0]
		return rule.Stats.Active == 0 && rule.Targets[0].Stats.Active == 0
	})
}

func TestReloadDisconnectConnection(test *testing.T) {
	for _, scenario := range []struct {
		name  string
		proxy bool
		http  bool
	}{
		{name: "forward"},
		{name: "proxy", proxy: true},
		{name: "http", http: true},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			target := startEchoServer(test)
			rule := config.Rule{Name: "rule", Listen: config.Listen{Host: "127.0.0.1"}}
			if !scenario.proxy {
				rule.Forward = config.Forward{Host: "127.0.0.1", Port: uint32(target.Addr().(*net.TCPAddr).Port)}
			}
			app := newTestApp(test, &config.Config{WebUI: config.Address{Host: "127.0.0.1"}, Rules: []config.Rule{rule}})
			if err := app.Reload(); err != nil {
				test.Fatal(err)
			}
			connection, err := net.DialTimeout("tcp", app.Status().Rules[0].Address, time.Second)
			if err != nil {
				test.Fatal(err)
			}
			defer connection.Close()
			if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
				test.Fatal(err)
			}
			reader := bufio.NewReader(connection)
			client := connection.LocalAddr().String()
			if scenario.proxy {
				request, err := http.NewRequest(http.MethodConnect, "http://"+target.Addr().String(), nil)
				if err != nil {
					test.Fatal(err)
				}
				if err := request.Write(connection); err != nil {
					test.Fatal(err)
				}
				response, err := http.ReadResponse(reader, request)
				if err != nil {
					test.Fatal(err)
				}
				defer response.Body.Close()
				if response.StatusCode != http.StatusOK {
					test.Fatalf("CONNECT response = %s, want 200", response.Status)
				}
			}
			payload := "live connection payload"
			if _, err := io.WriteString(connection, payload); err != nil {
				test.Fatal(err)
			}
			reply := make([]byte, len(payload))
			if _, err := io.ReadFull(reader, reply); err != nil {
				test.Fatal(err)
			}
			if string(reply) != payload {
				test.Fatalf("echo = %q, want %q", reply, payload)
			}
			snapshot := waitForSnapshot(test, app, func(snapshot metrics.Snapshot) bool {
				if len(snapshot.Rules) != 1 || len(snapshot.Rules[0].Connections) != 1 {
					return false
				}
				live := snapshot.Rules[0].Connections[0]
				return live.Stats.Up == int64(len(payload)) && live.Stats.Down == int64(len(payload))
			})
			live := snapshot.Rules[0].Connections[0]
			if live.ID == 0 || live.Client != client || live.Target != target.Addr().String() || live.Via != "" {
				test.Fatalf("connection = %+v, want client %q and target %q", live, client, target.Addr().String())
			}
			if scenario.http {
				transport := &http.Transport{}
				test.Cleanup(transport.CloseIdleConnections)
				client := &http.Client{Transport: transport, Timeout: time.Second}
				request, err := http.NewRequest(http.MethodDelete, app.webURL()+"/apis/stats/connections/"+strconv.FormatUint(live.ID, 10), nil)
				if err != nil {
					test.Fatal(err)
				}
				response, err := client.Do(request)
				if err != nil {
					test.Fatal(err)
				}
				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil {
					test.Fatal(err)
				}
				if response.StatusCode != http.StatusOK {
					test.Fatalf("DELETE response = %s: %s", response.Status, body)
				}
			} else if err := app.metrics.Disconnect(live.ID); err != nil {
				test.Fatal(err)
			}
			if size, err := reader.Read(make([]byte, 1)); size != 0 || err == nil {
				test.Fatalf("Read after Disconnect = (%d, %v), want EOF or error", size, err)
			} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				test.Fatalf("Disconnect did not close the client: %v", err)
			}
			waitForSnapshot(test, app, func(snapshot metrics.Snapshot) bool {
				rule := snapshot.Rules[0]
				return rule.Connections != nil && len(rule.Connections) == 0 && rule.Stats.Active == 0 &&
					len(rule.Targets) == 1 && rule.Targets[0].Stats.Active == 0
			})
		})
	}
}

func TestReloadCountsNoProxyAsDirect(test *testing.T) {
	previousNoProxy := chain.NoProxy
	chain.NoProxy = hostmatcher.NewMatcher([]string{"example.invalid"})
	test.Cleanup(func() { chain.NoProxy = previousNoProxy })
	app := newTestApp(test, &config.Config{
		NoProxy: config.NoProxy{List: []string{"127.0.0.1"}},
		Rules: []config.Rule{{
			Name:    "direct",
			Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"socks5://127.0.0.1:1"}}}},
		}},
	})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, "target")
	}))
	test.Cleanup(target.Close)
	proxyURL := &url.URL{Scheme: "http", Host: app.Status().Rules[0].Address}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
	test.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second}
	response, err := client.Get(target.URL)
	if err != nil {
		test.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		test.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || string(body) != "target" {
		test.Fatalf("no_proxy response = %s, %q", response.Status, body)
	}
	snapshot := waitForSnapshot(test, app, func(snapshot metrics.Snapshot) bool {
		if len(snapshot.Rules) != 1 || len(snapshot.Rules[0].Targets) != 1 {
			return false
		}
		rule := snapshot.Rules[0]
		return rule.Stats.Total == 1 && rule.Stats.Active == 0 && rule.Targets[0].Stats.Active == 0
	})
	rule := snapshot.Rules[0]
	entry := rule.Targets[0]
	if rule.Stats.Dials != 1 || rule.Stats.DialFailures != 0 || entry.Address != target.Listener.Addr().String() ||
		entry.Via != "" || entry.Stats.Total != 1 {
		test.Fatalf("unexpected no_proxy statistics: %+v", rule)
	}
	if len(rule.Forward) != 1 || rule.Forward[0].Index != 0 || rule.Forward[0].ParentIndex != -1 || len(rule.Forward[0].URLs) != 1 {
		test.Fatalf("unexpected forward hops: %+v", rule.Forward)
	}
	hop := rule.Forward[0].URLs[0]
	if hop.URL != "socks5://127.0.0.1:1" || hop.Stats.Dials != 0 {
		test.Fatalf("no_proxy did not bypass the forward chain: %+v", hop)
	}
}

func TestReloadForwardHalfClose(test *testing.T) {
	target := occupyPort(test)
	done := make(chan error, 1)
	go func() {
		connection, err := target.Accept()
		if err != nil {
			done <- err
			return
		}
		defer connection.Close()
		if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			done <- err
			return
		}
		request, err := io.ReadAll(connection)
		if err != nil {
			done <- err
			return
		}
		_, err = io.WriteString(connection, "reply: "+string(request))
		done <- err
	}()
	app := newTestApp(test, &config.Config{Rules: []config.Rule{{
		Name:    "fwd",
		Listen:  config.Listen{Host: "127.0.0.1"},
		Forward: config.Forward{Host: "127.0.0.1", Port: uint32(target.Addr().(*net.TCPAddr).Port)},
	}}})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	connection, err := net.DialTimeout("tcp", app.Status().Rules[0].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		test.Fatal(err)
	}
	payload := strings.Repeat("half-close", 32*1024)
	if _, err := io.WriteString(connection, payload); err != nil {
		test.Fatal(err)
	}
	if err := connection.(*net.TCPConn).CloseWrite(); err != nil {
		test.Fatal(err)
	}
	reply, err := io.ReadAll(connection)
	if err != nil {
		test.Fatal(err)
	}
	if string(reply) != "reply: "+payload {
		test.Fatalf("half-close response = %d bytes, want %d bytes", len(reply), len("reply: ")+len(payload))
	}
	if err := <-done; err != nil {
		test.Fatal(err)
	}
	waitForSnapshot(test, app, func(snapshot metrics.Snapshot) bool {
		if len(snapshot.Rules) != 1 || len(snapshot.Rules[0].Targets) != 1 {
			return false
		}
		rule := snapshot.Rules[0]
		return rule.Stats.Total == 1 && rule.Stats.Up == int64(len(payload)) && rule.Stats.Down == int64(len(reply)) &&
			rule.Stats.Active == 0 && rule.Targets[0].Stats.Active == 0
	})
}

func TestReloadRetainsStatsAcrossReload(test *testing.T) {
	conf := &config.Config{Rules: []config.Rule{{Name: "a"}, {Name: "b"}}}
	app := newTestApp(test, conf)
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, "target")
	}))
	test.Cleanup(target.Close)
	for _, rule := range app.Status().Rules {
		proxyURL := &url.URL{Scheme: "http", Host: rule.Address}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
		test.Cleanup(transport.CloseIdleConnections)
		client := &http.Client{Transport: transport, Timeout: time.Second}
		response, err := client.Get(target.URL)
		if err != nil {
			test.Fatal(err)
		}
		_, err = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if err != nil {
			test.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			test.Fatalf("rule %q response = %s", rule.Name, response.Status)
		}
	}
	before := waitForSnapshot(test, app, func(snapshot metrics.Snapshot) bool {
		if len(snapshot.Rules) != 2 {
			return false
		}
		for _, rule := range snapshot.Rules {
			if rule.Stats.Total != 1 || rule.Stats.Active != 0 || len(rule.Targets) != 1 || rule.Targets[0].Stats.Active != 0 {
				return false
			}
		}
		return true
	})
	conf.Rules[0].Name = "c"
	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	for _, name := range []string{"renamed", "unchanged"} {
		test.Run(name, func(test *testing.T) {
			if err := app.Reload(); err != nil {
				test.Fatal(err)
			}
			snapshot := app.metrics.Snapshot()
			if len(snapshot.Rules) != 2 || snapshot.Rules[0].Name != "c" || snapshot.Rules[1].Name != "b" {
				test.Fatalf("unexpected rules after reload: %+v", snapshot.Rules)
			}
			renamed := snapshot.Rules[0]
			if renamed.Stats.Total != 0 || renamed.Stats.Up != 0 || renamed.Stats.Down != 0 || renamed.Stats.Dials != 0 || len(renamed.Targets) != 0 {
				test.Fatalf("renamed rule retained statistics: %+v", renamed)
			}
			retained, previous := snapshot.Rules[1], before.Rules[1]
			if retained.Stats.Total != previous.Stats.Total || retained.Stats.Up != previous.Stats.Up ||
				retained.Stats.Down != previous.Stats.Down || retained.Stats.Dials != previous.Stats.Dials ||
				len(retained.Targets) != 1 || retained.Targets[0].Stats.Total != previous.Targets[0].Stats.Total {
				test.Fatalf("unchanged rule lost statistics: before=%+v, after=%+v", previous, retained)
			}
		})
	}
}

func TestMetricsEndpoint(test *testing.T) {
	app := newTestApp(test, &config.Config{Rules: []config.Rule{{Name: "direct"}}})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	transport := &http.Transport{DisableKeepAlives: true}
	test.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second}
	response, err := client.Get("http://" + app.Status().Address + "/metrics")
	if err != nil {
		test.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		test.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `jumpway_rule_connections_total{rule="direct"}`) ||
		!strings.Contains(string(body), "go_goroutines") {
		test.Fatalf("metrics response = %s, %s", response.Status, body)
	}
	response, err = client.Get("http://" + app.Status().Address + "/apis/stats")
	if err != nil {
		test.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		test.Fatalf("statistics response = %s", response.Status)
	}
	var snapshot metrics.Snapshot
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		test.Fatal(err)
	}
	if len(snapshot.Rules) != 1 || snapshot.Rules[0].Name != "direct" {
		test.Fatalf("unexpected HTTP statistics: %+v", snapshot)
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

func TestReloadAlertsOnceOnBindFailure(test *testing.T) {
	if os.Getenv("JUMPWAY_TEST_OCCUPIED_EPISODE") != "" {
		occupied := occupyPort(test)
		app := newTestApp(test, &config.Config{Rules: []config.Rule{
			{Name: "occupied", Listen: config.Listen{Port: uint32(occupied.Addr().(*net.TCPAddr).Port)}},
		}})
		if err := app.Reload(); err == nil {
			test.Fatal("reload bound an occupied port")
		}
		waitForStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Attempt >= 3 })
		occupied.Close()
		waitForStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Running })
		return
	}
	// The logger writes to the process's stderr, so the episode runs in a child test binary.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+test.Name()+"$", "-test.count=1", "-test.timeout=20s")
	command.WaitDelay = time.Second
	command.Env = append(os.Environ(), "JUMPWAY_TEST_OCCUPIED_EPISODE=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		test.Fatalf("episode child: %v\n%s%s", err, stdout.String(), stderr.String())
	}
	var errorLines, warnLines, infoLines []string
	for _, line := range strings.Split(stderr.String(), "\n") {
		if !strings.Contains(line, " rule=occupied") {
			continue
		}
		switch {
		case strings.Contains(line, " level=ERROR "):
			errorLines = append(errorLines, line)
		case strings.Contains(line, " level=WARN "):
			warnLines = append(warnLines, line)
		case strings.Contains(line, " level=INFO "):
			infoLines = append(infoLines, line)
		}
	}
	if len(errorLines) != 1 || len(warnLines) < 2 || len(infoLines) != 1 {
		test.Fatalf("occupied rule logged %d ERROR, %d WARN and %d INFO lines, want 1 ERROR, >= 2 WARN and 1 INFO:\n%s", len(errorLines), len(warnLines), len(infoLines), stderr.String())
	}
	first := errorLines[0]
	start, end := strings.Index(first, " msg="), strings.Index(first, " err=")
	if start < 0 || end < start || !strings.Contains(first, " attempt=1 ") {
		test.Fatalf("first failure = %q, want msg, err and attempt=1", first)
	}
	msg := first[start:end]
	for _, line := range warnLines {
		if !strings.Contains(line, msg) || !strings.Contains(line, " err=") || !strings.Contains(line, " attempt=") ||
			strings.Contains(line, " attempt=1 ") || !strings.Contains(line, " backoff=") {
			test.Fatalf("retry = %q, want%s with err, attempt >= 2 and backoff", line, msg)
		}
	}
	if strings.Contains(infoLines[0], " attempt=") {
		test.Fatalf("recovery = %q, want a bound address", infoLines[0])
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

func TestStopCancelsMetrics(test *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := &App{metrics: metrics.NewRegistry(), metricsCancel: cancel}
	done := make(chan struct{})
	go func() {
		app.metrics.Run(ctx)
		close(done)
	}()
	app.stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		test.Fatal("metrics ticker did not stop")
	}
}

func TestPrimaryAddressFallback(test *testing.T) {
	app := &App{rules: []*ruleState{
		{name: "remote", listenAddress: "0.0.0.0:10000", address: "0.0.0.0:10000", remote: true, http: true, running: true},
		{name: "forward", listenAddress: "0.0.0.0:10002", address: "127.0.0.1:20002", target: "127.0.0.1:5432", running: true},
		{name: "virtual", listenAddress: "virtual://x", address: "virtual://x", virtual: true, http: true, running: true},
		{name: "socks", listenAddress: "0.0.0.0:10003", address: "127.0.0.1:20003", running: true},
		{name: "local", listenAddress: "0.0.0.0:10001", address: "127.0.0.1:20001", http: true},
	}}
	if address := app.primaryAddress(); address != "127.0.0.1:10001" {
		test.Fatalf("fallback address = %q", address)
	}
	app.rules[4].running = true
	if address := app.primaryAddress(); address != "127.0.0.1:20001" {
		test.Fatalf("running proxy address = %q", address)
	}
	app.rules = app.rules[:4]
	if address := app.primaryAddress(); address != "" {
		test.Fatalf("remote, forward or non-HTTP address used as local proxy: %q", address)
	}
	app.rules[1].running = false
	if address := app.primaryAddress(); address != "" {
		test.Fatalf("remote or forward address used as local fallback: %q", address)
	}
	app.updateStatus()
}

func TestReloadVirtualEndpoints(test *testing.T) {
	target := startMultiEchoServer(test)
	conf := &config.Config{Rules: []config.Rule{
		{Name: "entry", Listen: config.Listen{Host: "127.0.0.1"}, Forward: config.Forward{Virtual: "x"}},
		{Name: "exit", Listen: config.Listen{Virtual: "x"}, Forward: config.Forward{Host: "127.0.0.1", Port: uint32(target.Addr().(*net.TCPAddr).Port)}},
	}}
	app := newTestApp(test, conf)
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	status := app.Status()
	if len(status.Rules) != 2 || !status.Rules[0].Running || !status.Rules[1].Running || status.Rules[1].Remote {
		test.Fatalf("unexpected status: %+v", status)
	}
	if status.Rules[0].Target != "virtual://x" || status.Rules[1].Address != "virtual://x" || status.Rules[1].Target != target.Addr().String() {
		test.Fatalf("virtual endpoints not displayed: %+v", status.Rules)
	}
	connection, err := net.DialTimeout("tcp", status.Rules[0].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	reader := bufio.NewReader(connection)
	payload := "through virtual\n"
	assertTunnelEcho(test, connection, reader, payload)
	client := connection.LocalAddr().String()
	snapshot := waitForSnapshot(test, app, func(snapshot metrics.Snapshot) bool {
		if len(snapshot.Rules) != 2 || len(snapshot.Rules[0].Connections) != 1 || len(snapshot.Rules[1].Connections) != 1 {
			return false
		}
		return snapshot.Rules[0].Connections[0].Stats.Down == int64(len(payload)) && snapshot.Rules[1].Connections[0].Stats.Down == int64(len(payload))
	})
	entry, exit := snapshot.Rules[0].Connections[0], snapshot.Rules[1].Connections[0]
	if entry.Client != client || entry.Target != "virtual://x" || exit.Client != "virtual://x" || exit.Target != target.Addr().String() {
		test.Fatalf("entry = %+v, exit = %+v, want client %q and target %q", entry, exit, client, target.Addr().String())
	}
	if len(snapshot.Rules[0].Targets) != 1 || snapshot.Rules[0].Targets[0].Address != "virtual://x" || snapshot.Rules[0].Targets[0].Stats.Total != 1 {
		test.Fatalf("entry targets = %+v", snapshot.Rules[0].Targets)
	}
	conf.Rules[1].Disabled = true
	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	if status := app.Status(); len(status.Rules) != 1 || status.Rules[0].Name != "entry" || !status.Rules[0].Running {
		test.Fatalf("status after disabling exit: %+v", status)
	}
	if size, err := reader.Read(make([]byte, 1)); size != 0 || err == nil {
		test.Fatalf("Read after disabling exit = (%d, %v), want EOF or error", size, err)
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		test.Fatalf("disabling exit did not release the connection: %v", err)
	}
	conf.Rules[1].Disabled = false
	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	if status := app.Status(); len(status.Rules) != 2 || !status.Rules[1].Running || status.Rules[1].Address != "virtual://x" {
		test.Fatalf("status after re-enabling exit: %+v", status)
	}
	again, err := net.DialTimeout("tcp", status.Rules[0].Address, time.Second)
	if err != nil {
		test.Fatal(err)
	}
	defer again.Close()
	assertTunnelEcho(test, again, again, "after re-enable\n")
}

func TestReloadVirtualProxyExit(test *testing.T) {
	target := startMultiEchoServer(test)
	app := newTestApp(test, &config.Config{Rules: []config.Rule{
		{Name: "entry", Listen: config.Listen{Host: "127.0.0.1"}, Forward: config.Forward{Virtual: "x"}},
		{Name: "exit", Listen: config.Listen{Virtual: "x", Username: "alice", Password: "test-secret"}},
	}})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	status := app.Status()
	if len(status.Rules) != 2 || !status.Rules[1].Running || status.Rules[1].Address != "virtual://x" || status.Rules[1].Target != "" {
		test.Fatalf("unexpected status: %+v", status)
	}
	if address := app.primaryAddress(); address != "" {
		test.Fatalf("virtual proxy listener offered as system proxy: %q", address)
	}
	if address := app.ruleAddress("exit"); address != "" {
		test.Fatalf("virtual proxy listener exported: %q", address)
	}
	if entries := menuModel([]ruleState{*app.rules[1]}); entries[0].localProxy {
		test.Fatalf("virtual proxy listener listed as local proxy: %+v", entries[0])
	}
	app.mu.Lock()
	app.systemProxyRule = "exit"
	app.mu.Unlock()
	if address, _, removed := app.syncSystemProxySelection(); address != "" || removed != "exit" {
		test.Fatalf("virtual proxy listener kept as system proxy: %q, removed %q", address, removed)
	}
	for _, scenario := range []struct {
		name        string
		credentials string
		code        int
	}{
		{name: "auth-required", code: http.StatusProxyAuthRequired},
		{name: "auth-accepted", credentials: "alice:test-secret", code: http.StatusOK},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			connection, err := net.DialTimeout("tcp", status.Rules[0].Address, time.Second)
			if err != nil {
				test.Fatal(err)
			}
			defer connection.Close()
			if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
				test.Fatal(err)
			}
			request, err := http.NewRequest(http.MethodConnect, "http://"+target.Addr().String(), nil)
			if err != nil {
				test.Fatal(err)
			}
			if scenario.credentials != "" {
				request.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(scenario.credentials)))
			}
			if err := request.Write(connection); err != nil {
				test.Fatal(err)
			}
			reader := bufio.NewReader(connection)
			response, err := http.ReadResponse(reader, request)
			if err != nil {
				test.Fatal(err)
			}
			if response.StatusCode != scenario.code {
				test.Fatalf("CONNECT response = %s, want %d", response.Status, scenario.code)
			}
			if scenario.code != http.StatusOK {
				return
			}
			payload := "proxied through virtual\n"
			assertTunnelEcho(test, connection, reader, payload)
			snapshot := waitForSnapshot(test, app, func(snapshot metrics.Snapshot) bool {
				return len(snapshot.Rules) == 2 && len(snapshot.Rules[1].Connections) == 1 && snapshot.Rules[1].Connections[0].Stats.Down == int64(len(payload))
			})
			live := snapshot.Rules[1].Connections[0]
			if live.Client != "virtual://x" || live.Target != target.Addr().String() || live.Via != "" {
				test.Fatalf("exit connection = %+v, want client virtual://x and target %q", live, target.Addr().String())
			}
		})
	}
}

func TestReloadProtocolSelection(test *testing.T) {
	var mu sync.Mutex
	var calls []string
	previous := setSystemProxy
	setSystemProxy = func(address string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, address)
	}
	test.Cleanup(func() { setSystemProxy = previous })
	conf := &config.Config{Rules: []config.Rule{
		{Name: "multi", Listen: config.Listen{Host: "127.0.0.1", Password: "shared", Protocols: []config.Protocol{
			{Type: "http", Username: "alice"},
			{Type: "socks5", Username: "bob", Password: "bob-secret"},
		}}},
		{Name: "socks", Listen: config.Listen{Host: "127.0.0.1", Protocols: []config.Protocol{{Type: "socks5"}}}},
	}}
	app := newTestApp(test, conf)
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	status := app.Status()
	if len(status.Rules) != 2 || !status.Rules[0].Running || !status.Rules[1].Running {
		test.Fatalf("unexpected status: %+v", status)
	}
	multi, socks := status.Rules[0].Address, status.Rules[1].Address
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, "target")
	}))
	test.Cleanup(target.Close)
	alice, bob := url.UserPassword("alice", "shared"), url.UserPassword("bob", "bob-secret")
	for _, scenario := range []struct {
		name  string
		proxy *url.URL
		code  int
	}{
		{name: "http_alice", proxy: &url.URL{Scheme: "http", Host: multi, User: alice}, code: http.StatusOK},
		{name: "http_bob", proxy: &url.URL{Scheme: "http", Host: multi, User: bob}, code: http.StatusProxyAuthRequired},
		{name: "http_anonymous", proxy: &url.URL{Scheme: "http", Host: multi}, code: http.StatusProxyAuthRequired},
		{name: "socks5_bob", proxy: &url.URL{Scheme: "socks5", Host: multi, User: bob}, code: http.StatusOK},
		{name: "socks5_alice", proxy: &url.URL{Scheme: "socks5", Host: multi, User: alice}},
		{name: "socks4_not_selected", proxy: &url.URL{Scheme: "socks4", Host: multi, User: url.User("alice")}},
		{name: "socks_rule_rejects_http", proxy: &url.URL{Scheme: "http", Host: socks}},
		{name: "socks_rule_socks5", proxy: &url.URL{Scheme: "socks5", Host: socks}, code: http.StatusOK},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			if code, err := requestThrough(test, scenario.proxy, target.URL); code != scenario.code || (err == nil) != (scenario.code != 0) {
				test.Fatalf("status = %d, err = %v; want status %d", code, err, scenario.code)
			}
		})
	}
	if address := app.primaryAddress(); address != multi {
		test.Fatalf("primary address = %q, want the HTTP-serving rule %q", address, multi)
	}
	if app.ruleAddress("multi") != multi || app.ruleAddress("socks") != "" {
		test.Fatalf("export addresses = %q / %q, want %q and none", app.ruleAddress("multi"), app.ruleAddress("socks"), multi)
	}
	if entries := menuModel([]ruleState{*app.rules[0], *app.rules[1]}); !entries[0].localProxy || entries[1].localProxy {
		test.Fatalf("SOCKS5-only rule offered as system proxy: %+v", entries)
	}
	app.selectSystemProxy("socks")
	app.selectSystemProxy("multi")
	if !reflect.DeepEqual(calls, []string{"", multi}) || app.systemProxyRule != "multi" {
		test.Fatalf("selection calls = %q, rule = %q", calls, app.systemProxyRule)
	}
	conf.Rules[0].Listen.Protocols = conf.Rules[0].Listen.Protocols[1:]
	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	app.restoreSystemProxy()
	if !reflect.DeepEqual(calls, []string{"", multi, ""}) || app.systemProxyRule != "" || app.Mode != i18n.ManualProxy() {
		test.Fatalf("dropping HTTP kept the system proxy: calls = %q, rule = %q", calls, app.systemProxyRule)
	}
	if app.ruleAddress("multi") != "" || app.primaryAddress() != "" {
		test.Fatal("SOCKS5-only rule still exported or primary after reload")
	}
	multi = app.Status().Rules[0].Address
	if code, err := requestThrough(test, &url.URL{Scheme: "http", Host: multi, User: alice}, target.URL); err == nil {
		test.Fatalf("HTTP still served after dropping it: status %d", code)
	}
	if code, err := requestThrough(test, &url.URL{Scheme: "socks5", Host: multi, User: bob}, target.URL); err != nil || code != http.StatusOK {
		test.Fatalf("SOCKS5 after reload = %d, %v", code, err)
	}
}

// requestThrough fetches target through proxyURL; a refused proxy yields an error that is never a timeout.
func requestThrough(test *testing.T, proxyURL *url.URL, target string) (int, error) {
	test.Helper()
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		connection, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return connection, connection.SetDeadline(time.Now().Add(2 * time.Second))
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DialContext: dial, DisableKeepAlives: true}
	switch proxyURL.Scheme {
	case "socks5":
		proxy, err := socks5.NewDialer(proxyURL.String())
		if err != nil {
			test.Fatal(err)
		}
		proxy.ProxyDial = dial
		transport.Proxy, transport.DialContext = nil, proxy.DialContext
	case "socks4":
		proxy, err := socks4.NewDialer(proxyURL.String())
		if err != nil {
			test.Fatal(err)
		}
		proxy.ProxyDial = dial
		transport.Proxy, transport.DialContext = nil, proxy.DialContext
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	response, err := client.Get(target)
	if err != nil {
		var networkError net.Error
		if errors.As(err, &networkError) && networkError.Timeout() {
			test.Fatalf("request through %s timed out instead of being refused: %v", proxyURL.Redacted(), err)
		}
		return 0, err
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	return response.StatusCode, nil
}

func newTestApp(test *testing.T, conf *config.Config) *App {
	test.Helper()
	store := config.NewStore(test.TempDir())
	if err := store.Save(conf); err != nil {
		test.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	app := &App{store: store, actions: make(chan func()), metrics: metrics.NewRegistry(), metricsCancel: cancel}
	go app.metrics.Run(ctx)
	app.web = web.NewHandler(configs.NewConfigsService(store, app), stats.NewStatsService(app.metrics), metrics.NewHandler(app.metrics))
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

func startMultiEchoServer(test *testing.T) net.Listener {
	test.Helper()
	listener := occupyPort(test)
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer connection.Close()
				io.Copy(connection, connection)
			}()
		}
	}()
	return listener
}

func assertTunnelEcho(test *testing.T, connection net.Conn, reader io.Reader, payload string) {
	test.Helper()
	if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		test.Fatal(err)
	}
	if _, err := io.WriteString(connection, payload); err != nil {
		test.Fatalf("write %q: %v", payload, err)
	}
	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(reader, reply); err != nil {
		test.Fatalf("read %q: %v", payload, err)
	}
	if string(reply) != payload {
		test.Fatalf("echo = %q, want %q", reply, payload)
	}
}

func assertTunnelClosed(test *testing.T, connection net.Conn) {
	test.Helper()
	if err := connection.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		test.Fatal(err)
	}
	size, err := connection.Read(make([]byte, 1))
	if size != 0 || err == nil {
		test.Fatalf("Read after reload = (%d, %v), want EOF or reset", size, err)
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		test.Fatalf("reload did not close the tunnel: %v", err)
	}
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

func waitForSnapshot(test *testing.T, app *App, ready func(metrics.Snapshot) bool) metrics.Snapshot {
	test.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot := app.metrics.Snapshot()
		if ready(snapshot) {
			return snapshot
		}
		select {
		case <-deadline.C:
			test.Fatalf("statistics did not converge: %+v", snapshot)
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
