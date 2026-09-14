package jumpway

import (
	"context"
	"net"
	"net/http"
	"sync"

	"github.com/wzshiming/anyproxy"
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/cmux"
	"github.com/wzshiming/cmux/pattern"
)

func RunProxy(ctx context.Context, listener net.Listener, dialer bridge.Dialer, web http.Handler) error {
	address := listener.Addr().String()
	proxies := []string{
		"http://" + address,
		"socks5://" + address,
		"socks4://" + address,
		"ssh://" + address,
	}
	proxy, err := anyproxy.NewAnyProxy(ctx, proxies, &anyproxy.Config{
		Dialer:    dialer,
		BytesPool: BytesPool,
	})
	if err != nil {
		return err
	}

	host := proxy.Match(address)
	var serve cmux.Handler = host
	if web != nil {
		mux := cmux.NewCMux()
		prefixes := make([]string, 0, len(pattern.Pattern[pattern.HTTP]))
		for _, prefix := range pattern.Pattern[pattern.HTTP] {
			prefixes = append(prefixes, prefix+"/")
		}
		server := &http.Server{
			Handler: web,
			BaseContext: func(net.Listener) context.Context {
				return ctx
			},
		}
		if err := mux.HandlePrefix(anyproxy.NewHttpServeConn(server), prefixes...); err != nil {
			return err
		}
		if err := mux.NotFound(host); err != nil {
			return err
		}
		serve = mux
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go serve.ServeConn(conn)
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
