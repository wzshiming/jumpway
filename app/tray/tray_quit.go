package tray

import (
	"fyne.io/systray"
)

func (a *App) ItemQuit(menu *systray.MenuItem) {
	<-menu.ClickedCh
	a.Quit()
}
