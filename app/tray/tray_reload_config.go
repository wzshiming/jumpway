package tray

import (
	"context"
	"errors"
	"net"
	"slices"

	"github.com/gogpu/systray"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/hostmatcher"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/jumpway/utils"
)

func (a *App) ItemReloadConfig(menu *systray.Menu) {
	menu.Add(i18n.ReloadConfig(), func() {
		a.do(func() { _ = a.reload() })
	})

	a.do(func() { _ = a.reload() })
}

func (a *App) reload() error {
	log.Info(i18n.ReloadConfig())
	if a.cancel != nil {
		a.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	conf, err := a.store.Load()
	if err != nil {
		log.Error(err, i18n.ReloadConfig())
		a.mu.Lock()
		a.running, a.lastErr = false, err
		a.mu.Unlock()
		return err
	}
	index := slices.IndexFunc(conf.Rules, func(rule config.Rule) bool {
		return !rule.Disabled && !rule.Listen.Remote()
	})
	if index == -1 {
		err := errors.New("no enabled local rule")
		log.Error(err, i18n.ReloadConfig())
		a.mu.Lock()
		a.running, a.lastErr = false, err
		a.mu.Unlock()
		return err
	}
	rule := conf.Rules[index]

	if a.listener != nil {
		a.listener.Close()
	}

	address := rule.Listen.Address()
	listener, err := local.LOCAL.Listen(ctx, "tcp", address)
	a.listener = listener
	if err != nil {
		log.Error(err, i18n.Listen(address))
		a.mu.Lock()
		a.running, a.lastErr = false, err
		a.mu.Unlock()
		return err
	}

	a.mu.Lock()
	a.Address = formatAddress(listener.Addr().String())
	a.running, a.lastErr = true, nil
	a.mu.Unlock()
	a.UpdateStatus()
	go func() {
		dialer := jumpway.NewLogDialer(local.LOCAL, func(ctx context.Context, network, address string) {
			log.Info(i18n.UseProxy(), "address", address)
		})
		dialer, err := chain.Default.BridgeChainWithConfig(ctx, dialer, rule.Way...)
		if err != nil {
			log.Error(err, i18n.Connect(), "address", address)
			if ctx.Err() == nil {
				a.mu.Lock()
				a.running, a.lastErr = false, err
				a.mu.Unlock()
			}
			return
		}
		dialer = jumpway.NewRetryDialer(dialer, jumpway.DefaultDialRetries, jumpway.DefaultDialBackoff, func(ctx context.Context, network, address string, attempt int, err error) {
			log.Info(i18n.Connect(), "proxy", true, "address", address, "attempt", attempt, "err", err)
		})
		dialer = jumpway.NewLogDialer(dialer, func(ctx context.Context, network, address string) {
			log.Info(i18n.Connect(), "proxy", true, "address", address)
		})

		if noProxy := conf.NoProxy.GetList(a.store.Dir()); len(noProxy) != 0 {
			matcher := hostmatcher.NewMatcher(noProxy)
			subDialer := jumpway.NewLogDialer(local.LOCAL, func(ctx context.Context, network, address string) {
				log.Info(i18n.Connect(), "proxy", false, "address", address)
			})
			dialer = chain.NewShuntDialer(dialer, subDialer, matcher)
		}

		err = jumpway.RunProxy(ctx, listener, dialer, rule.Listen.User())
		if err != nil && !utils.IsClosedConnError(err) {
			log.Error(err, i18n.RunProxy())
			if ctx.Err() == nil {
				a.mu.Lock()
				a.running, a.lastErr = false, err
				a.mu.Unlock()
			}
		}
	}()
	return nil
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
