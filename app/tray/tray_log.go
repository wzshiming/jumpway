package tray

import (
	"github.com/pkg/browser"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/logger"
	"github.com/wzshiming/systray"
)

func (a *App) ItemLog(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		err := browser.OpenFile(a.Log)
		if err != nil {
			logger.Error(err, i18n.Log())
		}
	}
}
