package tray

import (
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/systray"
)

func (a *App) ItemStatus(menu *systray.MenuItem) {
	menu.Disable()
	a.UpdateStatus = func() {
		menu.SetTitle(i18n.Status(a.Mode, a.Address))
	}
}
