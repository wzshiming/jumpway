package tray

import (
	"github.com/wzshiming/jumpway/daemon"
)

func (a *App) onExit() {
	a.stop()
	daemon.Stop()
}
