package log

import (
	"os"
	"path/filepath"
	"time"

	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/logger"
	"github.com/wzshiming/logger/zap"
)

func InitLog() error {
	dir := filepath.Join(config.GetConfigDir(), "logs")
	os.MkdirAll(dir, 0755)
	logfile := filepath.Join(dir, time.Now().Format("2006_01_02_15_04_05")+".log")
	f, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	logger.Log.Info("Log redirect", "file", logfile)
	logger.SetLogger(zap.WithOut(zap.Log, f))
	os.Stdout = f
	os.Stderr = f
	return nil
}
