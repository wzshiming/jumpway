package main

import (
	"os"
	_ "time/tzdata"

	_ "github.com/wzshiming/bridge/protocols/command"
	_ "github.com/wzshiming/bridge/protocols/connect"
	_ "github.com/wzshiming/bridge/protocols/netcat"
	_ "github.com/wzshiming/bridge/protocols/shadowsocks"
	_ "github.com/wzshiming/bridge/protocols/snappy"
	_ "github.com/wzshiming/bridge/protocols/socks4"
	_ "github.com/wzshiming/bridge/protocols/socks5"
	_ "github.com/wzshiming/bridge/protocols/ssh"
	_ "github.com/wzshiming/bridge/protocols/tls"

	_ "github.com/wzshiming/anyproxy/proxies/shadowsocks"

	"github.com/wzshiming/jumpway/app/tray"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/daemon"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func main() {
	log.Info(i18n.Args(), "list", os.Args)
	if len(os.Args) == 2 {
		daemon.Run(os.Args[1])
		return
	}

	dir, err := config.DefaultDir()
	if err != nil {
		log.Error(err, "Get User Home Directory")
		os.Exit(2)
	}
	a := tray.NewApp(config.NewStore(dir))
	a.Run()
}
