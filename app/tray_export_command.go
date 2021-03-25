package app

import (
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/getlantern/systray"
	"github.com/wzshiming/logger"
)

func (a *App) ItemExportCommand(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		command := fmt.Sprintf("export http_proxy=http://%s; export https_proxy=http://%s; ", a.ProxyAddress, a.ProxyAddress)
		err := clipboard.WriteAll(command)
		if err != nil {
			logger.Log.Error(err, "write clipboard")
		}
	}
}
