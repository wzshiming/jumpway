package tray

import (
	"github.com/wzshiming/jumpway/daemon"
	"github.com/wzshiming/systray"
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
