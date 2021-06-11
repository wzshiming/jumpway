package tray

import (
	"github.com/getlantern/sysproxy"
	"github.com/getlantern/systray"
	"github.com/wzshiming/logger"
)

func (a *App) ItemProxyMode(global, manual *systray.MenuItem) {
	var checked proxyMode
	var cancel func() error

	check := func(checked proxyMode) {
		if checked == systemMode {
			global.Check()
			manual.Uncheck()
			a.Mode = "System"
			a.UpdateStatus()
			err := sysproxy.EnsureHelperToolPresent("sysproxy-cmd", "Input your password and save the world!", "")
			if err != nil {
				logger.Log.Error(err, "EnsureHelperToolPresent")
				return
			}
			cancel, err = sysproxy.On(a.Address)
			if err != nil {
				logger.Log.Error(err, "sysproxy.On")
				return
			}
		} else {
			manual.Check()
			global.Uncheck()
			a.Mode = "Manual"
			a.UpdateStatus()
			if cancel != nil {
				err := cancel()
				if err != nil {
					logger.Log.Error(err, "sysproxy.On cancel")
				}
				cancel = nil
			}
			err := sysproxy.Off(a.Address)
			if err != nil {
				logger.Log.Error(err, "sysproxy.Off")
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
		logger.Log.Info("System Proxy", "Check", checked)
	}
}

type proxyMode uint

const (
	manualMode proxyMode = iota
	systemMode
)
