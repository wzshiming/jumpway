package log

import (
	"os"

	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/logger"
	"github.com/wzshiming/logger/zap"
	"github.com/wzshiming/sysnotify"
)

func Redirect(logfile string) error {
	f, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	Info(i18n.RedirectLog(), "file", logfile)
	logger.SetLogger(zap.WithOut(zap.Log, f))
	os.Stdout = f
	os.Stderr = f
	return nil
}

func Error(err error, msg string, keysAndValues ...interface{}) {
	logger.Log.Error(err, msg, keysAndValues...)
	e := sysnotify.Alert(jumpway.AppName+" "+msg, err.Error(), "")
	if e != nil {
		logger.Log.Error(err, i18n.Alert(msg), keysAndValues...)
	}
}

func Info(msg string, keysAndValues ...interface{}) {
	logger.Log.Info(msg, keysAndValues...)
}
