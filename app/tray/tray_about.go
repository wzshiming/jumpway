package tray

import (
	"github.com/pkg/browser"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/systray"
)

func (a *App) ItemAbout(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		err := browser.OpenURL("https://github.com/wzshiming/jumpway")
		if err != nil {
			log.Error(err, i18n.About())
		}
	}
}
