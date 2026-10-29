package tray

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/hostmatcher"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/jumpway/metrics"
)

// firstEventTimeout bounds how long reload waits for new rules to report their first listen result.
const firstEventTimeout = 2 * time.Second

// exit is the shared outbound policy of one reload: the no-proxy list and its matcher.
type exit struct {
	noProxy []string
	matcher hostmatcher.Matcher
}

// reload applies the stored config: it restarts the rules whose definition changed, leaves the
// others running, and reports the first listen error of every started rule. Only the actions
// worker calls it, so a.runtimes needs no lock.
func (a *App) reload() error {
	log.Info(i18n.ReloadConfig())
	conf, err := a.store.Load()
	if err == nil {
		err = config.Validate(conf)
	}
	if err != nil {
		return a.failReload(err)
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
	policy := exit{noProxy: noProxy, matcher: hostmatcher.NewMatcher(noProxy)}
	enabled, fingerprints := enabledRules(conf.Rules, noProxy)
	a.stopStaleRules(fingerprints)
	rules := a.syncRuleStates(enabled)

	timer := time.NewTimer(firstEventTimeout)
	defer timer.Stop()
	firsts := make([]<-chan error, len(enabled))
	for index, rule := range enabled {
		if _, ok := a.runtimes[rule.Name]; ok {
			continue
		}
		firsts[index] = a.startRule(rule, rules[index], fingerprints[rule.Name], policy)
	}
	failures := collectFirstErrors(enabled, firsts, timer.C)

	a.mu.Lock()
	webErr := a.webErr
	a.mu.Unlock()
	if webErr != nil {
		failures = append(failures, fmt.Errorf("web_ui: %w", webErr))
	}
	if err := a.rebuildMenu(); err != nil {
		failures = append(failures, err)
	}
	a.updateStatus()
	return errors.Join(failures...)
}

// failReload records a config error and refreshes the menu while the previous rules keep running.
func (a *App) failReload(err error) error {
	log.Error(err, i18n.ReloadConfig())
	a.mu.Lock()
	a.lastErr = err
	a.mu.Unlock()
	if menuErr := a.rebuildMenu(); menuErr != nil {
		err = errors.Join(err, menuErr)
	}
	a.updateStatus()
	return err
}

// enabledRules drops disabled rules and fingerprints the rest; proxy rules fold in the no-proxy
// list so that changing it restarts them too.
func enabledRules(rules []config.Rule, noProxy []string) ([]config.Rule, map[string]string) {
	enabled := make([]config.Rule, 0, len(rules))
	fingerprints := make(map[string]string, len(rules))
	noProxyJSON, _ := json.Marshal(noProxy)
	for _, rule := range rules {
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
	return enabled, fingerprints
}

// stopStaleRules cancels every runtime whose rule vanished, changed or is not running, and waits for it to exit.
func (a *App) stopStaleRules(fingerprints map[string]string) {
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
}

// syncRuleStates publishes the state of every enabled rule, reusing the state of runtimes that survived.
func (a *App) syncRuleStates(enabled []config.Rule) []*ruleState {
	rules := make([]*ruleState, 0, len(enabled))
	for _, rule := range enabled {
		if runtime, ok := a.runtimes[rule.Name]; ok {
			rules = append(rules, runtime.state)
			continue
		}
		rules = append(rules, newRuleState(rule))
	}
	a.metrics.Sync(enabled)
	a.mu.Lock()
	a.rules = rules
	a.mu.Unlock()
	return rules
}

func newRuleState(rule config.Rule) *ruleState {
	return &ruleState{
		name:          rule.Name,
		listenAddress: rule.Listen.Address(),
		address:       rule.Listen.Address(),
		target:        rule.Forward.Target(),
		remote:        rule.Listen.Remote(),
		virtual:       rule.Listen.Virtual != "",
		http: rule.Forward.IsProxy() && slices.ContainsFunc(rule.Listen.Schemes(), func(protocol config.Protocol) bool {
			return protocol.Type == config.ProtocolHTTP
		}),
	}
}

// startRule registers the runtime of rule and serves it until the root context is cancelled.
// The returned channel yields the rule's first listen result; setup errors are delivered there too.
func (a *App) startRule(rule config.Rule, state *ruleState, fingerprint string, policy exit) <-chan error {
	ctx, cancel := context.WithCancel(a.root)
	done := make(chan struct{})
	a.runtimes[rule.Name] = &ruleRuntime{
		fingerprint: fingerprint,
		state:       state,
		cancel:      cancel,
		done:        done,
	}
	first := make(chan error, 1)
	report := a.ruleReporter(ctx, rule, state, first)
	listen, serve, err := a.buildRule(ctx, rule, policy)
	if err != nil {
		report(jumpway.Event{Err: err})
		cancel()
		close(done)
		return first
	}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer close(done)
		jumpway.Serve(ctx, listen, serve, report)
	}()
	return first
}

// ruleReporter records each listen event on state, logs it, and delivers the first one to first.
func (a *App) ruleReporter(ctx context.Context, rule config.Rule, state *ruleState, first chan<- error) func(jumpway.Event) {
	target := rule.Forward.Target()
	var once sync.Once
	return func(event jumpway.Event) {
		if ctx.Err() != nil && jumpway.IsClosedConnError(event.Err) {
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
}

// buildRule assembles the entry listener and the connection handler of one rule.
func (a *App) buildRule(ctx context.Context, rule config.Rule, policy exit) (listen func(context.Context) (net.Listener, error), serve func(context.Context, net.Listener) error, err error) {
	rs := a.metrics.Rule(rule.Name)
	listenConfig, err := jumpway.NewListenConfig(ctx, rule.Listen.Way, rs.HopWrapper(metrics.Listen))
	if err != nil {
		return nil, nil, err
	}
	dialer, err := a.buildDialer(ctx, rule, rs, policy)
	if err != nil {
		return nil, nil, err
	}
	listen = func(ctx context.Context) (net.Listener, error) {
		return listenConfig.Listen(ctx, "tcp", rule.Listen.Address())
	}
	if rule.Listen.Virtual != "" {
		listen = func(context.Context) (net.Listener, error) {
			return a.virtual.Listen(rule.Listen.Virtual)
		}
	}
	target := rule.Forward.Target()
	schemes := ruleSchemes(rule.Listen.Schemes())
	serve = func(ctx context.Context, listener net.Listener) error {
		listener = rs.WrapListener(listener)
		if target == "" {
			return jumpway.RunProxy(ctx, listener, dialer, schemes)
		}
		return jumpway.RunForward(ctx, listener, dialer, target)
	}
	return listen, serve, nil
}

// buildDialer is the exit side of a rule: the forward chain with retries, logging, no-proxy shunting and metrics.
func (a *App) buildDialer(ctx context.Context, rule config.Rule, rs *metrics.Rule, policy exit) (bridge.Dialer, error) {
	target := rule.Forward.Target()
	dialer := jumpway.NewLogDialer(local.LOCAL, func(ctx context.Context, network, address string) {
		log.Info(i18n.UseProxy(), "address", address, "rule", rule.Name)
	})
	if rule.Forward.Virtual != "" {
		dialer = virtualDialer{network: &a.virtual, name: rule.Forward.Virtual}
	}
	dialer, err := jumpway.NewChainDialer(ctx, dialer, rule.Forward.Way, rs.HopWrapper(metrics.Forward))
	if err != nil {
		return nil, err
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
	if target == "" && len(policy.noProxy) != 0 {
		subDialer := jumpway.NewLogDialer(local.LOCAL, func(ctx context.Context, network, address string) {
			log.Info(i18n.Connect(), "proxy", false, "address", address, "rule", rule.Name)
		})
		dialer = chain.NewShuntDialer(dialer, subDialer, policy.matcher)
	}
	return rs.WrapDialer(dialer), nil
}

func ruleSchemes(protocols []config.Protocol) []jumpway.Scheme {
	schemes := make([]jumpway.Scheme, len(protocols))
	for index, protocol := range protocols {
		schemes[index] = jumpway.Scheme{Type: protocol.Type, User: protocol.User()}
	}
	return schemes
}

// collectFirstErrors waits for each started rule's first listen result until deadline fires,
// after which only results already delivered are taken.
func collectFirstErrors(enabled []config.Rule, firsts []<-chan error, deadline <-chan time.Time) []error {
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
			case <-deadline:
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
	return failures
}
