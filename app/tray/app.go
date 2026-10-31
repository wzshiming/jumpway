package tray

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/app/web"
	"github.com/wzshiming/jumpway/app/web/services/configs"
	"github.com/wzshiming/jumpway/app/web/services/stats"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/icon"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/jumpway/metrics"
	"github.com/wzshiming/jumpway/tundev"
	"github.com/wzshiming/notify"
	"github.com/wzshiming/systheme"
)

type App struct {
	tray    *systray.SystemTray
	actions chan func()
	store   *config.Store
	web     http.Handler
	metrics *metrics.Registry
	logPath string
	openTUN func(context.Context, tundev.Config) (*tundev.Device, error)

	root          context.Context
	cancel        context.CancelFunc
	metricsCancel context.CancelFunc
	wg            sync.WaitGroup

	webListener net.Listener
	webServer   *http.Server

	proxyMu sync.Mutex
	stopped bool
	virtual virtualNetwork

	mu                 sync.Mutex
	webListenAddress   string
	webAddress         string
	webErr             error
	lastErr            error
	rules              []*ruleState
	runtimes           map[string]*ruleRuntime
	menuItems          *menuItems
	systemProxyRule    string
	systemProxyAddress string
}

type ruleRuntime struct {
	fingerprint string
	state       *ruleState
	cancel      context.CancelFunc
	done        chan struct{}
}

type ruleState struct {
	name          string
	listenAddress string
	address       string
	tun           string
	target        string
	remote        bool
	virtual       bool
	http          bool
	running       bool
	attempt       int
	err           error
}

// localProxy reports whether the rule is an HTTP proxy on this machine, usable as the system proxy.
func (r ruleState) localProxy() bool {
	return !r.remote && !r.virtual && r.target == "" && r.http
}

// displayAddress is the bound address, or the configured one before the first bind, made dialable for local rules.
func (r ruleState) displayAddress() string {
	address := r.address
	if address == "" {
		address = r.listenAddress
	}
	if !r.remote {
		address = formatAddress(address)
	}
	return address
}

var _ configs.Runtime = (*App)(nil)

func NewApp(store *config.Store) *App {
	a := &App{
		actions: make(chan func()),
		store:   store,
		metrics: metrics.NewRegistry(),
		openTUN: tundev.Open,
	}
	notify.On(syscall.SIGINT, a.Quit)
	notify.On(syscall.SIGTERM, func() { a.do(a.Quit) })
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
	a.logPath = logfile
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

	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.metricsCancel = cancel
	a.mu.Unlock()
	go a.metrics.Run(ctx)

	go func() {
		for fn := range a.actions {
			fn()
		}
	}()

	a.web = web.NewHandler(configs.NewConfigsService(a.store, a), stats.NewStatsService(a.metrics), metrics.NewHandler(a.metrics))
	a.tray = systray.New()
	a.onReady()
	err = a.tray.Run()
	if err != nil {
		log.Error(err, "Unable to run systray")
	}
	a.stop()
}

func (a *App) onReady() {
	ico := icon.Black
	switch t, _ := systheme.GetTheme(); t {
	case systheme.Light:
		ico = icon.Black
	case systheme.Dark:
		ico = icon.White
	case systheme.Unknown:
		ico = icon.Gray
	}

	a.tray.
		SetIcon(ico).
		SetTemplateIcon(ico).
		SetTooltip("Jump Way").
		SetMenu(a.buildMenu()).
		Show()
	a.do(func() {
		a.selectSystemProxy("")
		_ = a.reload()
	})
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
			Tun:     rule.tun,
			Target:  rule.target,
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
		if !rule.localProxy() {
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

// formatAddress replaces an unspecified host with loopback so the address is dialable from this machine.
func formatAddress(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	if net.ParseIP(host).IsUnspecified() {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func (a *App) stop() {
	a.proxyMu.Lock()
	a.stopped = true
	a.mu.Lock()
	cancel, server := a.cancel, a.webServer
	metricsCancel := a.metricsCancel
	systemProxyAddress := a.systemProxyAddress
	a.systemProxyRule, a.systemProxyAddress = "", ""
	a.mu.Unlock()
	if systemProxyAddress != "" {
		setSystemProxy("")
	}
	a.proxyMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if metricsCancel != nil {
		metricsCancel()
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
