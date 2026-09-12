package tray

import (
	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/daemon"
	"github.com/wzshiming/jumpway/i18n"
)

func (a *App) ItemDaemon(menu *systray.Menu) {
	var item *systray.MenuItem
	item = menu.AddCheckbox(i18n.Daemon(), daemon.DaemonIsRunning(), func() {
		a.do(func() {
			if daemon.DaemonIsRunning() {
				daemon.Remove()
			} else {
				daemon.Install()
			}
			item.SetChecked(daemon.DaemonIsRunning())
		})
	})
}
