package tray

import (
	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/i18n"
)

func (a *App) ItemStatus(menu *systray.Menu) {
	item := menu.Add("", nil)
	item.SetDisabled(true)
	a.mu.Lock()
	a.UpdateStatus = func() {
		address := a.primaryAddress()
		a.mu.Lock()
		mode := a.Mode
		if address == "" {
			address = a.webAddress
		}
		a.mu.Unlock()
		item.SetLabel(i18n.Status(mode, address))
	}
	a.mu.Unlock()
}
