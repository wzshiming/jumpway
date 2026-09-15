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
	accepted   bool
	generation uint64
	once       sync.Once
	err        error
}

func newConn(inner net.Conn, count *counter, accepted bool) *conn {
	return &conn{Conn: inner, count: count, accepted: accepted, generation: count.open()}
}

func (c *conn) Read(buffer []byte) (int, error) {
	size, err := c.Conn.Read(buffer)
	if size > 0 {
		if c.accepted {
			c.count.up.Add(int64(size))
		} else {
			c.count.down.Add(int64(size))
		}
		c.count.touch(time.Now().UnixNano())
	}
	return size, err
}

func (c *conn) Write(buffer []byte) (int, error) {
	size, err := c.Conn.Write(buffer)
	if size > 0 {
		if c.accepted {
			c.count.down.Add(int64(size))
		} else {
			c.count.up.Add(int64(size))
		}
		c.count.touch(time.Now().UnixNano())
	}
	return size, err
}

func (c *conn) Close() error {
	c.once.Do(func() {
		c.err = c.Conn.Close()
		c.count.close(c.generation)
	})
	return c.err
}

func (c *conn) CloseWrite() error {
	if closer, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return closer.CloseWrite()
	}
	return errors.ErrUnsupported
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
