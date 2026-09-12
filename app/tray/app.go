package tray

import (
	"os"
	"path/filepath"
	"time"

	"github.com/gogpu/systray"
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

	tray    *systray.SystemTray
	actions chan func()
}

func NewApp() *App {
	a := &App{
		actions: make(chan func()),
	}
	notify.On(os.Interrupt, a.Quit)
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

	go func() {
		for fn := range a.actions {
			fn()
		}
	}()

	a.tray = systray.New()
	a.onReady()
	err = a.tray.Run()
	if err != nil {
		log.Error(err, "Unable to run systray")
	}
	a.onExit()
}

// do runs fn on a dedicated worker so menu callbacks neither block the UI
// thread nor overlap with each other.
func (a *App) do(fn func()) {
	go func() {
		a.actions <- fn
	}()
}

func (a *App) Quit() {
	if a.tray != nil {
		a.tray.Remove()
	}
}
