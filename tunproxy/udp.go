package tunproxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/wzshiming/tun/bypass"
	"github.com/wzshiming/tun/stack"
)

var newBoundListenConfig = bypass.NewListenConfig

// Direct gives each UDP session one socket, bound to the physical interface when its first destination is captured.
func Direct(capture []netip.Prefix) stack.ListenConfig {
	return listenPacketFunc(func(ctx context.Context, network, address string) (net.PacketConn, error) {
		return &directConn{ctx: context.WithoutCancel(ctx), capture: capture, network: network, address: address, opened: make(chan struct{})}, nil
	})
}

// directConn opens its socket on the first datagram, choosing it by that datagram's destination.
type directConn struct {
	ctx              context.Context
	capture          []netip.Prefix
	network, address string

	mu     sync.Mutex
	conn   net.PacketConn
	closed bool
	// opened is closed once conn is set or the session closed.
	opened chan struct{}
}

func (c *directConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, &net.OpError{Op: "write", Net: c.network, Addr: addr, Err: net.ErrClosed}
	}
	if c.conn == nil {
		if err := c.open(addr); err != nil {
			c.mu.Unlock()
			return 0, err
		}
	}
	conn := c.conn
	c.mu.Unlock()
	return conn.WriteTo(p, addr)
}

// open listens in the requested family, or else that of destination, binding the socket when bypass would bind a dial to it.
func (c *directConn) open(destination net.Addr) error {
	network, listen := c.network, &net.ListenConfig{}
	if addrPort, err := netip.ParseAddrPort(destination.String()); err == nil {
		ip := addrPort.Addr().Unmap()
		if network == "udp" {
			network = "udp6"
			if ip.Is4() {
				network = "udp4"
			}
		}
		if c.bound(ip) {
			listen = newBoundListenConfig(c.capture)
		}
	}
	conn, err := listen.ListenPacket(c.ctx, network, c.address)
	if err != nil {
		return err
	}
	c.conn = conn
	close(c.opened)
	return nil
}

// bound reports whether a socket to ip escapes the TUN only bound, by bypass.NewDialer's rule.
func (c *directConn) bound(ip netip.Addr) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	return slices.ContainsFunc(c.capture, func(prefix netip.Prefix) bool { return prefix.Contains(ip) })
}

func (c *directConn) ReadFrom(p []byte) (int, net.Addr, error) {
	<-c.opened
	c.mu.Lock()
	conn, closed := c.conn, c.closed
	c.mu.Unlock()
	if closed {
		return 0, nil, &net.OpError{Op: "read", Net: c.network, Err: net.ErrClosed}
	}
	return conn.ReadFrom(p)
}

func (c *directConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.conn == nil {
		close(c.opened)
		return nil
	}
	return c.conn.Close()
}

func (c *directConn) LocalAddr() net.Addr {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return &net.UDPAddr{}
	}
	return c.conn.LocalAddr()
}

func (c *directConn) SetDeadline(t time.Time) error {
	return errors.Join(c.SetReadDeadline(t), c.SetWriteDeadline(t))
}

func (c *directConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	return c.conn.SetReadDeadline(t)
}

func (c *directConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	return c.conn.SetWriteDeadline(t)
}

// listenPacketFunc opens the socket of each UDP session the TUN captures.
type listenPacketFunc func(ctx context.Context, network, address string) (net.PacketConn, error)

func (listenPacketFunc) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return nil, fmt.Errorf("tun: listen %s %s: %w", network, address, errors.ErrUnsupported)
}

func (listen listenPacketFunc) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	return listen(ctx, network, address)
}
