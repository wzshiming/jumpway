package main

import (
	"os"
	"path/filepath"
	"time"
	_ "time/tzdata"

	_ "github.com/wzshiming/jumpway/app/web"

	_ "github.com/wzshiming/bridge/protocols/command"
	_ "github.com/wzshiming/bridge/protocols/connect"
	_ "github.com/wzshiming/bridge/protocols/emux"
	_ "github.com/wzshiming/bridge/protocols/netcat"
	_ "github.com/wzshiming/bridge/protocols/shadowsocks"
	_ "github.com/wzshiming/bridge/protocols/snappy"
	_ "github.com/wzshiming/bridge/protocols/socks4"
	_ "github.com/wzshiming/bridge/protocols/socks5"
	_ "github.com/wzshiming/bridge/protocols/ssh"
	_ "github.com/wzshiming/bridge/protocols/tls"

	_ "github.com/wzshiming/anyproxy/proxies/httpproxy"
	_ "github.com/wzshiming/anyproxy/proxies/shadowsocks"
	_ "github.com/wzshiming/anyproxy/proxies/socks4"
	_ "github.com/wzshiming/anyproxy/proxies/socks5"
	_ "github.com/wzshiming/anyproxy/proxies/sshproxy"

	"github.com/go-logr/zapr"
	"github.com/wzshiming/jumpway/app/tray"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/daemon"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/logger"
	"go.uber.org/zap"
)

func main() {
	if len(os.Args) == 2 {
		daemon.Run(os.Args[1])
		return
	}

	logdir := filepath.Join(config.GetConfigDir(), "logs")
	logfile := filepath.Join(logdir, time.Now().Format("2006_01_02_15_04_05")+".log")

	logConfig := zap.NewDevelopmentConfig()
	logConfig.OutputPaths = []string{logfile}
	logConfig.ErrorOutputPaths = logConfig.OutputPaths
	zapLog, err := logConfig.Build()
	if err != nil {
		logger.Error(err, "who watches the watchmen")
		os.Exit(1)
	}
	logger.Std = zapr.NewLogger(zapLog)

	err = config.InitConfig()
	if err != nil {
		logger.Error(err, i18n.InitConfig())
		return
	}

	a := tray.NewApp()
	a.Log = logfile
	a.Run()
}
