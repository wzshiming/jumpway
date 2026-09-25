package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/kardianos/service"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/log"
)

var (
	globalDaemon = sync.OnceValues(newDaemon)
	logDirectory string
)

func newDaemon() (service.Service, error) {
	option := service.KeyValue{
		"UserService": runtime.GOOS == "darwin",
		"RunAtLoad":   true,
		"KeepAlive":   false,
	}
	if dir, err := config.DefaultDir(); err == nil {
		logDirectory = filepath.Join(dir, "logs")
		option["LogDirectory"] = logDirectory
	}
	// nil Interface: this handle only controls the OS service, Run is never called.
	return service.New(nil, &service.Config{
		Name:        jumpway.AppName,
		DisplayName: jumpway.AppName,
		Description: jumpway.AppDescription,
		Option:      option,
	})
}

func lookup() (service.Service, service.Status, error) {
	svc, err := globalDaemon()
	if err != nil {
		return nil, service.StatusUnknown, err
	}
	status, err := svc.Status()
	return svc, status, err
}

func IsRunning() bool {
	_, status, err := lookup()
	if err != nil {
		log.Info("daemon status", "err", err)
		return false
	}
	log.Info("daemon status", "running", status == service.StatusRunning)
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
	svc, err := globalDaemon()
	if err != nil {
		log.Error(err, "daemon install")
		return
	}
	if _, err := svc.Status(); err == nil {
		log.Info("daemon install", "status", "already installed")
		return
	}
	if logDirectory != "" {
		err = os.MkdirAll(logDirectory, 0755)
	}
	if err == nil {
		err = svc.Install()
	}
	if err != nil {
		log.Error(err, "daemon install")
		return
	}
	log.Info("daemon install", "status", "installed")
}

func Start() {
	svc, status, err := lookup()
	if err != nil {
		log.Error(err, "daemon start")
		return
	}
	if status == service.StatusRunning {
		log.Info("daemon start", "status", "already running")
		return
	}
	if err := svc.Start(); err != nil {
		log.Error(err, "daemon start")
		return
	}
	log.Info("daemon start", "status", "started")
}

func Stop() {
	svc, status, err := lookup()
	if err != nil || status != service.StatusRunning {
		log.Info("daemon stop", "status", "not running", "err", err)
		return
	}
	if err := svc.Stop(); err != nil {
		log.Error(err, "daemon stop")
		return
	}
	log.Info("daemon stop", "status", "stopped")
}

func Remove() {
	svc, _, err := lookup()
	if err != nil {
		log.Info("daemon remove", "status", "not installed", "err", err)
		return
	}
	if err := svc.Uninstall(); err != nil {
		log.Error(err, "daemon remove")
		return
	}
	log.Info("daemon remove", "status", "removed")
}

func Status() {
	_, status, err := lookup()
	if err != nil && !errors.Is(err, service.ErrNotInstalled) {
		log.Error(err, "daemon status")
	}
	log.Info("daemon status", "installed", err == nil, "running", status == service.StatusRunning)
}
