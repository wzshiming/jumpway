package tray

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/wzshiming/jumpway/i18n"
)

func TestMenuModelRules(test *testing.T) {
	rules := []ruleState{
		{name: "alpha", address: "127.0.0.1:1097", running: true},
		{name: "beta", address: "127.0.0.1:1099", attempt: 3},
		{name: "remote", address: "0.0.0.0:2097", remote: true, running: true},
		{name: "database", address: "127.0.0.1:15432", target: "10.0.0.5:5432"},
		{name: "wildcard", listenAddress: "0.0.0.0:1197"},
	}
	want := []ruleMenuEntry{
		{
			name: "alpha", address: "127.0.0.1:1097", label: "alpha (127.0.0.1:1097)",
			status: "alpha \u00b7 127.0.0.1:1097 \u00b7 " + i18n.Running(), localProxy: true,
		},
		{
			name: "beta", address: "127.0.0.1:1099", label: "beta (127.0.0.1:1099)",
			status: "beta \u00b7 127.0.0.1:1099 \u00b7 " + i18n.Retrying(3), localProxy: true,
		},
		{
			name: "remote", address: "0.0.0.0:2097", label: "remote (0.0.0.0:2097)",
			status: "remote \u00b7 0.0.0.0:2097 \u00b7 " + i18n.Running() + " \u00b7 " + i18n.Remote(),
		},
		{
			name: "database", address: "127.0.0.1:15432", label: "database (127.0.0.1:15432)",
			status: "database \u00b7 127.0.0.1:15432 \u2192 10.0.0.5:5432 \u00b7 " + i18n.Stopped(),
		},
		{
			name: "wildcard", address: "127.0.0.1:1197", label: "wildcard (127.0.0.1:1197)",
			status: "wildcard \u00b7 127.0.0.1:1197 \u00b7 " + i18n.Stopped(), localProxy: true,
		},
	}
	if got := menuModel(rules); !reflect.DeepEqual(got, want) {
		test.Fatalf("menu model = %#v, want %#v", got, want)
	}
}

func TestBuildMenuRuleItems(test *testing.T) {
	app := &App{
		webAddress: "127.0.0.1:1098",
		rules: []*ruleState{
			{name: "alpha", address: "127.0.0.1:1097", running: true},
			{name: "beta", address: "127.0.0.1:1099", running: true},
			{name: "remote", address: "127.0.0.1:2097", remote: true},
			{name: "database", address: "127.0.0.1:15432", target: "10.0.0.5:5432"},
		},
	}
	if app.buildMenu() == nil {
		test.Fatal("missing menu")
	}
	items := app.menuItems
	if items == nil || items.web == nil || !items.web.IsDisabled() || len(items.rules) != 4 {
		test.Fatalf("missing status items: %+v", items)
	}
	for name, item := range items.rules {
		if !item.IsDisabled() {
			test.Fatalf("status %q is enabled", name)
		}
	}
	if len(items.proxies) != 2 || len(items.exports) != 2 || !items.manual.IsChecked() {
		test.Fatalf("unexpected proxy/export structure: %+v", items)
	}
	for _, name := range []string{"alpha", "beta"} {
		if items.proxies[name] == nil || items.exports[name] == nil || items.proxies[name].IsChecked() {
			test.Fatalf("missing or selected local proxy %q", name)
		}
	}
	app.systemProxyRule = "beta"
	app.buildMenu()
	if app.menuItems == items || app.menuItems.manual.IsChecked() || !app.menuItems.proxies["beta"].IsChecked() || app.menuItems.proxies["alpha"].IsChecked() {
		test.Fatal("rebuild lost the exclusive proxy selection")
	}
	if address := app.ruleAddress("beta"); address != "127.0.0.1:1099" {
		test.Fatalf("rule address = %q", address)
	}
	app.rules[1].address = "127.0.0.1:1199"
	if address := app.ruleAddress("beta"); address != "127.0.0.1:1199" {
		test.Fatalf("export retained a stale address: %q", address)
	}
	if app.ruleAddress("remote") != "" || app.ruleAddress("database") != "" || app.ruleAddress("missing") != "" {
		test.Fatal("non-local proxy address was exported")
	}
}

func TestBuildMenuNoLocalProxy(test *testing.T) {
	for _, rules := range [][]*ruleState{nil, {
		{name: "remote", remote: true},
		{name: "forward", target: "127.0.0.1:5432"},
	}} {
		app := &App{rules: rules}
		app.buildMenu()
		items := app.menuItems
		if !items.manual.IsChecked() || len(items.proxies) != 0 || len(items.exports) != 0 || items.noLocalRule == nil || !items.noLocalRule.IsDisabled() {
			test.Fatalf("unexpected empty proxy/export menu: %+v", items)
		}
	}
}

func TestMenuSnapshotStatus(test *testing.T) {
	app := &App{
		webAddress: "127.0.0.1:1098", Mode: i18n.ManualProxy(),
		rules: []*ruleState{{name: "alpha", address: "127.0.0.1:1097", running: true}},
	}
	before := app.menuSnapshot()
	app.rules[0].running, app.rules[0].attempt = false, 4
	app.webErr = errors.New("bind failed")
	after := app.menuSnapshot()
	if before.web != i18n.WebUI()+" \u00b7 127.0.0.1:1098 \u00b7 "+i18n.ManualProxy() || after.web != i18n.WebUI()+" \u00b7 bind failed \u00b7 "+i18n.ManualProxy() {
		test.Fatalf("web status = %q / %q", before.web, after.web)
	}
	if before.rules[0].status != "alpha \u00b7 127.0.0.1:1097 \u00b7 "+i18n.Running() || after.rules[0].status != "alpha \u00b7 127.0.0.1:1097 \u00b7 "+i18n.Retrying(4) {
		test.Fatalf("rule status = %q / %q", before.rules[0].status, after.rules[0].status)
	}
}

func TestUpdateStatusConcurrentRebuild(test *testing.T) {
	app := &App{rules: []*ruleState{{name: "alpha", address: "127.0.0.1:1097"}}}
	app.updateStatus()
	app.buildMenu()
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		for attempt := 1; attempt <= 20; attempt++ {
			app.mu.Lock()
			app.rules[0].attempt = attempt
			app.mu.Unlock()
			app.updateStatus()
		}
	}()
	for rebuild := 0; rebuild < 5; rebuild++ {
		app.buildMenu()
	}
	workers.Wait()
	if len(app.menuItems.rules) != 1 {
		test.Fatal("status item lost during rebuild")
	}
}

func TestUpdateStatusRemovedSelection(test *testing.T) {
	app := &App{
		systemProxyRule: "alpha",
		rules:           []*ruleState{{name: "alpha", address: "127.0.0.1:1097"}},
	}
	app.buildMenu()
	if !app.menuItems.proxies["alpha"].IsChecked() || app.menuItems.manual.IsChecked() {
		test.Fatal("initial selection missing")
	}
	app.rules = nil
	app.updateStatus()
	if app.menuItems.proxies["alpha"].IsChecked() || !app.menuItems.manual.IsChecked() {
		test.Fatal("removed selection left two checked items before rebuild")
	}
}

func TestSystemProxySelectionReload(test *testing.T) {
	app := &App{
		systemProxyRule: "alpha",
		rules: []*ruleState{
			{name: "alpha", address: "127.0.0.1:1097"},
			{name: "beta", address: "127.0.0.1:1099"},
		},
	}
	address, changed, removed := app.syncSystemProxySelection()
	if address != "127.0.0.1:1097" || !changed || removed != "" || app.Mode != i18n.SystemProxy() {
		test.Fatalf("initial selection = %q, %v, %q", address, changed, removed)
	}
	if _, changed, _ := app.syncSystemProxySelection(); changed {
		test.Fatal("unchanged address reapplied")
	}
	app.rules[0].address = "127.0.0.1:1197"
	if address, changed, _ := app.syncSystemProxySelection(); address != "127.0.0.1:1197" || !changed || app.systemProxyRule != "alpha" {
		test.Fatalf("moved selection = %q, %v", address, changed)
	}
	app.rules = app.rules[1:]
	address, changed, removed = app.syncSystemProxySelection()
	if address != "" || !changed || removed != "alpha" || app.systemProxyRule != "" || app.Mode != i18n.ManualProxy() {
		test.Fatalf("removed selection = %q, %v, %q", address, changed, removed)
	}
	for _, rule := range []*ruleState{{name: "beta", remote: true}, {name: "beta", target: "127.0.0.1:5432"}} {
		app.rules, app.systemProxyRule = []*ruleState{rule}, "beta"
		if address, _, removed := app.syncSystemProxySelection(); address != "" || removed != "beta" || app.systemProxyRule != "" {
			test.Fatal("non-local proxy remained selected")
		}
	}
}
