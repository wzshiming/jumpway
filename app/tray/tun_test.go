package tray

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/app/web/services/configs"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/tun"
	"github.com/wzshiming/tun/stack"
	wgtun "golang.zx2c4.com/wireguard/tun"
)

// ttWait bounds every wait for something that must happen.
const ttWait = 5 * time.Second

// ttTUNAddresses and ttTUNMTU are what every TUN entry is opened with.
var ttTUNAddresses = []netip.Prefix{netip.MustParsePrefix("198.18.0.1/30"), netip.MustParsePrefix("fd00:6a77:7761::1/64")}

const ttTUNMTU = 1500

// ttTUNName is the interface name every TUN entry is opened with.
func ttTUNName() string {
	if runtime.GOOS == "windows" {
		return "JumpWay"
	}
	return "jumpway"
}

func TestTUNDialerPassesClientAddr(test *testing.T) {
	client := &net.TCPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000}
	for _, scenario := range []struct {
		name   string
		client net.Addr
		want   string
	}{
		{name: "captured flow", client: client, want: client.String()},
		{name: "no client", want: ""},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			inner := &ttClientAddrDialer{}
			ctx := stack.WithClientAddr(context.Background(), scenario.client)
			if _, err := (tunDialer{inner: inner}).DialContext(ctx, "tcp", "192.0.2.1:443"); err != nil {
				test.Fatal(err)
			}
			if inner.client != scenario.want || inner.address != "192.0.2.1:443" {
				test.Fatalf("rule dialer saw client %q dialing %q, want client %q dialing 192.0.2.1:443", inner.client, inner.address, scenario.want)
			}
		})
	}
}

// ttClientAddrDialer records the jumpway client address and the address of its last dial.
type ttClientAddrDialer struct {
	client, address string
}

func (dialer *ttClientAddrDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer.client = jumpway.ClientAddr(ctx)
	dialer.address = address
	conn, peer := net.Pipe()
	peer.Close()
	return conn, nil
}

func TestRuleStateTUNCapable(test *testing.T) {
	for _, scenario := range []struct {
		name  string
		state ruleState
		want  bool
	}{
		{name: "http proxy", state: ruleState{http: true}, want: true},
		{name: "socks proxy", state: ruleState{}, want: true},
		{name: "forward", state: ruleState{target: "10.0.0.5:5432"}, want: true},
		{name: "remote", state: ruleState{remote: true, http: true}},
		{name: "virtual", state: ruleState{virtual: true, http: true}},
	} {
		if got := scenario.state.tunCapable(); got != scenario.want {
			test.Errorf("%s: tunCapable() = %v, want %v", scenario.name, got, scenario.want)
		}
	}
}

func TestBuildMenuTUNItems(test *testing.T) {
	ttForceTUNSupported(test)
	app := &App{rules: []*ruleState{
		{name: "alpha", address: "127.0.0.1:1097", http: true, running: true},
		{name: "database", address: "127.0.0.1:15432", target: "10.0.0.5:5432"},
		{name: "socks", address: "127.0.0.1:1081", running: true},
		{name: "remote", address: "0.0.0.0:2097", remote: true, http: true},
		{name: "virtual", address: "virtual://x", virtual: true, http: true, running: true},
	}}
	app.buildMenu()
	ttAssertTUNMenu(test, app, "", "alpha", "database", "socks")
	app.tunRule = "database"
	app.buildMenu()
	ttAssertTUNMenu(test, app, "database", "alpha", "database", "socks")
}

func TestBuildMenuWithoutTUNCapableRule(test *testing.T) {
	ttForceTUNSupported(test)
	app := &App{rules: []*ruleState{
		{name: "remote", address: "0.0.0.0:2097", remote: true, http: true},
		{name: "virtual", address: "virtual://x", virtual: true, http: true},
	}}
	app.buildMenu()
	ttAssertTUNMenu(test, app, "")
}

func TestBuildMenuOmitsTUNWhenUnsupported(test *testing.T) {
	previous := tunSupported
	tunSupported = false
	test.Cleanup(func() { tunSupported = previous })
	app := &App{rules: []*ruleState{
		{name: "alpha", address: "127.0.0.1:1097", http: true, running: true},
		{name: "database", address: "127.0.0.1:15432", target: "10.0.0.5:5432"},
	}, systemProxyRule: "alpha", tunRule: "alpha"}
	app.buildMenu()
	items := app.menuItems
	if items.tunOff != nil || len(items.tun) != 0 {
		test.Fatalf("unsupported platform: TUN Off = %v, TUN items = %v; want none", items.tunOff, items.tun)
	}
	if items.web == nil || items.manual == nil || items.manual.IsChecked() || items.noLocalRule != nil {
		test.Fatalf("unsupported platform changed the other items: %+v", items)
	}
	if proxy := items.proxies["alpha"]; len(items.proxies) != 1 || proxy == nil || !proxy.IsChecked() {
		test.Fatalf("System Proxy items = %v, want alpha checked", items.proxies)
	}
	if got := slices.Sorted(maps.Keys(items.rules)); !slices.Equal(got, []string{"alpha", "database"}) {
		test.Fatalf("rule status items = %v, want alpha and database", got)
	}
	if got := slices.Sorted(maps.Keys(items.exports)); !slices.Equal(got, []string{"alpha"}) {
		test.Fatalf("export items = %v, want alpha", got)
	}

	tunSupported = true
	app.buildMenu()
	ttAssertTUNMenu(test, app, "alpha", "alpha", "database")
}

func TestUpdateStatusSyncsTUNChecks(test *testing.T) {
	ttForceTUNSupported(test)
	app := &App{rules: []*ruleState{
		{name: "alpha", address: "127.0.0.1:1097", http: true},
		{name: "beta", address: "127.0.0.1:1099", http: true},
	}}
	app.buildMenu()
	app.tunRule = "beta"
	app.updateStatus()
	ttAssertTUNMenu(test, app, "beta", "alpha", "beta")
	app.tunRule = ""
	app.updateStatus()
	ttAssertTUNMenu(test, app, "", "alpha", "beta")
}

func TestEnabledRulesTUNFingerprints(test *testing.T) {
	rules := []config.Rule{
		ttTUNRule("alpha", 1080),
		{Name: "database", Listen: config.Listen{Port: 15432}, Forward: config.Forward{Host: "10.0.0.5", Port: 5432}},
		{Name: "beta", Listen: config.Listen{Port: 1081}},
		{Name: "off", Disabled: true, Listen: config.Listen{Port: 1082}},
	}
	names := []string{"alpha", "database", "beta"}
	noProxy, otherNoProxy := []string{"10.0.0.0/8"}, []string{"192.0.2.0/24"}
	capture := []netip.Prefix{netip.MustParsePrefix("0.0.0.0/1"), netip.MustParsePrefix("128.0.0.0/1")}
	otherCapture := []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")}
	prints := func(noProxy []string, selected string, capture []netip.Prefix) map[string]string {
		test.Helper()
		enabled, fingerprints := enabledRules(rules, noProxy, selected, capture)
		got := make([]string, 0, len(enabled))
		for _, rule := range enabled {
			got = append(got, rule.Name)
		}
		if !slices.Equal(got, names) || len(fingerprints) != len(names) {
			test.Fatalf("enabledRules kept %v with %d fingerprints, want %v", got, len(fingerprints), names)
		}
		return fingerprints
	}
	off := prints(noProxy, "", nil)
	if prints(otherNoProxy, "", nil)["alpha"] == off["alpha"] {
		test.Fatal("TUN off: a proxy rule's fingerprint ignores no_proxy")
	}
	if prints(otherNoProxy, "", nil)["database"] != off["database"] {
		test.Fatal("TUN off: a forward rule's fingerprint folds in no_proxy")
	}
	if !maps.Equal(prints(noProxy, "", capture), off) {
		test.Fatal("TUN off: fingerprints fold in a capture")
	}
	onAlpha := prints(noProxy, "alpha", capture)
	for _, name := range names {
		if onAlpha[name] == off[name] {
			test.Fatalf("selecting alpha left %q's fingerprint unchanged", name)
		}
	}
	if prints(otherNoProxy, "alpha", capture)["database"] == onAlpha["database"] {
		test.Fatal("TUN on: a forward rule's fingerprint ignores no_proxy")
	}
	for name, fingerprint := range prints(noProxy, "alpha", otherCapture) {
		if fingerprint == onAlpha[name] {
			test.Fatalf("TUN on: %q's fingerprint ignores the capture", name)
		}
	}
	onBeta := prints(noProxy, "beta", capture)
	if onBeta["alpha"] == onAlpha["alpha"] || onBeta["beta"] == onAlpha["beta"] {
		test.Fatal("moving the TUN from alpha to beta left either rule's fingerprint unchanged")
	}
	if onBeta["database"] != onAlpha["database"] {
		test.Fatal("moving the TUN between other rules changed the database rule's fingerprint")
	}
}

func TestReloadLeavesTUNOff(test *testing.T) {
	opener := &ttTUNOpener{}
	app, recorder := ttNewTUNApp(test, &config.Config{Rules: []config.Rule{
		ttTUNRule("alpha", 0),
		{Name: "database", Listen: config.Listen{Host: "127.0.0.1"}, Forward: config.Forward{Host: "127.0.0.1", Port: 9}},
		{Name: "virtual", Listen: config.Listen{Virtual: "tt-entry"}},
		{Name: "disabled", Disabled: true, Listen: config.Listen{Host: "127.0.0.1"}},
	}}, opener)
	status := ttStart(test, app)
	for _, rule := range status.Rules {
		if rule.Tun != "" {
			test.Fatalf("rule %q runs on TUN %q at launch, want none", rule.Name, rule.Tun)
		}
	}
	ttAssertTUNMenu(test, app, "", "alpha", "database")
	if selected := ttSelectedTUN(app); selected != "" {
		test.Fatalf("TUN selection at launch = %q, want off", selected)
	}
	if captures, _ := recorder.recorded(); opener.calls.Load() != 0 || len(captures) != 0 {
		test.Fatalf("launch opened %d TUNs and built %d bypass dialers, want none", opener.calls.Load(), len(captures))
	}
}

func TestSelectTUNOpensDevice(test *testing.T) {
	opener := &ttTUNOpener{}
	app, _ := ttNewTUNApp(test, &config.Config{
		NoProxy: config.NoProxy{List: []string{"10.0.0.0/8", "192.0.2.7", "example.invalid"}},
		Rules:   []config.Rule{ttTUNRule("alpha", 0), ttTUNRule("beta", 0)},
	}, opener)
	ttStart(test, app)
	app.selectTUN("alpha")
	ttAssertTUNMenu(test, app, "alpha", "alpha", "beta")
	if selected := ttSelectedTUN(app); selected != "alpha" {
		test.Fatalf("TUN selection = %q, want alpha", selected)
	}
	status := ttWaitStatus(test, app, func(status configs.Status) bool {
		return status.Rules[0].Running && status.Rules[0].Tun == "utun9" && status.Rules[1].Running
	})
	if _, _, err := net.SplitHostPort(status.Rules[0].Address); err != nil || status.Rules[0].Error != "" || status.Rules[1].Tun != "" {
		test.Fatalf("status = %+v, want alpha's http listener beside TUN utun9, and no TUN on beta", status.Rules)
	}
	want := tun.Config{
		Name:      ttTUNName(),
		Addresses: ttTUNAddresses,
		MTU:       ttTUNMTU,
		Bypass:    []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("192.0.2.7/32")},
	}
	if calls := opener.calls.Load(); calls != 1 {
		test.Fatalf("open calls = %d, want 1", calls)
	}
	if got := opener.request(0); !reflect.DeepEqual(got, want) {
		test.Fatalf("opened TUN with %+v, want %+v", got, want)
	}
	for index, want := range []string{"utun9", ""} {
		data, err := json.Marshal(status.Rules[index])
		if err != nil {
			test.Fatal(err)
		}
		var object map[string]any
		if err := json.Unmarshal(data, &object); err != nil {
			test.Fatal(err)
		}
		if value, ok := object["tun"]; ok != (want != "") || ok && value != want {
			test.Fatalf("rule %q JSON = %s, want tun %q", status.Rules[index].Name, data, want)
		}
	}
}

func TestSelectTUNOffRestartsWithoutDevice(test *testing.T) {
	opener := &ttTUNOpener{}
	app, _ := ttNewTUNApp(test, &config.Config{Rules: []config.Rule{ttTUNRule("alpha", 0)}}, opener)
	ttStart(test, app)
	app.selectTUN("alpha")
	ttWaitStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Running && status.Rules[0].Tun == "utun9" })
	app.selectTUN("")
	ttAssertTUNMenu(test, app, "", "alpha")
	if selected := ttSelectedTUN(app); selected != "" {
		test.Fatalf("TUN selection after Off = %q, want none", selected)
	}
	ttWaitUntil(test, "the device to close", opener.device(0).isClosed)
	ttWaitStatus(test, app, func(status configs.Status) bool {
		return status.Rules[0].Running && status.Rules[0].Tun == "" && status.Rules[0].Error == ""
	})
	if calls := opener.calls.Load(); calls != 1 {
		test.Fatalf("open calls after Off = %d, want 1", calls)
	}
}

func TestSelectTUNMovesToAnotherRule(test *testing.T) {
	opener := &ttTUNOpener{}
	app, _ := ttNewTUNApp(test, &config.Config{Rules: []config.Rule{ttTUNRule("alpha", 0), ttTUNRule("beta", 0)}}, opener)
	ttStart(test, app)
	app.selectTUN("alpha")
	ttWaitStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Running && status.Rules[0].Tun == "utun9" })
	app.selectTUN("beta")
	ttAssertTUNMenu(test, app, "beta", "alpha", "beta")
	if selected := ttSelectedTUN(app); selected != "beta" {
		test.Fatalf("TUN selection = %q, want beta", selected)
	}
	ttWaitUntil(test, "alpha's device to close", opener.device(0).isClosed)
	ttWaitStatus(test, app, func(status configs.Status) bool {
		return status.Rules[0].Running && status.Rules[0].Tun == "" && status.Rules[1].Running && status.Rules[1].Tun == "utun9"
	})
	if calls, closed := opener.calls.Load(), opener.device(1).isClosed(); calls != 2 || closed {
		test.Fatalf("open calls = %d, beta's device closed = %v; want 2 and open", calls, closed)
	}
}

func TestSelectTUNOpenFailureStopsRule(test *testing.T) {
	for _, scenario := range []struct {
		name           string
		err            error
		want, unwanted string
	}{
		{name: "denied", err: fmt.Errorf("tt helper: %w", tun.ErrDenied), want: i18n.TUNDenied(), unwanted: i18n.TUNUnavailable()},
		{name: "unsupported", err: errors.ErrUnsupported, want: i18n.TUNUnavailable(), unwanted: i18n.TUNDenied()},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			port, address := ttFreePort(test)
			opener := &ttTUNOpener{err: scenario.err}
			app, _ := ttNewTUNApp(test, &config.Config{Rules: []config.Rule{ttTUNRule("alpha", port)}}, opener)
			ttStart(test, app)
			app.selectTUN("alpha")
			rule := ttWaitStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Error != "" }).Rules[0]
			if rule.Running || rule.Attempt != 0 || rule.Tun != "" || !strings.HasPrefix(rule.Error, scenario.want+": ") ||
				strings.Contains(rule.Error, scenario.unwanted) || !strings.Contains(rule.Error, scenario.err.Error()) {
				test.Fatalf("status = %+v, want stopped at attempt 0 with %q and the cause %q", rule, scenario.want+": ", scenario.err)
			}
			ttWaitUntil(test, "alpha's listener to close", func() bool { return ttRefused(address) })
			// A retry on the 100 ms listen backoff would land inside this window.
			time.Sleep(300 * time.Millisecond)
			if calls := opener.calls.Load(); calls != 1 {
				test.Fatalf("open calls = %d, want 1 with no retry", calls)
			}
			ttAssertRefused(test, address)
			if rule := app.Status().Rules[0]; rule.Running || rule.Attempt != 0 {
				test.Fatalf("status after the retry window = %+v, want still stopped at attempt 0", rule)
			}
			if status := app.menuSnapshot().rules[0].status; !strings.HasSuffix(status, " \u00b7 "+i18n.Stopped()) {
				test.Fatalf("menu status = %q, want it to end in %q", status, i18n.Stopped())
			}
			if selected := ttSelectedTUN(app); selected != "alpha" {
				test.Fatalf("TUN selection after the failure = %q, want alpha kept", selected)
			}
			ttAssertTUNMenu(test, app, "alpha", "alpha")
		})
	}
}

func TestSelectTUNOpenFailureRecovery(test *testing.T) {
	for _, scenario := range []struct {
		name      string
		recover   func(*App)
		selected  string
		wantCalls int32
	}{
		{name: "off", recover: func(app *App) { app.selectTUN("") }, wantCalls: 1},
		{name: "reselect", recover: func(app *App) { app.selectTUN("alpha") }, selected: "alpha", wantCalls: 2},
		{name: "reload", recover: func(app *App) { _ = app.Reload() }, selected: "alpha", wantCalls: 2},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			opener := &ttTUNOpener{err: errors.New("tt open failed")}
			app, _ := ttNewTUNApp(test, &config.Config{Rules: []config.Rule{ttTUNRule("alpha", 0)}}, opener)
			ttStart(test, app)
			app.selectTUN("alpha")
			ttWaitStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Error != "" })
			scenario.recover(app)
			if scenario.selected == "" {
				ttWaitStatus(test, app, func(status configs.Status) bool {
					return status.Rules[0].Running && status.Rules[0].Tun == "" && status.Rules[0].Error == ""
				})
			} else {
				ttWaitUntil(test, "the TUN to be opened again", func() bool { return opener.calls.Load() >= scenario.wantCalls })
				rule := ttWaitStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Error != "" }).Rules[0]
				if rule.Running || rule.Attempt != 0 || !strings.HasPrefix(rule.Error, i18n.TUNUnavailable()+": ") {
					test.Fatalf("status after the retry = %+v, want stopped at attempt 0 with the TUN error", rule)
				}
			}
			time.Sleep(300 * time.Millisecond)
			if calls := opener.calls.Load(); calls != scenario.wantCalls {
				test.Fatalf("open calls = %d, want %d", calls, scenario.wantCalls)
			}
			if selected := ttSelectedTUN(app); selected != scenario.selected {
				test.Fatalf("TUN selection = %q, want %q", selected, scenario.selected)
			}
			ttAssertTUNMenu(test, app, scenario.selected, "alpha")
		})
	}
}

func TestSelectTUNWithInvalidConfigKeepsOff(test *testing.T) {
	conf := &config.Config{Rules: []config.Rule{ttTUNRule("alpha", 0), ttTUNRule("beta", 0)}}
	opener := &ttTUNOpener{}
	app, _ := ttNewTUNApp(test, conf, opener)
	ttStart(test, app)
	if err := app.store.SaveRaw([]byte("rules: [")); err != nil {
		test.Fatal(err)
	}
	app.selectTUN("alpha")
	if selected, calls := ttSelectedTUN(app), opener.calls.Load(); selected != "" || calls != 0 {
		test.Fatalf("selecting with an invalid config: selection = %q, open calls = %d; want off and none", selected, calls)
	}
	app.updateStatus()
	ttAssertTUNMenu(test, app, "", "alpha", "beta")

	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if selected, calls, tun := ttSelectedTUN(app), opener.calls.Load(), app.Status().Rules[0].Tun; selected != "" || calls != 0 || tun != "" {
		test.Fatalf("reload after the fix: selection = %q, open calls = %d, alpha's TUN = %q; want TUN still off", selected, calls, tun)
	}
}

func TestSelectTUNWithInvalidConfigKeepsPrevious(test *testing.T) {
	conf := &config.Config{Rules: []config.Rule{ttTUNRule("alpha", 0), ttTUNRule("beta", 0)}}
	opener := &ttTUNOpener{}
	app, _ := ttNewTUNApp(test, conf, opener)
	ttStart(test, app)
	app.selectTUN("alpha")
	ttWaitStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Tun == "utun9" })
	if err := app.store.SaveRaw([]byte("rules: [")); err != nil {
		test.Fatal(err)
	}
	app.selectTUN("beta")
	if selected, calls, closed := ttSelectedTUN(app), opener.calls.Load(), opener.device(0).isClosed(); selected != "alpha" || calls != 1 || closed {
		test.Fatalf("moving with an invalid config: selection = %q, open calls = %d, alpha's device closed = %v; want alpha kept open", selected, calls, closed)
	}
	app.updateStatus()
	ttAssertTUNMenu(test, app, "alpha", "alpha", "beta")

	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if selected, calls, closed := ttSelectedTUN(app), opener.calls.Load(), opener.device(0).isClosed(); selected != "alpha" || calls != 1 || closed {
		test.Fatalf("reload after the fix: selection = %q, open calls = %d, alpha's device closed = %v; want alpha kept open", selected, calls, closed)
	}
}

func TestReloadKeepsSelectedTUNDevice(test *testing.T) {
	conf := &config.Config{Rules: []config.Rule{ttTUNRule("alpha", 0)}}
	opener := &ttTUNOpener{}
	app, _ := ttNewTUNApp(test, conf, opener)
	running := func(status configs.Status) bool { return status.Rules[0].Running && status.Rules[0].Tun == "utun9" }
	ttStart(test, app)
	app.selectTUN("alpha")
	ttWaitStatus(test, app, running)
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	if calls, closed := opener.calls.Load(), opener.device(0).isClosed(); calls != 1 || closed {
		test.Fatalf("unchanged reload: open calls = %d, first device closed = %v; want the device kept", calls, closed)
	}
	if selected := ttSelectedTUN(app); selected != "alpha" {
		test.Fatalf("TUN selection after an unchanged reload = %q, want alpha", selected)
	}
	ttAssertTUNMenu(test, app, "alpha", "alpha")
	conf.NoProxy.List = []string{"10.0.0.0/8"}
	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	ttWaitUntil(test, "a second open after the no_proxy change", func() bool { return opener.calls.Load() >= 2 })
	ttWaitUntil(test, "the first device to close", opener.device(0).isClosed)
	ttWaitStatus(test, app, running)
	if calls := opener.calls.Load(); calls != 2 {
		test.Fatalf("open calls after the no_proxy change = %d, want 2", calls)
	}
}

func TestReloadClearsIneligibleTUNRule(test *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*config.Rule)
	}{
		{name: "deleted"},
		{name: "disabled", change: func(rule *config.Rule) { rule.Disabled = true }},
		{name: "remote", change: func(rule *config.Rule) {
			rule.Listen.Way = []bridgeconfig.Node{{LB: []string{"socks5://127.0.0.1:1"}}}
		}},
		{name: "virtual", change: func(rule *config.Rule) { rule.Listen = config.Listen{Virtual: "tt-alpha"} }},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			original := &config.Config{Rules: []config.Rule{ttTUNRule("alpha", 0), ttTUNRule("beta", 0)}}
			opener := &ttTUNOpener{}
			app, _ := ttNewTUNApp(test, original, opener)
			ttStart(test, app)
			app.selectTUN("alpha")
			ttWaitStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Tun == "utun9" })
			changed := &config.Config{Rules: []config.Rule{ttTUNRule("beta", 0)}}
			if scenario.change != nil {
				changed.Rules = []config.Rule{ttTUNRule("alpha", 0), ttTUNRule("beta", 0)}
				scenario.change(&changed.Rules[0])
			}
			if err := app.store.Save(changed); err != nil {
				test.Fatal(err)
			}
			// The remote listener's first bind may fail; only the TUN selection matters here.
			_ = app.Reload()
			if selected := ttSelectedTUN(app); selected != "" {
				test.Fatalf("TUN selection after alpha became ineligible = %q, want none", selected)
			}
			app.buildMenu()
			ttAssertTUNMenu(test, app, "", "beta")
			ttWaitUntil(test, "alpha's device to close", opener.device(0).isClosed)

			if err := app.store.Save(original); err != nil {
				test.Fatal(err)
			}
			if err := app.Reload(); err != nil {
				test.Fatal(err)
			}
			status := ttWaitStatus(test, app, func(status configs.Status) bool {
				return len(status.Rules) == 2 && status.Rules[0].Running && status.Rules[1].Running
			})
			time.Sleep(300 * time.Millisecond)
			if selected, calls := ttSelectedTUN(app), opener.calls.Load(); selected != "" || calls != 1 || status.Rules[0].Tun != "" {
				test.Fatalf("re-adding alpha: selection = %q, open calls = %d, status = %+v; want TUN still off", selected, calls, status.Rules)
			}
		})
	}
}

func TestStopClearsTUNSelection(test *testing.T) {
	opener := &ttTUNOpener{}
	app, _ := ttNewTUNApp(test, &config.Config{Rules: []config.Rule{ttTUNRule("alpha", 0)}}, opener)
	ttStart(test, app)
	app.selectTUN("alpha")
	ttWaitStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Tun == "utun9" })
	app.stop()
	if selected := ttSelectedTUN(app); selected != "" {
		test.Fatalf("TUN selection after stop = %q, want none", selected)
	}
	ttWaitUntil(test, "the device to close on stop", opener.device(0).isClosed)
	app.selectTUN("alpha")
	time.Sleep(300 * time.Millisecond)
	if selected, calls := ttSelectedTUN(app), opener.calls.Load(); selected != "" || calls != 1 {
		test.Fatalf("selecting after stop: selection = %q, open calls = %d; want none and 1", selected, calls)
	}
}

func TestBypassBaseFollowsTUNSupported(test *testing.T) {
	previous := tunSupported
	test.Cleanup(func() { tunSupported = previous })
	for _, supported := range []bool{false, true} {
		tunSupported = supported
		if got := bypassBase(nil) != local.LOCAL; got != supported {
			test.Errorf("tunSupported = %v: bypassBase applies the bypass dialer = %v, want %v", supported, got, supported)
		}
	}
}

func TestSelectTUNBypassDialer(test *testing.T) {
	bypassed := runtime.GOOS == "darwin" || runtime.GOOS == "linux" || runtime.GOOS == "windows"
	want := tun.CaptureRoutes(tun.Config{
		Addresses: ttTUNAddresses,
		MTU:       ttTUNMTU,
		// The no_proxy list below without its hostname and its malformed CIDR.
		Bypass: []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/8"),
			netip.MustParsePrefix("192.0.2.7/32"),
			netip.MustParsePrefix("2001:db8::/32"),
			netip.MustParsePrefix("3fff::9/128"),
		},
	})
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, "target")
	}))
	test.Cleanup(target.Close)
	for _, scenario := range []struct {
		name     string
		selected bool
	}{
		{name: "selected", selected: true},
		{name: "off"},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			app, recorder := ttNewTUNApp(test, &config.Config{
				NoProxy: config.NoProxy{List: []string{"10.0.0.0/8", "192.0.2.7", "2001:db8::/32", "3fff::9", "example.invalid", "10.1.2.3/33"}},
				Rules:   []config.Rule{ttTUNRule("alpha", 0)},
			}, &ttTUNOpener{})
			ttStart(test, app)
			if scenario.selected {
				app.selectTUN("alpha")
			}
			status := ttWaitStatus(test, app, func(status configs.Status) bool {
				return status.Rules[0].Running && (status.Rules[0].Tun == "utun9") == scenario.selected
			})
			proxy := &url.URL{Scheme: "http", Host: status.Rules[0].Address}
			if code, err := requestThrough(test, proxy, target.URL); err != nil || code != http.StatusOK {
				test.Fatalf("request through the rule = %d, %v; want 200", code, err)
			}
			captures, dialed := recorder.recorded()
			if !scenario.selected || !bypassed {
				if len(captures) != 0 {
					test.Fatalf("newBypassDialer captures = %v, want no call on %s", captures, runtime.GOOS)
				}
				return
			}
			if len(captures) == 0 || slices.ContainsFunc(captures, func(capture []netip.Prefix) bool { return !slices.Equal(capture, want) }) {
				test.Fatalf("newBypassDialer captures = %v, want calls with %v only", captures, want)
			}
			if !slices.Contains(dialed, target.Listener.Addr().String()) {
				test.Fatalf("bypass dialer connected to %q, want the proxied target %s", dialed, target.Listener.Addr())
			}
		})
	}
}

// ttTUNDevice is a TUN without traffic: Read blocks until Close, Write drops every packet.
type ttTUNDevice struct {
	name   string
	events chan wgtun.Event
	done   chan struct{}
	once   sync.Once
}

func (device *ttTUNDevice) File() *os.File                               { return nil }
func (device *ttTUNDevice) Write(bufs [][]byte, offset int) (int, error) { return len(bufs), nil }
func (device *ttTUNDevice) MTU() (int, error)                            { return 1500, nil }
func (device *ttTUNDevice) Name() (string, error)                        { return device.name, nil }
func (device *ttTUNDevice) Events() <-chan wgtun.Event                   { return device.events }
func (device *ttTUNDevice) BatchSize() int                               { return 1 }

func (device *ttTUNDevice) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	<-device.done
	return 0, os.ErrClosed
}

func (device *ttTUNDevice) Close() error {
	device.once.Do(func() {
		close(device.done)
		close(device.events)
	})
	return nil
}

func (device *ttTUNDevice) isClosed() bool {
	select {
	case <-device.done:
		return true
	default:
		return false
	}
}

// ttTUNOpener is a fake App.openTUN recording its calls: it fails with err, or opens a ttTUNDevice named utun9.
type ttTUNOpener struct {
	err      error
	calls    atomic.Int32
	mu       sync.Mutex
	requests []tun.Config
	devices  []*ttTUNDevice
}

func (opener *ttTUNOpener) open(ctx context.Context, cfg tun.Config) (*tun.Device, error) {
	opener.mu.Lock()
	opener.requests = append(opener.requests, cfg)
	opener.mu.Unlock()
	opener.calls.Add(1)
	if opener.err != nil {
		return nil, opener.err
	}
	device := &ttTUNDevice{name: "utun9", events: make(chan wgtun.Event), done: make(chan struct{})}
	opener.mu.Lock()
	opener.devices = append(opener.devices, device)
	opener.mu.Unlock()
	return &tun.Device{Device: device, Interface: "utun9"}, nil
}

func (opener *ttTUNOpener) device(index int) *ttTUNDevice {
	opener.mu.Lock()
	defer opener.mu.Unlock()
	return opener.devices[index]
}

// request is the Config of the open call at index.
func (opener *ttTUNOpener) request(index int) tun.Config {
	opener.mu.Lock()
	defer opener.mu.Unlock()
	return opener.requests[index]
}

// ttBypassRecorder stands in for bypass.NewDialer, recording each capture set and every address its dialers connect to.
type ttBypassRecorder struct {
	mu       sync.Mutex
	captures [][]netip.Prefix
	dialed   []string
}

func (recorder *ttBypassRecorder) newDialer(capture []netip.Prefix) *net.Dialer {
	recorder.mu.Lock()
	recorder.captures = append(recorder.captures, slices.Clone(capture))
	recorder.mu.Unlock()
	return &net.Dialer{Control: func(network, address string, _ syscall.RawConn) error {
		recorder.mu.Lock()
		recorder.dialed = append(recorder.dialed, address)
		recorder.mu.Unlock()
		return nil
	}}
}

func (recorder *ttBypassRecorder) recorded() ([][]netip.Prefix, []string) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return slices.Clone(recorder.captures), slices.Clone(recorder.dialed)
}

// ttNewTUNApp is newTestApp with opener as its TUN and a ttBypassRecorder as newBypassDialer, so no test opens a real TUN or binds to an interface.
func ttNewTUNApp(test *testing.T, conf *config.Config, opener *ttTUNOpener) (*App, *ttBypassRecorder) {
	test.Helper()
	ttForceTUNSupported(test)
	recorder := &ttBypassRecorder{}
	previous := newBypassDialer
	newBypassDialer = recorder.newDialer
	// Registered before newTestApp's cleanup, so it runs once the app has stopped.
	test.Cleanup(func() { newBypassDialer = previous })
	app := newTestApp(test, conf)
	app.openTUN = opener.open
	return app, recorder
}

// ttForceTUNSupported builds the TUN submenu for the rest of test, even where this platform cannot open a TUN.
func ttForceTUNSupported(test *testing.T) {
	previous := tunSupported
	tunSupported = true
	test.Cleanup(func() { tunSupported = previous })
}

// ttTUNRule is the http proxy rule name on 127.0.0.1:port.
func ttTUNRule(name string, port uint32) config.Rule {
	return config.Rule{Name: name, Listen: config.Listen{Host: "127.0.0.1", Port: port, Protocols: []config.Protocol{{Type: config.ProtocolHTTP}}}}
}

// ttStart reloads app, waits for every rule to run, and builds the menu a tray would show.
func ttStart(test *testing.T, app *App) configs.Status {
	test.Helper()
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	status := ttWaitStatus(test, app, func(status configs.Status) bool {
		for _, rule := range status.Rules {
			if !rule.Running {
				return false
			}
		}
		return len(status.Rules) != 0
	})
	app.buildMenu()
	return status
}

func ttSelectedTUN(app *App) string {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.tunRule
}

// ttAssertTUNMenu fails unless the TUN submenu holds exactly an item per name, with only selected checked, or Off when selected is empty.
func ttAssertTUNMenu(test *testing.T, app *App, selected string, names ...string) {
	test.Helper()
	app.mu.Lock()
	items := app.menuItems
	app.mu.Unlock()
	if items == nil || items.tunOff == nil {
		test.Fatalf("menu has no TUN Off item: %+v", items)
	}
	if checked := items.tunOff.IsChecked(); checked != (selected == "") {
		test.Fatalf("TUN Off checked = %v with %q selected", checked, selected)
	}
	got := slices.Sorted(maps.Keys(items.tun))
	if !slices.Equal(got, slices.Sorted(slices.Values(names))) {
		test.Fatalf("TUN items = %v, want %v", got, names)
	}
	for _, name := range names {
		if checked := items.tun[name].IsChecked(); checked != (name == selected) {
			test.Fatalf("TUN item %q checked = %v with %q selected", name, checked, selected)
		}
	}
}

// ttRefused reports whether nothing accepts TCP connections at address.
func ttRefused(address string) bool {
	connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
	if err == nil {
		connection.Close()
	}
	return err != nil
}

// ttFreePort returns a loopback port nothing listens on, and its address.
func ttFreePort(test *testing.T) (uint32, string) {
	test.Helper()
	listener := occupyPort(test)
	if err := listener.Close(); err != nil {
		test.Fatal(err)
	}
	return uint32(listener.Addr().(*net.TCPAddr).Port), listener.Addr().String()
}

// ttAssertRefused fails when something accepts TCP connections at address.
func ttAssertRefused(test *testing.T, address string) {
	test.Helper()
	connection, err := net.DialTimeout("tcp", address, ttWait)
	if err == nil {
		connection.Close()
		test.Fatalf("%s accepted a connection, want it refused", address)
	}
}

func ttWaitStatus(test *testing.T, app *App, ready func(configs.Status) bool) configs.Status {
	test.Helper()
	deadline := time.Now().Add(ttWait)
	for {
		status := app.Status()
		if ready(status) {
			return status
		}
		if time.Now().After(deadline) {
			test.Fatalf("status did not converge: %+v", status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func ttWaitUntil(test *testing.T, what string, ready func() bool) {
	test.Helper()
	deadline := time.Now().Add(ttWait)
	for !ready() {
		if time.Now().After(deadline) {
			test.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
