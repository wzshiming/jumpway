package tray

import (
	"github.com/getlantern/systray"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/logger"
)

func (a *App) ItemEditConfig(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		err := config.EditConfig()
		if err != nil {
			logger.Log.Error(err, "EditConfig")
		}
	}
}
