// +build !windows

package tray

import (
	"github.com/getlantern/systray"
)

func (a *App) itemExportCommand(menu *systray.MenuItem) {
	a.itemExportCommandShell(menu)
}
