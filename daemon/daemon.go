package daemon

import (
	"os"

	"github.com/takama/daemon"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/log"
)

var (
	globalDaemon daemon.Daemon
)

func init() {
	svc, err := daemon.New(jumpway.AppName, jumpway.AppDescription, Kind)
	if err != nil {
		log.Error(err, "get daemon")
		os.Exit(2)
	}
	globalDaemon = svc
}

func DaemonIsRunning() bool {
	status, err := globalDaemon.Status()
	if err != nil {
		log.Info("daemon status", "status", err)
		return false
	}
	log.Info("daemon status", "status", status)
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
		log.Error(nil, "Command not defined", "command", command)
		return
	}
}

func Install() {
	status, err := globalDaemon.Install()
	if err != nil {
		log.Error(err, "daemon install")
	}
	log.Info("daemon install", "status", status)
}

func Start() {
	status, err := globalDaemon.Start()
	if err != nil {
		log.Error(err, "daemon start")
	}
	log.Info("daemon start", "status", status)
}

func Stop() {
	status, err := globalDaemon.Stop()
	if err != nil {
		log.Error(err, "daemon stop")
	}
	log.Info("daemon stop", "status", status)
}

func Remove() {
	status, err := globalDaemon.Remove()
	if err != nil {
		log.Error(err, "daemon remove")
	}
	log.Info("daemon remove", "status", status)
}

func Status() {
	status, err := globalDaemon.Status()
	if err != nil {
		log.Error(err, "daemon status")
	}
	log.Info("daemon status", "status", status)
}
