package tray

import (
	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/i18n"
)

func (a *App) ItemStatus(menu *systray.Menu) {
	item := menu.Add("", nil)
	item.SetDisabled(true)
	a.UpdateStatus = func() {
		item.SetLabel(i18n.Status(a.Mode, a.Address))
	}
}
