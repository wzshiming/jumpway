package tray

import (
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/systray"
)

func (a *App) ItemEditConfig(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		err := config.EditConfig()
		if err != nil {
			log.Error(err, i18n.EditConfig())
		}
	}
}
