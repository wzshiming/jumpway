package tray

import (
	"os"

	"github.com/wzshiming/notify"
	"github.com/wzshiming/systray"
)

type App struct {
	Address      string
	RawHost      string
	Mode         string
	Log          string
	UpdateStatus func()
}

func NewApp() *App {
	a := &App{}
	notify.On(os.Interrupt, a.Quit)
	return a
}

func (a *App) Run() {
	systray.Run(a.onReady, a.onExit)
}

func (App) Quit() {
	systray.Quit()
}
