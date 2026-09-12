package tray

import (
	"fmt"

	"github.com/gogpu/systray"
	"github.com/pkg/browser"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func (a *App) ItemView(menu *systray.Menu) {
	menu.Add(i18n.ViewEditConfig(), func() {
		a.do(func() {
			err := browser.OpenURL(fmt.Sprintf("http://%s", a.Address))
			if err != nil {
				log.Error(err, i18n.ViewEditConfig())
			}
		})
	})
}
