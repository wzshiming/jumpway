package tray

import (
	"github.com/getlantern/systray"
	"github.com/wzshiming/jumpway/daemon"
)

func (a *App) ItemDaemon(menu *systray.MenuItem) {
	check := func() {
		if daemon.DaemonIsRunning() {
			menu.Check()
		} else {
			menu.Uncheck()
		}
	}
	check()

	for range menu.ClickedCh {
		if daemon.DaemonIsRunning() {
			daemon.Remove()
		} else {
			daemon.Install()
		}
		check()
	}
}
