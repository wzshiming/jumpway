package tray

import (
	"os"

	"github.com/getlantern/systray"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/logger"
	"github.com/wzshiming/notify"
)

type App struct {
	Port    int
	RawHost string
	Host    string
	Mode    string

	UpdateStatus func()
}

func NewApp() *App {
	a := &App{}
	notify.On(a.Quit, os.Interrupt)
	return a
}

func (a *App) Run() {
	err := log.InitLog()
	if err != nil {
		logger.Log.Error(err, "InitLog")
		return
	}
	err = config.InitConfig()
	if err != nil {
		logger.Log.Error(err, "InitConfig")
		return
	}
	systray.Run(a.onReady, a.onExit)
}

func (App) Quit() {
	systray.Quit()
}
