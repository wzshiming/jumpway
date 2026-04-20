package tray

import (
	"context"
	"net"
	"strconv"

	"fyne.io/systray"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/hostmatcher"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/jumpway/utils"
)

func (a *App) ItemReloadConfig(menu *systray.MenuItem) {
	var cancel func()
	var ctx context.Context
	var listener net.Listener

	check := func() {
		log.Info(i18n.ReloadConfig())
		if cancel != nil {
			cancel()
		}
		ctx, cancel = context.WithCancel(context.Background())
		conf, err := config.LoadConfig()
		if err != nil {
			log.Error(err, i18n.ReloadConfig())
			return
		}
		port := conf.Proxy.Port
		host := conf.Proxy.Host
		if host == "" {
			host = "127.0.0.1"
		}

		if listener != nil {
			listener.Close()
		}

		address := net.JoinHostPort(host, strconv.FormatUint(uint64(port), 10))
		listener, err = local.LOCAL.Listen(ctx, "tcp", address)
		if err != nil {
			log.Error(err, i18n.Listen(address))
			return
		}

		a.Address = formatAddress(listener.Addr().String())

		a.RawHost = host
		a.UpdateStatus()
		go func() {
			dialer := jumpway.NewLogDialer(local.LOCAL, func(ctx context.Context, network, address string) {
				log.Info(i18n.UseProxy(), "address", address)
			})
			dialer, err := chain.Default.BridgeChainWithConfig(ctx, dialer, conf.GetWay()...)
			if err != nil {
				log.Error(err, i18n.Connect(), "address", address)
				return
			}
			dialer = jumpway.NewLogDialer(dialer, func(ctx context.Context, network, address string) {
				log.Info(i18n.Connect(), "proxy", true, "address", address)
			})

			if noProxy := conf.NoProxy.GetList(); len(noProxy) != 0 {
				matcher := hostmatcher.NewMatcher(noProxy)
				subDialer := jumpway.NewLogDialer(local.LOCAL, func(ctx context.Context, network, address string) {
					log.Info(i18n.Connect(), "proxy", false, "address", address)
				})
				dialer = chain.NewShuntDialer(dialer, subDialer, matcher)
			}

			err = jumpway.RunProxy(ctx, listener, dialer)
			if err != nil && !utils.IsClosedConnError(err) {
				log.Error(err, i18n.RunProxy())
			}
		}()
	}
	check()

	for range menu.ClickedCh {
		check()
	}
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
