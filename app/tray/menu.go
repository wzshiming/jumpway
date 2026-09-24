package tray

import (
	"errors"
	"fmt"

	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

type menuItems struct {
	web         *systray.MenuItem
	rules       map[string]*systray.MenuItem
	manual      *systray.MenuItem
	proxies     map[string]*systray.MenuItem
	exports     map[string]*systray.MenuItem
	noLocalRule *systray.MenuItem
}

type menuSnapshot struct {
	web      string
	rules    []ruleMenuEntry
	selected string
	items    *menuItems
}

type ruleMenuEntry struct {
	name       string
	address    string
	label      string
	status     string
	localProxy bool
}

func menuModel(rules []ruleState) []ruleMenuEntry {
	entries := make([]ruleMenuEntry, 0, len(rules))
	for _, rule := range rules {
		address := rule.address
		if address == "" {
			address = rule.listenAddress
		}
		if !rule.remote {
			address = formatAddress(address)
		}
		state := i18n.Stopped()
		if rule.running {
			state = i18n.Running()
		} else if rule.attempt > 0 {
			state = i18n.Retrying(rule.attempt)
		}
		status := rule.name + " \u00b7 " + address
		if rule.target != "" {
			status += " \u2192 " + rule.target
		}
		status += " \u00b7 " + state
		if rule.remote {
			status += " \u00b7 " + i18n.Remote()
		}
		entries = append(entries, ruleMenuEntry{
			name:       rule.name,
			address:    address,
			label:      rule.name + " (" + address + ")",
			status:     status,
			localProxy: rule.localProxy(),
		})
	}
	return entries
}

func (a *App) menuSnapshot() menuSnapshot {
	a.mu.Lock()
	rules := make([]ruleState, len(a.rules))
	for index, rule := range a.rules {
		rules[index] = *rule
	}
	address, webErr, lastErr := a.webAddress, a.webErr, a.lastErr
	mode, selected, items := a.Mode, a.systemProxyRule, a.menuItems
	a.mu.Unlock()
	if lastErr != nil {
		address = lastErr.Error()
	} else if webErr != nil {
		address = webErr.Error()
	}
	if mode == "" {
		mode = i18n.ManualProxy()
	}
	return menuSnapshot{
		web:      i18n.WebUI() + " \u00b7 " + address + " \u00b7 " + mode,
		rules:    menuModel(rules),
		selected: selected,
		items:    items,
	}
}

func (a *App) buildMenu() *systray.Menu {
	snapshot := a.menuSnapshot()
	menu := systray.NewMenu()
	items := &menuItems{
		web:     menu.Add(snapshot.web, nil),
		rules:   make(map[string]*systray.MenuItem),
		proxies: make(map[string]*systray.MenuItem),
		exports: make(map[string]*systray.MenuItem),
	}
	items.web.SetDisabled(true)
	selected := ""
	for _, rule := range snapshot.rules {
		item := menu.Add(rule.status, nil)
		item.SetDisabled(true)
		items.rules[rule.name] = item
		if rule.localProxy && rule.name == snapshot.selected {
			selected = rule.name
		}
	}
	menu.AddSeparator()
	a.ItemDaemon(menu)
	menu.AddSeparator()

	proxies, exports := systray.NewMenu(), systray.NewMenu()
	items.manual = proxies.AddCheckbox(i18n.ManualProxy(), selected == "", func() {
		a.do(func() { a.selectSystemProxy("") })
	})
	for _, rule := range snapshot.rules {
		if !rule.localProxy {
			continue
		}
		items.proxies[rule.name] = proxies.AddCheckbox(rule.label, selected == rule.name, func() {
			a.do(func() { a.selectSystemProxy(rule.name) })
		})
		items.exports[rule.name] = a.addExportCommands(exports, rule)
	}
	if len(items.exports) == 0 {
		items.noLocalRule = exports.Add(i18n.NoLocalRule(), nil)
		items.noLocalRule.SetDisabled(true)
	}
	menu.AddSubmenu(i18n.SystemProxy(), proxies)
	menu.AddSubmenu(i18n.ExportCommand(), exports)
	menu.AddSeparator()

	configuration := systray.NewMenu()
	a.ItemEditConfig(configuration)
	a.ItemReloadConfig(configuration)
	a.ItemView(configuration)
	menu.AddSubmenu(i18n.Config(), configuration)
	menu.AddSeparator()
	a.ItemLog(menu)
	a.ItemAbout(menu)
	a.ItemQuit(menu)
	a.mu.Lock()
	a.menuItems = items
	a.mu.Unlock()
	return menu
}

func (a *App) rebuildMenu() error {
	if a.tray == nil {
		return nil
	}
	a.restoreSystemProxy()
	a.mu.Lock()
	previous := a.menuItems
	a.mu.Unlock()
	menu := a.buildMenu()
	err := runOnMain(func() { a.tray.SetMenu(menu) })
	if err == nil {
		return nil
	}
	err = fmt.Errorf("rebuild tray menu: %w", err)
	log.Error(err, i18n.ReloadConfig())
	a.mu.Lock()
	// The installed menu stays; keep routing status updates to its items.
	a.menuItems = previous
	a.lastErr = errors.Join(a.lastErr, err)
	a.mu.Unlock()
	return err
}
