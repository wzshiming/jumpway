package logger

import (
	"github.com/go-logr/logr"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/sysnotify"
)

var (
	Std = logr.Discard()
)

func Error(err error, msg string, keysAndValues ...interface{}) {
	Std.Error(err, msg, keysAndValues...)
	e := sysnotify.Alert(jumpway.AppName+" "+msg, err.Error(), "")
	if e != nil {
		Std.Error(err, i18n.Alert(msg), keysAndValues...)
	}
}
