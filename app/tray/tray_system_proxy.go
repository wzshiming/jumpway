package tray

import (
	"context"
	"net"
	"net/netip"
	"net/url"

	"fyne.io/systray"
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/hostmatcher"
	"github.com/wzshiming/jumpway"
	appconfig "github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/sysproxy"
)

func (a *App) ItemProxyMode(system, global, manual *systray.MenuItem) {
	var checked proxyMode
	var tunProxy *jumpway.TUNProxy

	uncheckAll := func() {
		system.Uncheck()
		global.Uncheck()
		manual.Uncheck()
	}

	stopTUN := func() {
		if tunProxy != nil {
			tunProxy.Close()
			tunProxy = nil
		}
	}

	check := func(checked proxyMode) {
		stopTUN()

		switch checked {
		case systemMode:
			uncheckAll()
			system.Check()
			a.Mode = i18n.SystemProxy()
			a.UpdateStatus()

			err := sysproxy.OnHTTPS(a.Address)
			if err != nil {
				log.Error(err, "sysproxy.OnHTTPS")
				return
			}
			err = sysproxy.OnHTTP(a.Address)
			if err != nil {
				log.Error(err, "sysproxy.OnHTTP")
				return
			}
		case globalMode:
			uncheckAll()
			global.Check()
			a.Mode = i18n.GlobalProxy()
			a.UpdateStatus()

			err := sysproxy.OffHTTPS()
			if err != nil {
				log.Error(err, "sysproxy.OffHTTPS")
			}
			err = sysproxy.OffHTTP()
			if err != nil {
				log.Error(err, "sysproxy.OffHTTP")
			}

			conf, err := appconfig.LoadConfig()
			if err != nil {
				log.Error(err, i18n.GlobalProxy())
				return
			}

			tunName := jumpway.DefaultTUNName()
			tunAddrStr := "198.18.0.1/15"
			if conf.TUN.Name != "" {
				tunName = conf.TUN.Name
			}
			if conf.TUN.Address != "" {
				tunAddrStr = conf.TUN.Address
			}

			tunAddr, err := netip.ParsePrefix(tunAddrStr)
			if err != nil {
				log.Error(err, i18n.GlobalProxy())
				return
			}

			// Build a dialer chain using a marked base dialer so the
			// proxy's own outgoing connections bypass TUN routing.
			tunDialer, err := buildTUNDialer(conf)
			if err != nil {
				log.Error(err, i18n.GlobalProxy())
				return
			}

			// Extract proxy server IPs from config for bypass routing.
			bypassAddrs := extractProxyAddrs(conf.GetWay())

			tp, err := jumpway.RunTUNProxy(context.Background(), tunName, tunAddr, tunDialer, bypassAddrs)
			if err != nil {
				log.Error(err, i18n.GlobalProxy())
				return
			}
			tunProxy = tp
		default:
			uncheckAll()
			manual.Check()
			a.Mode = i18n.ManualProxy()
			a.UpdateStatus()

			err := sysproxy.OffHTTPS()
			if err != nil {
				log.Error(err, "sysproxy.OffHTTPS")
			}
			err = sysproxy.OffHTTP()
			if err != nil {
				log.Error(err, "sysproxy.OffHTTP")
			}
		}
	}
	check(checked)
	for {
		select {
		case <-system.ClickedCh:
			checked = systemMode
		case <-global.ClickedCh:
			checked = globalMode
		case <-manual.ClickedCh:
			checked = manualMode
		}
		check(checked)
		log.Info(i18n.ProxyMode(), "mode", checked)
	}
}

// buildTUNDialer builds a dialer chain for TUN proxy using a marked base
// dialer. On Linux, the marked dialer sets SO_MARK so that outgoing proxy
// connections bypass the TUN routing rules and go through the real interface.
func buildTUNDialer(conf *appconfig.Config) (bridge.Dialer, error) {
	markedBase := jumpway.NewMarkedDialer()

	dialer := jumpway.NewLogDialer(markedBase, func(ctx context.Context, network, address string) {
		log.Info(i18n.UseProxy(), "address", address)
	})
	dialer, err := chain.Default.BridgeChainWithConfig(context.Background(), dialer, conf.GetWay()...)
	if err != nil {
		return nil, err
	}
	dialer = jumpway.NewLogDialer(dialer, func(ctx context.Context, network, address string) {
		log.Info(i18n.Connect(), "proxy", true, "address", address)
	})

	if noProxy := conf.NoProxy.GetList(); len(noProxy) != 0 {
		matcher := hostmatcher.NewMatcher(noProxy)
		subDialer := jumpway.NewLogDialer(markedBase, func(ctx context.Context, network, address string) {
			log.Info(i18n.Connect(), "proxy", false, "address", address)
		})
		dialer = chain.NewShuntDialer(dialer, subDialer, matcher)
	}

	return dialer, nil
}

// extractProxyAddrs parses proxy server IP addresses from bridge config nodes.
// These IPs are used for bypass routing so the proxy's own connections don't
// get routed back through the TUN device (relevant on macOS and Windows).
func extractProxyAddrs(nodes []config.Node) []netip.Addr {
	seen := map[netip.Addr]struct{}{}
	var addrs []netip.Addr
	for _, node := range nodes {
		for _, raw := range node.LB {
			u, err := url.Parse(raw)
			if err != nil {
				continue
			}
			host := u.Hostname()
			if host == "" {
				continue
			}
			ip, err := netip.ParseAddr(host)
			if err != nil {
				// Hostname — try to resolve before TUN takes over routing.
				ips, err := net.LookupHost(host)
				if err != nil {
					continue
				}
				for _, ipStr := range ips {
					ip, err := netip.ParseAddr(ipStr)
					if err != nil {
						continue
					}
					if _, ok := seen[ip]; !ok {
						seen[ip] = struct{}{}
						addrs = append(addrs, ip)
					}
				}
				continue
			}
			if _, ok := seen[ip]; !ok {
				seen[ip] = struct{}{}
				addrs = append(addrs, ip)
			}
		}
	}
	return addrs
}

type proxyMode uint

const (
	manualMode proxyMode = iota
	systemMode
	globalMode
)
