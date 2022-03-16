package tray

import (
	"fmt"

	"github.com/pkg/browser"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/logger"
	"github.com/wzshiming/systray"
)

func (a *App) ItemView(menu *systray.MenuItem) {
	var err error
	for range menu.ClickedCh {
		err = browser.OpenURL(fmt.Sprintf("http://%s", a.Address))
		if err != nil {
			logger.Error(err, i18n.ViewEditConfig())
		}
	}
}
