package tray

import (
	"github.com/getlantern/systray"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/sysproxy"
)

func (a *App) ItemProxyMode(global, manual *systray.MenuItem) {
	var checked proxyMode

	check := func(checked proxyMode) {
		if checked == systemMode {
			global.Check()
			manual.Uncheck()
			a.Mode = "System"
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
			a.Mode = "Manual"
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
		log.Info("System Proxy", "Check", checked)
	}
}

type proxyMode uint

const (
	manualMode proxyMode = iota
	systemMode
)
