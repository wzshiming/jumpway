package tray

import (
	"github.com/gogpu/systray"
	"github.com/pkg/browser"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func (a *App) ItemView(menu *systray.Menu) {
	menu.Add(i18n.WebUI(), func() {
		a.do(func() {
			err := browser.OpenURL(a.webURL())
			if err != nil {
				log.Error(err, i18n.WebUI())
			}
		})
	})
}
