package tray

import (
	"github.com/gogpu/systray"
	"github.com/pkg/browser"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func (a *App) ItemLog(menu *systray.Menu) {
	menu.Add(i18n.Log(), func() {
		a.do(func() {
			err := browser.OpenFile(a.Log)
			if err != nil {
				log.Error(err, i18n.Log())
			}
		})
	})
}
