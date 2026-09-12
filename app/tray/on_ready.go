package tray

import (
	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/icon"
	"github.com/wzshiming/systheme"
)

func (a *App) onReady() {
	ico := icon.Black
	switch t, _ := systheme.GetTheme(); t {
	case systheme.Light:
		ico = icon.Black
	case systheme.Dark:
		ico = icon.White
	case systheme.Unknown:
		ico = icon.Gray
	}

	menu := systray.NewMenu()

	a.ItemStatus(menu)

	menu.AddSeparator()

	a.ItemDaemon(menu)

	menu.AddSeparator()

	a.ItemProxyMode(menu)
	a.ItemExportCommand(menu)

	menu.AddSeparator()

	mConfig := systray.NewMenu()
	{
		a.ItemEditConfig(mConfig)
		a.ItemReloadConfig(mConfig)
		a.ItemView(mConfig)
	}
	menu.AddSubmenu(i18n.Config(), mConfig)

	menu.AddSeparator()

	a.ItemLog(menu)
	a.ItemAbout(menu)
	a.ItemQuit(menu)

	a.tray.
		SetIcon(ico).
		SetTemplateIcon(ico).
		SetTooltip("Jump Way").
		SetMenu(menu).
		Show()
}
