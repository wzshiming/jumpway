package app

import (
	"fmt"

	"github.com/getlantern/systray"
)

func (a *App) ItemStatus(menu *systray.MenuItem) {
	menu.Disable()
	a.UpdateStatus = func() {
		status := fmt.Sprintf("JumpWay %s Mode On %s:%d", a.Mode, a.Host, a.Port)
		menu.SetTitle(status)
	}
}
