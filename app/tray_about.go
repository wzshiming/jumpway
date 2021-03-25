package app

import (
	"github.com/getlantern/systray"
	"github.com/wzshiming/jumpway/webview"
	"github.com/wzshiming/logger"
)

func (a *App) ItemAbout(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		err := webview.View("https://github.com/wzshiming/jumpway", "JumpWay", 800, 600)
		if err != nil {
			logger.Log.Error(err, "About")
		}
	}
}
