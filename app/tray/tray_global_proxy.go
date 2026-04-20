package tray

import (
	"context"
	"net/netip"

	"fyne.io/systray"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func (a *App) ItemGlobalProxy(menu *systray.MenuItem) {
	var tp *tunState

	for range menu.ClickedCh {
		if menu.Checked() {
			menu.Uncheck()
			if tp != nil {
				tp.stop()
				tp = nil
			}
			log.Info(i18n.GlobalProxy(), "status", "stopped")
			continue
		}

		// Stop any existing TUN before starting a new one.
		if tp != nil {
			tp.stop()
			tp = nil
		}

		t, err := startGlobalProxy()
		if err != nil {
			log.Error(err, i18n.GlobalProxy())
			continue
		}
		tp = t

		menu.Check()
		log.Info(i18n.GlobalProxy(), "status", "started")
	}

	if tp != nil {
		tp.stop()
	}
}

type tunState struct {
	tunProxy *jumpway.TunProxy
	cancel   context.CancelFunc
}

func (t *tunState) stop() {
	if t.tunProxy != nil {
		err := t.tunProxy.Close()
		if err != nil {
			log.Error(err, "close tun")
		}
	}
	if t.cancel != nil {
		t.cancel()
	}
}

func startGlobalProxy() (*tunState, error) {
	conf, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}

	var inet4Address []netip.Prefix
	if conf.Tun.Inet4Address != "" {
		prefix, err := netip.ParsePrefix(conf.Tun.Inet4Address)
		if err != nil {
			return nil, err
		}
		inet4Address = []netip.Prefix{prefix}
	}

	cfg := jumpway.TunConfig{
		Name:         conf.Tun.Name,
		MTU:          conf.Tun.MTU,
		Inet4Address: inet4Address,
		AutoRoute:    conf.Tun.AutoRoute,
		Stack:        conf.Tun.Stack,
	}

	ctx, cancel := context.WithCancel(context.Background())

	dialer := jumpway.NewLogDialer(local.LOCAL, func(ctx context.Context, network, address string) {
		log.Info(i18n.UseProxy(), "address", address)
	})
	dialer, err = chain.Default.BridgeChainWithConfig(ctx, dialer, conf.GetWay()...)
	if err != nil {
		cancel()
		return nil, err
	}
	dialer = jumpway.NewLogDialer(dialer, func(ctx context.Context, network, address string) {
		log.Info(i18n.Connect(), "proxy", true, "address", address)
	})

	tunp, err := jumpway.NewTunProxy(ctx, dialer, cfg, &tunLogger{})
	if err != nil {
		cancel()
		return nil, err
	}

	err = tunp.Start()
	if err != nil {
		tunp.Close()
		cancel()
		return nil, err
	}

	return &tunState{tunProxy: tunp, cancel: cancel}, nil
}

// tunLogger adapts the project's logger to sing-tun's logger interface.
type tunLogger struct{}

func (t *tunLogger) Trace(args ...any) { log.Info("tun [trace]", "msg", args) }
func (t *tunLogger) Debug(args ...any) { log.Info("tun [debug]", "msg", args) }
func (t *tunLogger) Info(args ...any)  { log.Info("tun [info]", "msg", args) }
func (t *tunLogger) Warn(args ...any)  { log.Info("tun [warn]", "msg", args) }
func (t *tunLogger) Error(args ...any) { log.Info("tun [error]", "msg", args) }
func (t *tunLogger) Fatal(args ...any) { log.Info("tun [fatal]", "msg", args) }
func (t *tunLogger) Panic(args ...any) { log.Info("tun [panic]", "msg", args) }
