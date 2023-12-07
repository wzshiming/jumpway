package tray

import (
	"fyne.io/systray"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/sysproxy"
)

func (a *App) ItemProxyMode(global, manual *systray.MenuItem) {
	var checked proxyMode

	check := func(checked proxyMode) {
		if checked == systemMode {
			global.Check()
			manual.Uncheck()
			a.Mode = i18n.SystemProxy()
			a.UpdateStatus()

			err := sysproxy.OnHTTPS(a.Address)
			if err != nil {
				log.Error(err, "sysproxy.OnHTTPS")
				return
			}
			err = sysproxy.OnHTTP(a.Address)
			if err != nil {
				log.Error(err, "sysproxy.OnHTTP")
				return
			}
		} else {
			manual.Check()
			global.Uncheck()
			a.Mode = i18n.ManualProxy()
			a.UpdateStatus()

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
	check(checked)
	for {
		select {
		case <-global.ClickedCh:
			checked = systemMode
		case <-manual.ClickedCh:
			checked = manualMode
		}
		check(checked)
		log.Info(i18n.ProxyMode(), "mode", checked)
	}
}

type proxyMode uint

const (
	manualMode proxyMode = iota
	systemMode
)
