package app

import (
	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"
	"github.com/wzshiming/logger"
)

func (a *App) ItemAbout(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		err := open.Start("https://github.com/wzshiming/jumpway")
		if err != nil {
			logger.Log.Error(err, "About")
		}
	}
}
