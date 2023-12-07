package tray

import (
	"fmt"

	"fyne.io/systray"
	"github.com/pkg/browser"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func (a *App) ItemView(menu *systray.MenuItem) {
	var err error
	for range menu.ClickedCh {
		err = browser.OpenURL(fmt.Sprintf("http://%s", a.Address))
		if err != nil {
			log.Error(err, i18n.ViewEditConfig())
		}
	}
}
