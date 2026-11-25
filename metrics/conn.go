package metrics

import (
	"errors"
	"net"
	"sync"
	"time"
)

type conn struct {
	net.Conn
	count      *counter
	extra      *counter
	accepted   bool
	generation uint64
	onClose    func()
	once       sync.Once
	err        error
}

func newConn(inner net.Conn, count *counter, accepted bool) *conn {
	return &conn{Conn: inner, count: count, accepted: accepted, generation: count.open()}
}

func (c *conn) Read(buffer []byte) (int, error) {
	size, err := c.Conn.Read(buffer)
	c.record(size, true)
	return size, err
}

func (c *conn) Write(buffer []byte) (int, error) {
	size, err := c.Conn.Write(buffer)
	c.record(size, false)
	return size, err
}

func (c *conn) record(size int, read bool) {
	if size <= 0 {
		return
	}
	up := read == c.accepted
	now := time.Now().UnixNano()
	c.count.addBytes(up, int64(size), now)
	if c.extra != nil {
		c.extra.addBytes(up, int64(size), now)
	}
}

func (c *conn) Close() error {
	c.once.Do(func() {
		c.err = c.Conn.Close()
		c.count.close(c.generation)
		if c.onClose != nil {
			c.onClose()
		}
	})
	return c.err
}

func (c *conn) CloseWrite() error {
	return errors.ErrUnsupported
}

// exposed is c, a net.PacketConn too when its conn can address any destination.
func (c *conn) exposed() net.Conn {
	if _, connected := c.Conn.(*net.UDPConn); connected {
		return c
	}
	if packet, ok := c.Conn.(net.PacketConn); ok {
		return &fullConeConn{conn: c, packet: packet}
	}
	return c
}

// fullConeConn counts ReadFrom and WriteTo as conn counts Read and Write.
type fullConeConn struct {
	*conn
	packet net.PacketConn
}

func (c *fullConeConn) ReadFrom(buffer []byte) (int, net.Addr, error) {
	size, addr, err := c.packet.ReadFrom(buffer)
	c.record(size, true)
	return size, addr, err
}

func (c *fullConeConn) WriteTo(buffer []byte, addr net.Addr) (int, error) {
	size, err := c.packet.WriteTo(buffer, addr)
	c.record(size, false)
	return size, err
}

type listener struct {
	net.Listener
	count *counter
}

func (l *listener) Accept() (net.Conn, error) {
	accepted, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return newConn(accepted, l.count, true), nil
}
