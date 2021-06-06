package tray

import (
	"github.com/getlantern/systray"
)

func (a *App) ItemQuit(menu *systray.MenuItem) {
	<-menu.ClickedCh
	a.Quit()
}
