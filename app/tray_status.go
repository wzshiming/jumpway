package app

import (
	"fmt"

	"github.com/getlantern/systray"
)

func (a *App) ItemStatus(menu *systray.MenuItem) {
	menu.Disable()
	a.UpdateStatus = func() {
		status := fmt.Sprintf("JumpWay %s Mode On Port %d", a.Mode, a.Port)
		menu.SetTitle(status)
	}
}
