package tray

import (
	"github.com/gogpu/systray"
	"github.com/pkg/browser"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func (a *App) ItemAbout(menu *systray.Menu) {
	menu.Add(i18n.About(), func() {
		a.do(func() {
			err := browser.OpenURL("https://github.com/wzshiming/jumpway")
			if err != nil {
				log.Error(err, i18n.About())
			}
		})
	})
}
