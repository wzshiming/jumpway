package jumpway

import (
	"context"
	"fmt"

	"github.com/wzshiming/bridge/chain"
	_ "github.com/wzshiming/bridge/protocols/command"
	_ "github.com/wzshiming/bridge/protocols/connect"
	_ "github.com/wzshiming/bridge/protocols/netcat"
	_ "github.com/wzshiming/bridge/protocols/shadowsocks"
	_ "github.com/wzshiming/bridge/protocols/smux"
	_ "github.com/wzshiming/bridge/protocols/socks4"
	_ "github.com/wzshiming/bridge/protocols/socks5"
	_ "github.com/wzshiming/bridge/protocols/ssh"
	_ "github.com/wzshiming/bridge/protocols/tls"
	_ "github.com/wzshiming/bridge/protocols/ws"
	"github.com/wzshiming/logger"
)

func RunProxy(ctx context.Context, port uint32, ways []string) error {
	address := fmt.Sprintf(":%d", port)
	proxies := Way{"http://" + address, "socks5://" + address, "socks4://" + address}
	listens := []string{proxies.String()}
	dials := append([]string{"-"}, ways...)
	logger.Log.Info("listens", "list", listens)
	logger.Log.Info("dials", "list", dials)
	return chain.Bridge(ctx, listens, dials, false)
}
