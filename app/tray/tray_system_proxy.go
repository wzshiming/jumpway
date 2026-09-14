package tray

import (
	"errors"

	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/sysproxy"
)

func (a *App) ItemProxyMode(menu *systray.Menu) {
	var global, manual *systray.MenuItem

	check := func(checked proxyMode) {
		if checked == systemMode {
			address := a.primaryAddress()
			if address == "" {
				log.Error(errors.New(i18n.NoLocalRule()), i18n.SystemProxy())
				return
			}
			global.SetChecked(true)
			manual.SetChecked(false)
			a.mu.Lock()
			a.Mode = i18n.SystemProxy()
			a.mu.Unlock()
			a.updateStatus()

			err := sysproxy.OnHTTPS(address)
			if err != nil {
				log.Error(err, "sysproxy.OnHTTPS")
				return
			}
			err = sysproxy.OnHTTP(address)
			if err != nil {
				log.Error(err, "sysproxy.OnHTTP")
				return
			}
		} else {
			manual.SetChecked(true)
			global.SetChecked(false)
			a.mu.Lock()
			a.Mode = i18n.ManualProxy()
			a.mu.Unlock()
			a.updateStatus()

			err := sysproxy.OffHTTPS()
			if err != nil {
				log.Error(err, "sysproxy.OffHTTPS")
			}
			err = sysproxy.OffHTTP()
			if err != nil {
				log.Error(err, "sysproxy.OffHTTP")
			}
		}
	}

	selectMode := func(checked proxyMode) func() {
		return func() {
			a.do(func() {
				check(checked)
				log.Info(i18n.ProxyMode(), "mode", checked)
			})
		}
	}

	manual = menu.AddCheckbox(i18n.ManualProxy(), true, selectMode(manualMode))
	global = menu.AddCheckbox(i18n.SystemProxy(), false, selectMode(systemMode))

	a.do(func() {
		check(manualMode)
	})
}

type proxyMode uint

const (
	manualMode proxyMode = iota
	systemMode
)
