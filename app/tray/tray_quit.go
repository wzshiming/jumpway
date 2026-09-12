package tray

import (
	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/i18n"
)

func (a *App) ItemQuit(menu *systray.Menu) {
	menu.Add(i18n.Quit(), a.Quit)
}
