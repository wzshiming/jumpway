package tray

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/gogpu/systray"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/hostmatcher"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/jumpway/metrics"
	"github.com/wzshiming/jumpway/utils"
)

func (a *App) ItemReloadConfig(menu *systray.Menu) {
	menu.Add(i18n.ReloadConfig(), func() {
		a.do(func() { _ = a.reload() })
	})
}

func (a *App) reload() error {
	log.Info(i18n.ReloadConfig())
	conf, err := a.store.Load()
	if err == nil {
		err = config.Validate(conf)
	}
	if err != nil {
		log.Error(err, i18n.ReloadConfig())
		a.mu.Lock()
		a.lastErr = err
		a.mu.Unlock()
		a.rebuildMenu()
		a.updateStatus()
		return err
	}
	a.mu.Lock()
	if a.root == nil {
		a.root, a.cancel = context.WithCancel(context.Background())
		a.runtimes = make(map[string]*ruleRuntime)
	}
	a.lastErr = nil
	a.mu.Unlock()

	a.listenWebUI(conf.WebUI.String())
	noProxy := conf.NoProxy.GetList(a.store.Dir())
	matcher := hostmatcher.NewMatcher(noProxy)
	enabled := make([]config.Rule, 0, len(conf.Rules))
	fingerprints := make(map[string]string, len(conf.Rules))
	noProxyJSON, _ := json.Marshal(noProxy)
	for _, rule := range conf.Rules {
		if rule.Disabled {
			continue
		}
		enabled = append(enabled, rule)
		ruleJSON, _ := json.Marshal(rule)
		fingerprint := string(ruleJSON)
		if rule.Forward.IsProxy() {
			fingerprint += string(noProxyJSON)
		}
		fingerprints[rule.Name] = fingerprint
	}
	var stopped []*ruleRuntime
	for name, runtime := range a.runtimes {
		a.mu.Lock()
		running := runtime.state.running
		a.mu.Unlock()
		if fingerprint, ok := fingerprints[name]; ok && fingerprint == runtime.fingerprint && running {
			continue
		}
		runtime.cancel()
		stopped = append(stopped, runtime)
		delete(a.runtimes, name)
	}
	for _, runtime := range stopped {
		<-runtime.done
	}
	rules := make([]*ruleState, 0, len(enabled))
	for _, rule := range enabled {
		if runtime, ok := a.runtimes[rule.Name]; ok {
			rules = append(rules, runtime.state)
			continue
		}
		protocols := rule.Listen.Schemes()
		rules = append(rules, &ruleState{
			name:          rule.Name,
			listenAddress: rule.Listen.Address(),
			address:       rule.Listen.Address(),
			target:        rule.Forward.Target(),
			remote:        rule.Listen.Remote(),
			virtual:       rule.Listen.Virtual != "",
			http: rule.Forward.IsProxy() && slices.ContainsFunc(protocols, func(protocol config.Protocol) bool {
				return protocol.Type == config.ProtocolHTTP
			}),
		})
	}
	a.metrics.Sync(enabled)
	a.mu.Lock()
	a.rules = rules
	a.mu.Unlock()

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	firsts := make([]chan error, len(enabled))
	for index, rule := range enabled {
		if _, ok := a.runtimes[rule.Name]; ok {
			continue
		}
		ctx, cancel := context.WithCancel(a.root)
		rs := a.metrics.Rule(rule.Name)
		target := rule.Forward.Target()
		state := rules[index]
		done := make(chan struct{})
		a.runtimes[rule.Name] = &ruleRuntime{
			fingerprint: fingerprints[rule.Name],
			state:       state,
			cancel:      cancel,
			done:        done,
		}
		first := make(chan error, 1)
		firsts[index] = first
		var once sync.Once
		report := func(event jumpway.Event) {
			if ctx.Err() != nil && utils.IsClosedConnError(event.Err) {
				return
			}
			a.mu.Lock()
			if event.Addr != nil {
				state.address = event.Addr.String()
				if !state.remote {
					state.address = formatAddress(state.address)
				}
				state.running, state.attempt, state.err = true, 0, nil
			} else {
				state.running, state.attempt, state.err = false, event.Attempt, event.Err
			}
			address := state.address
			a.mu.Unlock()
			if event.Addr != nil {
				if target == "" {
					log.Info(i18n.Listen(address), "rule", rule.Name)
				} else {
					log.Info(i18n.Listen(address), "rule", rule.Name, "target", target)
				}
			} else if event.Attempt <= 1 {
				log.Error(event.Err, i18n.Listen(address), "rule", rule.Name, "attempt", event.Attempt, "backoff", event.Backoff)
			} else {
				// Only the first failure of an outage alerts; retries stay in the log.
				log.Warn(i18n.Listen(address), "err", event.Err, "rule", rule.Name, "attempt", event.Attempt, "backoff", event.Backoff)
			}
			a.updateStatus()
			once.Do(func() { first <- event.Err })
		}
		listenConfig, err := jumpway.NewListenConfig(ctx, rule.Listen.Way, rs.HopWrapper(metrics.Listen))
		if err != nil {
			report(jumpway.Event{Err: err})
			cancel()
			close(done)
			continue
		}
		dialer := jumpway.NewLogDialer(local.LOCAL, func(ctx context.Context, network, address string) {
			log.Info(i18n.UseProxy(), "address", address, "rule", rule.Name)
		})
		if rule.Forward.Virtual != "" {
			dialer = virtualDialer{network: &a.virtual, name: rule.Forward.Virtual}
		}
		dialer, err = jumpway.NewChainDialer(ctx, dialer, rule.Forward.Way, rs.HopWrapper(metrics.Forward))
		if err != nil {
			report(jumpway.Event{Err: err})
			cancel()
			close(done)
			continue
		}
		if target == "" && len(rule.Forward.Way) > 0 {
			dialer = chain.NewEnvDialer(dialer)
		}
		dialer = jumpway.NewRetryDialer(dialer, jumpway.DefaultDialRetries, jumpway.DefaultDialBackoff, func(ctx context.Context, network, address string, attempt int, err error) {
			log.Info(i18n.Connect(), "proxy", true, "address", address, "rule", rule.Name, "attempt", attempt, "err", err)
		})
		dialer = jumpway.NewLogDialer(dialer, func(ctx context.Context, network, address string) {
			log.Info(i18n.Connect(), "proxy", true, "address", address, "rule", rule.Name)
		})

		// Fixed targets must stay on their configured chain, including exit-node loopback addresses.
		if target == "" && len(noProxy) != 0 {
			subDialer := jumpway.NewLogDialer(local.LOCAL, func(ctx context.Context, network, address string) {
				log.Info(i18n.Connect(), "proxy", false, "address", address, "rule", rule.Name)
			})
			dialer = chain.NewShuntDialer(dialer, subDialer, matcher)
		}
		dialer = rs.WrapDialer(dialer)

		listen := func(ctx context.Context) (net.Listener, error) {
			return listenConfig.Listen(ctx, "tcp", rule.Listen.Address())
		}
		if rule.Listen.Virtual != "" {
			listen = func(context.Context) (net.Listener, error) {
				return a.virtual.Listen(rule.Listen.Virtual)
			}
		}
		protocols := rule.Listen.Schemes()
		schemes := make([]jumpway.Scheme, len(protocols))
		for index, protocol := range protocols {
			schemes[index] = jumpway.Scheme{Type: protocol.Type, User: protocol.User()}
		}
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			defer close(done)
			jumpway.Serve(ctx, listen, func(ctx context.Context, listener net.Listener) error {
				listener = rs.WrapListener(listener)
				if target == "" {
					return jumpway.RunProxy(ctx, listener, dialer, schemes)
				}
				return jumpway.RunForward(ctx, listener, dialer, target)
			}, report)
		}()
	}

	var failures []error
	timedOut := false
	for index, first := range firsts {
		if first == nil {
			continue
		}
		var firstErr error
		if !timedOut {
			select {
			case firstErr = <-first:
			case <-timer.C:
				timedOut = true
			}
		}
		if timedOut {
			select {
			case firstErr = <-first:
			default:
			}
		}
		if firstErr != nil {
			failures = append(failures, fmt.Errorf("rule %q: %w", enabled[index].Name, firstErr))
		}
	}
	a.mu.Lock()
	webErr := a.webErr
	a.mu.Unlock()
	if webErr != nil {
		failures = append(failures, fmt.Errorf("web_ui: %w", webErr))
	}
	a.rebuildMenu()
	a.updateStatus()
	return errors.Join(failures...)
}

func (a *App) listenWebUI(address string) {
	a.mu.Lock()
	if a.webListener != nil && a.webListenAddress == address {
		a.mu.Unlock()
		return
	}
	previousListener, previousServer := a.webListener, a.webServer
	a.webListener, a.webServer = nil, nil
	a.webListenAddress, a.webAddress = address, formatAddress(address)
	a.webErr = nil
	a.mu.Unlock()
	if previousListener != nil {
		previousListener.Close()
	}
	if previousServer != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			previousServer.Shutdown(ctx)
			previousServer.Close()
		}()
	}
	listener, err := local.LOCAL.Listen(context.Background(), "tcp", address)
	if err != nil {
		log.Error(err, i18n.Listen(address), "web_ui", true)
		a.mu.Lock()
		a.webErr = err
		a.mu.Unlock()
		return
	}
	server := &http.Server{Handler: a.web}
	a.mu.Lock()
	a.webListener, a.webServer = listener, server
	a.webAddress, a.webErr = formatAddress(listener.Addr().String()), nil
	boundAddress := a.webAddress
	a.mu.Unlock()
	log.Info(i18n.Listen(boundAddress), "web_ui", true)
	go func() {
		err := server.Serve(listener)
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return
		}
		a.mu.Lock()
		current := a.webServer == server
		if current {
			a.webListener, a.webErr = nil, err
		}
		a.mu.Unlock()
		if current {
			log.Error(err, i18n.Listen(boundAddress), "web_ui", true)
			a.updateStatus()
		}
	}()
}

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
