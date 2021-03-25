package app

import (
	"github.com/getlantern/sysproxy"
	"github.com/getlantern/systray"
	"github.com/wzshiming/logger"
)

func (a *App) ItemSystemProxy(menu *systray.MenuItem) {
	checked := menu.Checked()
	var cancel func() error

	check := func(checked bool) {
		if checked {
			err := sysproxy.EnsureHelperToolPresent("sysproxy-cmd", "Input your password and save the world!", "")
			if err != nil {
				logger.Log.Error(err, "EnsureHelperToolPresent")
				return
			}
			cancel, err = sysproxy.On(a.ProxyAddress)
			if err != nil {
				logger.Log.Error(err, "sysproxy.On")
				return
			}
		} else {
			if cancel != nil {
				err := cancel()
				if err != nil {
					logger.Log.Error(err, "sysproxy.Off")
				}
				cancel = nil
			}
		}
	}
	check(checked)
	for range menu.ClickedCh {
		if checked {
			menu.Uncheck()
		} else {
			menu.Check()
		}
		check(!checked)
		checked = !checked
		logger.Log.Info("System Proxy", "Check", checked)
	}
}
