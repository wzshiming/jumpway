package log

import (
	"log/slog"
	"os"
	"sync"

	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/sysnotify"
)

var (
	logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	mut    sync.RWMutex
)

func Redirect(logfile string) error {
	f, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	mut.Lock()
	defer mut.Unlock()

	logger = slog.New(slog.NewTextHandler(f, nil))
	return nil
}

func Error(err error, msg string, keysAndValues ...interface{}) {
	mut.RLock()
	defer mut.RUnlock()

	logger.Error(msg, append([]any{"err", err}, keysAndValues...))
	e := sysnotify.Alert(jumpway.AppName+" "+msg, err.Error(), "")
	if e != nil {
		logger.Error(i18n.Alert(msg), "err", e)
	}
}

func Info(msg string, keysAndValues ...interface{}) {
	mut.RLock()
	defer mut.RUnlock()

	logger.Info(msg, keysAndValues...)
}
