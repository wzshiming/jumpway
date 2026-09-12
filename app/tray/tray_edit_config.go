package tray

import (
	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func (a *App) ItemEditConfig(menu *systray.Menu) {
	menu.Add(i18n.EditConfig(), func() {
		a.do(func() {
			err := config.EditConfig()
			if err != nil {
				log.Error(err, i18n.EditConfig())
			}
		})
	})
}
