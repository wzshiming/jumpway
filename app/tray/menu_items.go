package tray

import (
	"github.com/gogpu/systray"
	"github.com/pkg/browser"
	"github.com/wzshiming/jumpway/daemon"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func (a *App) itemDaemon(menu *systray.Menu) {
	var item *systray.MenuItem
	item = menu.AddCheckbox(i18n.Daemon(), daemon.IsInstalled(), func() {
		a.do(func() {
			if daemon.IsInstalled() {
				daemon.Remove()
			} else {
				daemon.Install()
			}
			item.SetChecked(daemon.IsInstalled())
		})
	})
}

func (a *App) itemEditConfig(menu *systray.Menu) {
	menu.Add(i18n.EditConfig(), func() {
		a.do(func() {
			err := a.store.Edit()
			if err != nil {
				log.Error(err, i18n.EditConfig())
			}
		})
	})
}

func (a *App) itemReloadConfig(menu *systray.Menu) {
	menu.Add(i18n.ReloadConfig(), func() {
		a.do(func() { _ = a.reload() })
	})
}

func (a *App) itemView(menu *systray.Menu) {
	menu.Add(i18n.WebUI(), func() {
		a.do(func() {
			err := browser.OpenURL(a.webURL())
			if err != nil {
				log.Error(err, i18n.WebUI())
			}
		})
	})
}

func (a *App) itemLog(menu *systray.Menu) {
	menu.Add(i18n.Log(), func() {
		a.do(func() {
			err := browser.OpenFile(a.logPath)
			if err != nil {
				log.Error(err, i18n.Log())
			}
		})
	})
}

func (a *App) itemAbout(menu *systray.Menu) {
	menu.Add(i18n.About(), func() {
		a.do(func() {
			err := browser.OpenURL("https://github.com/wzshiming/jumpway")
			if err != nil {
				log.Error(err, i18n.About())
			}
		})
	})
}

func (a *App) itemQuit(menu *systray.Menu) {
	menu.Add(i18n.Quit(), a.Quit)
}
