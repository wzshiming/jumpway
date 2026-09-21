package tray

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
)

type virtualAddr string

func (virtualAddr) Network() string { return jumpway.VirtualNetwork }

func (a virtualAddr) String() string { return config.VirtualScheme + string(a) }

// virtualNetwork pairs the virtual listeners and dialers of one App by channel name.
type virtualNetwork struct {
	mu        sync.Mutex
	listeners map[string]*virtualListener
}

type virtualListener struct {
	network *virtualNetwork
	name    string
	conns   chan net.Conn
	done    chan struct{}
	once    sync.Once
}

type virtualConn struct {
	net.Conn
	addr virtualAddr
}

func (c virtualConn) LocalAddr() net.Addr { return c.addr }

func (c virtualConn) RemoteAddr() net.Addr { return c.addr }

func (n *virtualNetwork) Listen(name string) (net.Listener, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, ok := n.listeners[name]; ok {
		return nil, fmt.Errorf("virtual channel %q is already listening", name)
	}
	if n.listeners == nil {
		n.listeners = make(map[string]*virtualListener)
	}
	listener := &virtualListener{network: n, name: name, conns: make(chan net.Conn), done: make(chan struct{})}
	n.listeners[name] = listener
	return listener, nil
}

func (n *virtualNetwork) Dial(ctx context.Context, name string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	n.mu.Lock()
	listener, ok := n.listeners[name]
	n.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("virtual channel %q has no listener", name)
	}
	client, server := net.Pipe()
	addr := virtualAddr(name)
	select {
	case listener.conns <- virtualConn{server, addr}:
		return virtualConn{client, addr}, nil
	case <-listener.done:
		_ = client.Close()
		return nil, fmt.Errorf("virtual channel %q has no listener", name)
	case <-ctx.Done():
		_ = client.Close()
		return nil, ctx.Err()
	}
}

func (l *virtualListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *virtualListener) Close() error {
	l.once.Do(func() {
		close(l.done)
		l.network.mu.Lock()
		if l.network.listeners[l.name] == l {
			delete(l.network.listeners, l.name)
		}
		l.network.mu.Unlock()
	})
	return nil
}

func (l *virtualListener) Addr() net.Addr { return virtualAddr(l.name) }

// virtualDialer is the fixed-target dialer of a rule whose forward side is a virtual channel.
type virtualDialer struct {
	network *virtualNetwork
	name    string
}

func (d virtualDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	return d.network.Dial(ctx, d.name)
}
