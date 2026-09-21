package tray

import (
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/sysproxy"
)

func (a *App) ruleAddress(name string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return localProxyAddress(a.rules, name)
}

func localProxyAddress(rules []*ruleState, name string) string {
	for _, rule := range rules {
		if rule.name != name || !rule.localProxy() {
			continue
		}
		address := rule.address
		if address == "" {
			address = rule.listenAddress
		}
		return formatAddress(address)
	}
	return ""
}

func (a *App) syncSystemProxySelection() (address string, changed bool, removed string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	address = localProxyAddress(a.rules, a.systemProxyRule)
	if a.systemProxyRule != "" && address == "" {
		removed = a.systemProxyRule
		a.systemProxyRule = ""
	}
	changed = address != a.systemProxyAddress || removed != ""
	a.systemProxyAddress = address
	a.Mode = i18n.ManualProxy()
	if a.systemProxyRule != "" {
		a.Mode = i18n.SystemProxy()
	}
	return
}

func (a *App) selectSystemProxy(name string) {
	a.proxyMu.Lock()
	if a.stopped {
		a.proxyMu.Unlock()
		return
	}
	a.mu.Lock()
	a.systemProxyRule = name
	a.mu.Unlock()
	address, _, removed := a.syncSystemProxySelection()
	if removed != "" {
		log.Info("System proxy rule removed", "rule", removed)
	}
	setSystemProxy(address)
	// updateStatus may wait on the UI thread, which may itself be in Quit waiting for proxyMu.
	a.proxyMu.Unlock()
	log.Info(i18n.ProxyMode(), "rule", name, "address", address)
	a.updateStatus()
}

func (a *App) restoreSystemProxy() {
	a.proxyMu.Lock()
	defer a.proxyMu.Unlock()
	address, changed, removed := a.syncSystemProxySelection()
	if removed != "" {
		log.Info("System proxy rule removed", "rule", removed)
	}
	if changed {
		setSystemProxy(address)
	}
}

var setSystemProxy = func(address string) {
	if address == "" {
		if err := sysproxy.OffHTTPS(); err != nil {
			log.Error(err, "sysproxy.OffHTTPS")
		}
		if err := sysproxy.OffHTTP(); err != nil {
			log.Error(err, "sysproxy.OffHTTP")
		}
		return
	}
	if err := sysproxy.OnHTTPS(address); err != nil {
		log.Error(err, "sysproxy.OnHTTPS")
	}
	if err := sysproxy.OnHTTP(address); err != nil {
		log.Error(err, "sysproxy.OnHTTP")
	}
}
