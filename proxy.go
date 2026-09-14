package jumpway

import (
	"context"
	"net"
	"net/url"
	"sync"

	"github.com/wzshiming/anyproxy"
	"github.com/wzshiming/bridge"
)

func RunProxy(ctx context.Context, listener net.Listener, dialer bridge.Dialer, user *url.Userinfo) error {
	address := listener.Addr().String()
	proxyAddress := address
	if user != nil {
		proxyAddress = user.String() + "@" + address
	}
	proxies := []string{
		"http://" + proxyAddress,
		"socks5://" + proxyAddress,
		"socks4://" + proxyAddress,
		"ssh://" + proxyAddress,
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

var DefaultSize = 32 * 1024

type bytesPool struct {
	sync.Pool
}

func (b *bytesPool) Get() []byte {
	buf := b.Pool.Get().([]byte)
	buf = buf[:cap(buf)]
	return buf
}

func (b *bytesPool) Put(d []byte) {
	if cap(d) < DefaultSize {
		return
	}
	b.Pool.Put(d)
}

var BytesPool = &bytesPool{
	Pool: sync.Pool{
		New: func() interface{} {
			return make([]byte, DefaultSize)
		},
	},
}
