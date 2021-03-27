package app

import (
	"context"

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
		a.Port = int(conf.Proxy.Port)
		a.UpdateStatus()
		go func() {
			err := jumpway.RunProxy(ctx, conf.Proxy.Port, conf.Ways.Strings())
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
