package tray

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/app/web"
	"github.com/wzshiming/jumpway/app/web/services/configs"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/notify"
)

type App struct {
	Mode         string
	Log          string
	UpdateStatus func()

	tray    *systray.SystemTray
	actions chan func()
	store   *config.Store
	web     http.Handler

	cancel context.CancelFunc
	wg     sync.WaitGroup

	webListener net.Listener
	webServer   *http.Server

	mu               sync.Mutex
	webListenAddress string
	webAddress       string
	webErr           error
	lastErr          error
	rules            []*ruleState
}

type ruleState struct {
	name          string
	listenAddress string
	address       string
	remote        bool
	running       bool
	attempt       int
	err           error
}

var _ configs.Runtime = (*App)(nil)

func NewApp(store *config.Store) *App {
	a := &App{
		actions: make(chan func()),
		store:   store,
	}
	notify.On(os.Interrupt, a.Quit)
	return a
}

func (a *App) Run() {
	logdir := filepath.Join(a.store.Dir(), "logs")
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
	err = a.store.Init()
	if err != nil {
		log.Error(err, i18n.InitConfig())
		return
	}

	go func() {
		for fn := range a.actions {
			fn()
		}
	}()

	a.web = web.NewHandler(configs.NewConfigsService(a.store, a))
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

func (a *App) Status() configs.Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	status := configs.Status{
		Address: a.webAddress,
		Running: a.webListener != nil && a.webErr == nil,
		Rules:   make([]configs.RuleStatus, 0, len(a.rules)),
	}
	if a.lastErr != nil {
		status.Error = a.lastErr.Error()
	} else if a.webErr != nil {
		status.Error = a.webErr.Error()
	}
	for _, rule := range a.rules {
		state := configs.RuleStatus{
			Name:    rule.name,
			Address: rule.address,
			Remote:  rule.remote,
			Running: rule.running,
			Attempt: rule.attempt,
		}
		if rule.err != nil {
			state.Error = rule.err.Error()
		}
		status.Rules = append(status.Rules, state)
	}
	return status
}

func (a *App) primaryAddress() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var fallback string
	for _, rule := range a.rules {
		if rule.remote {
			continue
		}
		if rule.running {
			return rule.address
		}
		if fallback == "" {
			fallback = formatAddress(rule.listenAddress)
		}
	}
	return fallback
}

func (a *App) webURL() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return "http://" + a.webAddress
}

func (a *App) updateStatus() {
	a.mu.Lock()
	update := a.UpdateStatus
	a.mu.Unlock()
	if update != nil {
		update()
	}
}

func (a *App) stop() {
	a.mu.Lock()
	cancel, server := a.cancel, a.webServer
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if server != nil {
		server.Close()
	}
}

func (a *App) Quit() {
	a.stop()
	if a.tray != nil {
		a.tray.Remove()
	}
}
