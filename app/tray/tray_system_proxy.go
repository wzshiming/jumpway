package tray

import (
	"context"
	"net/netip"

	"fyne.io/systray"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
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

			if a.Dialer == nil {
				log.Info("global proxy: dialer not ready")
				return
			}

			tunName := "jumpway0"
			tunAddrStr := "198.18.0.1/15"
			if conf, err := config.LoadConfig(); err == nil {
				if conf.TUN.Name != "" {
					tunName = conf.TUN.Name
				}
				if conf.TUN.Address != "" {
					tunAddrStr = conf.TUN.Address
				}
			}

			tunAddr, err := netip.ParsePrefix(tunAddrStr)
			if err != nil {
				log.Error(err, i18n.GlobalProxy())
				return
			}

			tp, err := jumpway.RunTUNProxy(context.Background(), tunName, tunAddr, a.Dialer)
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

type proxyMode uint

const (
	manualMode proxyMode = iota
	systemMode
	globalMode
)
