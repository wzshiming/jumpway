package tray

import (
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/logger"
	"github.com/wzshiming/systray"
)

func (a *App) ItemEditConfig(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		err := config.EditConfig()
		if err != nil {
			logger.Error(err, i18n.EditConfig())
		}
	}
}
