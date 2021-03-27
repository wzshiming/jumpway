package app

import (
	"fmt"

	"github.com/getlantern/sysproxy"
	"github.com/getlantern/systray"
	"github.com/wzshiming/logger"
)

func (a *App) ItemProxyMode(global, manual *systray.MenuItem) {
	var checked proxyMode
	var cancel func() error

	check := func(checked proxyMode) {
		if checked == globalMode {
			global.Check()
			manual.Uncheck()
			a.Mode = "Global"
			a.UpdateStatus()
			err := sysproxy.EnsureHelperToolPresent("sysproxy-cmd", "Input your password and save the world!", "")
			if err != nil {
				logger.Log.Error(err, "EnsureHelperToolPresent")
				return
			}
			cancel, err = sysproxy.On(fmt.Sprintf("127.0.0.1:%d", a.Port))
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
					logger.Log.Error(err, "sysproxy.Off")
				}
				cancel = nil
			}
		}
	}
	check(checked)
	for {
		select {
		case <-global.ClickedCh:
			checked = globalMode
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
	globalMode
)
