package tray

import (
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/getlantern/systray"
	"github.com/wzshiming/logger"
)

func (a *App) ItemExportCommand(menu *systray.MenuItem) {
	mShell := menu.AddSubMenuItem("Shell", "")
	go a.itemExportCommandShell(mShell)

	mCmd := menu.AddSubMenuItem("Cmd", "")
	go a.itemExportCommandCmd(mCmd)

	mPowerShell := menu.AddSubMenuItem("PowerShell", "")
	go a.itemExportCommandPowerShell(mPowerShell)
}

func (a *App) itemExportCommandShell(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		command := fmt.Sprintf("export http_proxy=http://%s https_proxy=http://%s; ", a.Address, a.Address)
		err := clipboard.WriteAll(command)
		if err != nil {
			logger.Log.Error(err, "write clipboard")
		}
	}
}

func (a *App) itemExportCommandCmd(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		command := fmt.Sprintf("set http_proxy=http://%s && set https_proxy=http://%s", a.Address, a.Address)
		err := clipboard.WriteAll(command)
		if err != nil {
			logger.Log.Error(err, "write clipboard")
		}
	}
}

func (a *App) itemExportCommandPowerShell(menu *systray.MenuItem) {
	for range menu.ClickedCh {
		command := fmt.Sprintf("$env:http_proxy='http://%s'; $env:https_proxy='http://%s'; ", a.Address, a.Address)
		err := clipboard.WriteAll(command)
		if err != nil {
			logger.Log.Error(err, "write clipboard")
		}
	}
}
