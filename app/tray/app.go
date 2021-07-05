package tray

import (
	"os"
	"path/filepath"
	"time"

	"github.com/getlantern/systray"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/notify"
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
	notify.On(a.Quit, os.Interrupt)
	return a
}

func (a *App) Run() {
	logdir := filepath.Join(config.GetConfigDir(), "logs")
	os.MkdirAll(logdir, 0755)
	logfile := filepath.Join(logdir, time.Now().Format("2006_01_02_15_04_05")+".log")
	a.Log = logfile
	err := log.Redirect(logfile)
	if err != nil {
		log.Error(err, i18n.RedirectLog())
		return
	}
	err = config.InitConfig()
	if err != nil {
		log.Error(err, i18n.InitConfig())
		return
	}
	systray.Run(a.onReady, a.onExit)
}

func (App) Quit() {
	systray.Quit()
}
