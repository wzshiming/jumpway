package tray

import (
	"github.com/wzshiming/systray"
)

func (a *App) ItemQuit(menu *systray.MenuItem) {
	<-menu.ClickedCh
	a.Quit()
}
