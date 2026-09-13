package tray

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/app/web/runtime"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/notify"
)

type App struct {
	Address      string
	Mode         string
	Log          string
	UpdateStatus func()

	tray     *systray.SystemTray
	actions  chan func()
	cancel   context.CancelFunc
	listener net.Listener

	mu      sync.Mutex
	running bool
	lastErr error
}

var _ runtime.Runtime = (*App)(nil)

func NewApp() *App {
	a := &App{
		actions: make(chan func()),
	}
	notify.On(os.Interrupt, a.Quit)
	return a
}

func (a *App) Run() {
	logdir := filepath.Join(config.GetConfigDir(), "logs")
	err := os.MkdirAll(logdir, 0755)
	if err != nil {
		log.Error(err, i18n.RedirectLog())
		return
	}
	logfile := filepath.Join(logdir, time.Now().Format("2006_01_02_15_04_05")+".log")
	a.Log = logfile
	err = log.Redirect(logfile)
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

	runtime.Set(a)
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

// doSync runs fn on the actions worker and waits; never call it from the worker itself.
func (a *App) doSync(fn func() error) error {
	result := make(chan error, 1)
	a.actions <- func() {
		result <- fn()
	}
	return <-result
}

func (a *App) Reload() error {
	return a.doSync(a.reload)
}

func (a *App) Status() runtime.Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	status := runtime.Status{
		Address: a.Address,
		Running: a.running,
	}
	if a.lastErr != nil {
		status.Error = a.lastErr.Error()
	}
	return status
}

func (a *App) Quit() {
	if a.tray != nil {
		a.tray.Remove()
	}
}
