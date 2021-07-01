package tray

import (
	"fmt"

	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"
	"github.com/wzshiming/logger"
)

func (a *App) ItemView(menu *systray.MenuItem) {
	var err error
	for range menu.ClickedCh {
		err = open.Start(fmt.Sprintf("http://%s", a.Address))
		if err != nil {
			logger.Log.Error(err, "View")
		}
	}
}
