package tray

import (
	"context"
	"net"
	"strconv"

	"github.com/wzshiming/bridge/protocols/local"

	"github.com/getlantern/systray"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/jumpway/utils"
)

func (a *App) ItemReloadConfig(menu *systray.MenuItem) {
	var cancel func()
	var ctx context.Context
	var listener net.Listener

	check := func() {
		log.Info("Reload config")
		if cancel != nil {
			cancel()
		}
		ctx, cancel = context.WithCancel(context.Background())
		conf, err := config.LoadConfig()
		if err != nil {
			log.Error(err, "LoadConfig")
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
			log.Error(err, "Listen")
			return
		}

		a.Address = listener.Addr().String()
		a.RawHost = host
		a.UpdateStatus()
		go func() {
			err := jumpway.RunProxy(ctx, listener, conf.GetWay(), conf.NoProxy.GetList())
			if err != nil && !utils.IsClosedConnError(err) {
				log.Error(err, "RunProxy")
			}
		}()
	}
	check()

	for range menu.ClickedCh {
		check()
	}
}
