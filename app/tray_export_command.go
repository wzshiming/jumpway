package app

import (
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/getlantern/systray"
	"github.com/wzshiming/logger"
)

func (a *App) ItemExportCommand(menu *systray.MenuItem) {
	a.itemExportCommand(menu)
}

func (a *App) itemExportCommandShell(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		command := fmt.Sprintf("export http_proxy=http://%s:%d https_proxy=http://%s:%d; ", a.Host, a.Port, a.Host, a.Port)
		err := clipboard.WriteAll(command)
		if err != nil {
			logger.Log.Error(err, "write clipboard")
		}
	}
}
