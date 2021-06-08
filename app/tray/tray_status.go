package tray

import (
	"fmt"

	"github.com/getlantern/systray"
)

func (a *App) ItemStatus(menu *systray.MenuItem) {
	menu.Disable()
	a.UpdateStatus = func() {
		status := fmt.Sprintf("JumpWay %s Mode On %s", a.Mode, a.Address)
		menu.SetTitle(status)
	}
}
