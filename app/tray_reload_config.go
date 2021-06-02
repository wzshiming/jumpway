package app

import (
	"context"
	"fmt"

	"github.com/getlantern/systray"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/logger"
)

func (a *App) ItemReloadConfig(menu *systray.MenuItem) {
	var cancel func()
	var ctx context.Context

	check := func() {
		logger.Log.Info("Reload config")
		if cancel != nil {
			cancel()
		}
		ctx, cancel = context.WithCancel(context.Background())
		conf, err := config.LoadConfig()
		if err != nil {
			logger.Log.Error(err, "LoadConfig")
			systray.Quit()
		}
		port := conf.Proxy.Port
		host := conf.Proxy.Host
		a.Port = int(port)
		a.RawHost = host
		a.Host = host
		if a.Host == "" || a.Host == "0.0.0.0" {
			a.Host = "127.0.0.1"
		}
		a.UpdateStatus()
		go func() {
			err := jumpway.RunProxy(ctx, fmt.Sprintf("%s:%d", host, port), conf.Ways.Strings())
			if err != nil {
				logger.Log.Error(err, "RunProxy")
				systray.Quit()
			}
		}()
	}
	check()

	for range menu.ClickedCh {
		check()
	}
}
