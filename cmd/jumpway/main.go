package main

import (
	"os"
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

	"github.com/wzshiming/jumpway/app/tray"
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

	a := tray.NewApp()
	a.Run()
}
