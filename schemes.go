package jumpway

import (
	"context"
	"net"

	"github.com/wzshiming/anyproxy"
	socks4adapter "github.com/wzshiming/anyproxy/proxies/socks4"
	socks5adapter "github.com/wzshiming/anyproxy/proxies/socks5"
	sshproxyadapter "github.com/wzshiming/anyproxy/proxies/sshproxy"
	"github.com/wzshiming/cmux/pattern"
	"github.com/wzshiming/httpproxy"
	"github.com/wzshiming/socks4"
	"github.com/wzshiming/socks5"
	"github.com/wzshiming/sshproxy"
)

// Attribute statistics to each client instead of anyproxy's shared server context.
func init() {
	anyproxy.Register("http", newHTTPServeConn)
	anyproxy.Register("socks5", newSOCKS5ServeConn)
	anyproxy.Register("socks4", newSOCKS4ServeConn)
	anyproxy.Register("ssh", newSSHServeConn)
}

type serveConnFunc func(net.Conn)

func (serve serveConnFunc) ServeConn(conn net.Conn) {
	serve(conn)
}

var httpPatterns = append(pattern.Pattern[pattern.HTTP], pattern.Pattern[pattern.HTTP2]...)

func newHTTPServeConn(ctx context.Context, scheme, address string, conf *anyproxy.Config) (anyproxy.ServeConn, []string, error) {
	server, err := httpproxy.NewSimpleServer(scheme + "://" + address)
	if err != nil {
		return nil, nil, err
	}
	server.Server.BaseContext = func(net.Listener) context.Context {
		return ctx
	}
	server.Server.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		return WithClientAddr(ctx, conn.RemoteAddr())
	}
	if conf.Users != nil {
		auth := map[string]string{}
		for _, user := range conf.Users {
			password, _ := user.Password()
			auth[user.Username()] = password
		}
		server.Authentication = httpproxy.BasicAuthFunc(func(username, password string) bool {
			expected, ok := auth[username]
			return ok && expected == password
		})
	}
	server.Logger = conf.Logger
	if conf.Dialer != nil {
		server.ProxyDial = conf.Dialer.DialContext
	}
	server.BytesPool = conf.BytesPool
	return anyproxy.NewHttpServeConn(&server.Server), httpPatterns, nil
}

func newSOCKS5ServeConn(ctx context.Context, scheme, address string, conf *anyproxy.Config) (anyproxy.ServeConn, []string, error) {
	host, patterns, err := socks5adapter.NewServeConn(ctx, scheme, address, conf)
	if err != nil {
		return nil, nil, err
	}
	template := host.(*socks5.SimpleServer)
	return serveConnFunc(func(conn net.Conn) {
		// SOCKS servers embed a mutex, so the template is never copied whole.
		server := socks5.Server{
			Context:                   WithClientAddr(ctx, conn.RemoteAddr()),
			Authentication:            template.Authentication,
			ProxyDial:                 template.ProxyDial,
			ProxyListen:               template.ProxyListen,
			ProxyListenPacket:         template.ProxyListenPacket,
			ProxyOutgoingListenPacket: template.ProxyOutgoingListenPacket,
			PacketForwardAddress:      template.PacketForwardAddress,
			ProxyListenBind:           template.ProxyListenBind,
			ListenBindReuseTimeout:    template.ListenBindReuseTimeout,
			ListenBindAcceptTimeout:   template.ListenBindAcceptTimeout,
			Logger:                    template.Logger,
			BytesPool:                 template.BytesPool,
		}
		server.ServeConn(conn)
	}), patterns, nil
}

func newSOCKS4ServeConn(ctx context.Context, scheme, address string, conf *anyproxy.Config) (anyproxy.ServeConn, []string, error) {
	host, patterns, err := socks4adapter.NewServeConn(ctx, scheme, address, conf)
	if err != nil {
		return nil, nil, err
	}
	template := host.(*socks4.SimpleServer)
	return serveConnFunc(func(conn net.Conn) {
		server := socks4.Server{
			Context:                 WithClientAddr(ctx, conn.RemoteAddr()),
			Authentication:          template.Authentication,
			ProxyDial:               template.ProxyDial,
			ProxyListenBind:         template.ProxyListenBind,
			ListenBindReuseTimeout:  template.ListenBindReuseTimeout,
			ListenBindAcceptTimeout: template.ListenBindAcceptTimeout,
			Logger:                  template.Logger,
			BytesPool:               template.BytesPool,
		}
		server.ServeConn(conn)
	}), patterns, nil
}

func newSSHServeConn(ctx context.Context, scheme, address string, conf *anyproxy.Config) (anyproxy.ServeConn, []string, error) {
	host, patterns, err := sshproxyadapter.NewServeConn(ctx, scheme, address, conf)
	if err != nil {
		return nil, nil, err
	}
	template := host.(*sshproxy.SimpleServer)
	return serveConnFunc(func(conn net.Conn) {
		server := template.Server
		server.Context = WithClientAddr(ctx, conn.RemoteAddr())
		server.ServeConn(conn)
	}), patterns, nil
}
