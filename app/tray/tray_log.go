package tray

import (
	"github.com/getlantern/systray"
	"github.com/skratchdot/open-golang/open"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func (a *App) ItemLog(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		err := open.Start(a.Log)
		if err != nil {
			log.Error(err, i18n.Log())
		}
	}
}
