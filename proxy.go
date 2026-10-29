package jumpway

import (
	"context"
	"errors"
	"net"
	"net/url"

	"github.com/wzshiming/anyproxy"
	"github.com/wzshiming/bridge"
)

// VirtualNetwork is the Addr.Network of in-process channel endpoints.
const VirtualNetwork = "virtual"

// Scheme is one proxy protocol served on a listener: its URL scheme and credentials, nil for unauthenticated.
type Scheme struct {
	Type string
	User *url.Userinfo
}

func RunProxy(ctx context.Context, listener net.Listener, dialer bridge.Dialer, schemes []Scheme) error {
	if len(schemes) == 0 {
		return errors.New("no proxy protocols to serve")
	}
	addr := listener.Addr()
	if addr == nil {
		return errors.New("listener has no address")
	}
	address := addr.String()
	if addr.Network() == VirtualNetwork {
		// anyproxy keys protocol servers by host:port, which a channel name is not.
		address = "virtual.invalid:0"
	}
	proxies := make([]string, 0, len(schemes))
	for _, scheme := range schemes {
		proxyAddress := address
		if scheme.User != nil {
			proxyAddress = scheme.User.String() + "@" + address
		}
		proxies = append(proxies, scheme.Type+"://"+proxyAddress)
	}
	proxy, err := anyproxy.NewAnyProxy(ctx, proxies, &anyproxy.Config{
		Dialer:    dialer,
		BytesPool: BytesPool,
	})
	if err != nil {
		return err
	}

	host := proxy.Match(address)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go host.ServeConn(conn)
	}
}
