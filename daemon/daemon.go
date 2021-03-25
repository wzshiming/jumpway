package daemon

import (
	"os"

	"github.com/takama/daemon"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/logger"
)

var (
	globalDaemon daemon.Daemon
)

func init() {
	svc, err := daemon.New(jumpway.AppName, jumpway.AppDescription, daemon.UserAgent)
	if err != nil {
		logger.Log.Error(err, "get daemon")
		os.Exit(2)
	}
	globalDaemon = svc
}

func DaemonIsRunning() bool {
	status, err := globalDaemon.Status()
	if err != nil {
		logger.Log.Info("daemon status", "status", err)
		return false
	}
	logger.Log.Info("daemon status", "status", status)
	return true
}

func Run(command string) {
	switch command {
	case "start":
		Install()
		Start()
		return
	case "stop":
		Stop()
		return
	case "install":
		Install()
		return
	case "remove":
		Stop()
		Remove()
		return
	case "status":
		Status()
		return
	default:
		logger.Log.Error(nil, "Command not defined", "command", command)
		return
	}
}

func Install() {
	status, err := globalDaemon.Install()
	if err != nil {
		logger.Log.Error(err, "daemon install")
		//continue
	}
	logger.Log.Info("daemon install", "status", status)
}

func Start() {
	status, err := globalDaemon.Start()
	if err != nil {
		logger.Log.Error(err, "daemon start")
		//continue
	}
	logger.Log.Info("daemon start", "status", status)
}

func Stop() {
	status, err := globalDaemon.Stop()
	if err != nil {
		logger.Log.Error(err, "daemon stop")
		//continue
	}
	logger.Log.Info("daemon stop", "status", status)
}

func Remove() {
	status, err := globalDaemon.Remove()
	if err != nil {
		logger.Log.Error(err, "daemon remove")
		//continue
	}
	logger.Log.Info("daemon remove", "status", status)
}

func Status() {
	status, err := globalDaemon.Status()
	if err != nil {
		logger.Log.Error(err, "daemon status")
		//continue
	}
	logger.Log.Info("daemon status", "status", status)
}
