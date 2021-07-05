package jumpway

import (
	"context"
	"net"
	"sync"

	"github.com/wzshiming/anyproxy"
	"github.com/wzshiming/bridge"
)

func RunProxy(ctx context.Context, listener net.Listener, dialer bridge.Dialer) error {
	address := listener.Addr().String()
	proxies := []string{
		"http://" + address,
		"socks5://" + address,
		"socks4://" + address,
		"view://" + address,
	}
	proxy, err := anyproxy.NewAnyProxy(ctx, proxies, dialer, nil, BytesPool)
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

	return nil
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
	if d == nil || len(d) < DefaultSize {
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
