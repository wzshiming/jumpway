package tray

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

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
