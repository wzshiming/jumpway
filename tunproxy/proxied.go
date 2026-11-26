package tunproxy

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/wzshiming/tun/stack"
)

var (
	// dialRetryAfter is how long a failed dial's error answers instead of dialing again.
	dialRetryAfter = time.Second
	// remoteIdleTimeout is how long a destination's conn lives without a datagram.
	remoteIdleTimeout = 60 * time.Second
)

// pendingLimit is how many datagrams a destination keeps while its conn is dialed.
const pendingLimit = 64

// ProxiedOptions tune Proxied; the zero value lets the first dial decide whether a session is full cone.
type ProxiedOptions struct {
	// Symmetric dials one conn per destination even when the first conn could address any.
	Symmetric bool
	// Idle closes a destination's conn after this long without datagrams; 0 means 60s.
	Idle time.Duration
}

// Proxied dials each UDP session's datagrams through dialer: one conn for all if it can address any, else one per destination.
func Proxied(dialer stack.Dialer, opts ProxiedOptions) stack.ListenConfig {
	return listenPacketFunc(func(ctx context.Context, network, address string) (net.PacketConn, error) {
		ctx, cancel := context.WithCancel(ctx)
		idle := opts.Idle
		if idle == 0 {
			idle = remoteIdleTimeout
		}
		conn := &proxiedConn{
			ctx:         ctx,
			cancel:      cancel,
			dialer:      dialer,
			retryAfter:  dialRetryAfter,
			idleTimeout: idle,
			remotes:     make(map[string]*remote),
			pending:     make(map[string]*[][]byte),
			failures:    make(map[string]failure),
			decided:     make(chan struct{}),
			replies:     make(chan datagram),
			done:        make(chan struct{}),
		}
		if opts.Symmetric {
			conn.started = true
			close(conn.decided)
		}
		return conn, nil
	})
}

// proxiedConn is one UDP session's socket, dialing on the first datagram to a destination.
type proxiedConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	dialer stack.Dialer
	// dialMu keeps the datagrams of a session not yet decided from dialing at once.
	dialMu      sync.Mutex
	retryAfter  time.Duration
	idleTimeout time.Duration

	mu      sync.Mutex
	cone    net.PacketConn
	remotes map[string]*remote
	// pending are the datagrams kept for each destination whose conn is being dialed.
	pending map[string]*[][]byte
	// failure is the last failed dial until one succeeds, failures then that of each destination.
	failure  failure
	failures map[string]failure
	started  bool
	closed   bool

	// decided is closed once it is known whether the session is full cone: at the first dial, or at once when symmetric.
	decided chan struct{}
	replies chan datagram
	done    chan struct{}
}

// remote is the conn dialed for one destination.
type remote struct {
	net.Conn
	source net.Addr
	// active is when it last carried a datagram, guarded by the session's mu.
	active time.Time
}

type datagram struct {
	payload []byte
	source  net.Addr
}

// failure is a dial's error, answered instead of dialing until retry.
type failure struct {
	err   error
	retry time.Time
}

func (c *proxiedConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	select {
	case <-c.decided:
	default:
		cone, remote, err := c.openFirst(addr)
		if err != nil {
			return 0, err
		}
		if cone != nil {
			return cone.WriteTo(p, addr)
		}
		if remote != nil {
			return remote.Write(p)
		}
	}
	if c.cone != nil {
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return 0, net.ErrClosed
		}
		return c.cone.WriteTo(p, addr)
	}
	return c.writeDestination(p, addr)
}

// openFirst dials the session's first conn, which decides whether it is full cone, unless another write already did.
func (c *proxiedConn) openFirst(addr net.Addr) (net.PacketConn, *remote, error) {
	c.dialMu.Lock()
	defer c.dialMu.Unlock()
	destination := addr.String()
	now := time.Now()
	c.mu.Lock()
	started, closed, failed := c.started, c.closed, c.failure
	c.mu.Unlock()
	if closed {
		return nil, nil, net.ErrClosed
	}
	if started {
		return nil, nil, nil
	}
	if now.Before(failed.retry) {
		return nil, nil, failed.err
	}
	conn, err := c.dialer.DialContext(c.ctx, "udp", destination)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.failure = failure{err: err, retry: time.Now().Add(c.retryAfter)}
		return nil, nil, err
	}
	if c.closed {
		conn.Close()
		return nil, nil, net.ErrClosed
	}
	c.started = true
	defer close(c.decided)
	// A dialed *net.UDPConn is connected, so it reaches its destination only.
	if packet, ok := conn.(net.PacketConn); ok {
		if _, connected := conn.(*net.UDPConn); !connected {
			c.cone = packet
			return packet, nil, nil
		}
	}
	found := &remote{Conn: conn, source: addr, active: time.Now()}
	c.remotes[destination] = found
	go c.relay(destination, found)
	return nil, found, nil
}

// writeDestination sends p over addr's conn, or queues it while that conn is dialed without waiting for the dial.
func (c *proxiedConn) writeDestination(p []byte, addr net.Addr) (int, error) {
	destination := addr.String()
	now := time.Now()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, net.ErrClosed
	}
	c.expire(now)
	if queue, dialing := c.pending[destination]; dialing {
		if len(*queue) < pendingLimit {
			*queue = append(*queue, bytes.Clone(p))
		}
		c.mu.Unlock()
		return len(p), nil
	}
	if found := c.remotes[destination]; found != nil {
		found.active = now
		c.mu.Unlock()
		return found.Write(p)
	}
	if failed := c.failures[destination]; now.Before(failed.retry) {
		c.mu.Unlock()
		return 0, failed.err
	}
	c.pending[destination] = &[][]byte{bytes.Clone(p)}
	c.mu.Unlock()
	go c.dial(destination, addr)
	return len(p), nil
}

// dial opens destination's conn, then sends the datagrams queued for it in order before writes use it directly.
func (c *proxiedConn) dial(destination string, addr net.Addr) {
	conn, err := c.dialer.DialContext(c.ctx, "udp", destination)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		if conn != nil {
			conn.Close()
		}
		return
	}
	if err != nil {
		delete(c.pending, destination)
		c.failures[destination] = failure{err: err, retry: time.Now().Add(c.retryAfter)}
		c.mu.Unlock()
		return
	}
	delete(c.failures, destination)
	found := &remote{Conn: conn, source: addr, active: time.Now()}
	c.remotes[destination] = found
	go c.relay(destination, found)
	for {
		queue := c.pending[destination]
		queued := *queue
		if len(queued) == 0 {
			delete(c.pending, destination)
			c.mu.Unlock()
			return
		}
		*queue = nil
		c.mu.Unlock()
		for _, payload := range queued {
			found.Write(payload)
		}
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
	}
}

// expire closes and forgets the destinations' conns that carried no datagram for the idle timeout, and the dial failures past their retry.
func (c *proxiedConn) expire(now time.Time) {
	for destination, r := range c.remotes {
		if now.Sub(r.active) >= c.idleTimeout {
			delete(c.remotes, destination)
			r.Close()
		}
	}
	for destination, failed := range c.failures {
		if !now.Before(failed.retry) {
			delete(c.failures, destination)
		}
	}
}

// relay passes r's replies to ReadFrom until r fails, then forgets r.
func (c *proxiedConn) relay(destination string, r *remote) {
	buf := make([]byte, 65535)
	for {
		n, err := r.Read(buf)
		if err != nil {
			c.mu.Lock()
			if c.remotes[destination] == r {
				delete(c.remotes, destination)
			}
			c.mu.Unlock()
			r.Close()
			return
		}
		c.mu.Lock()
		r.active = time.Now()
		c.mu.Unlock()
		select {
		case c.replies <- datagram{payload: bytes.Clone(buf[:n]), source: r.source}:
		case <-c.done:
			return
		}
	}
}

func (c *proxiedConn) ReadFrom(p []byte) (int, net.Addr, error) {
	select {
	case <-c.decided:
	case <-c.done:
		return 0, nil, net.ErrClosed
	}
	if c.cone != nil {
		n, source, err := c.cone.ReadFrom(p)
		return n, udpAddr(source), err
	}
	select {
	case reply := <-c.replies:
		return copy(p, reply.payload), reply.source, nil
	case <-c.done:
		return 0, nil, net.ErrClosed
	}
}

// udpAddr is addr as a *net.UDPAddr, which a TUN session answers from, when it names an IP and port.
func udpAddr(addr net.Addr) net.Addr {
	if _, ok := addr.(*net.UDPAddr); ok || addr == nil {
		return addr
	}
	if addrPort, err := netip.ParseAddrPort(addr.String()); err == nil {
		return net.UDPAddrFromAddrPort(addrPort)
	}
	return addr
}

func (c *proxiedConn) Close() error {
	c.cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.done)
	var errs []error
	if c.cone != nil {
		errs = append(errs, c.cone.Close())
	}
	for _, r := range c.remotes {
		errs = append(errs, r.Close())
	}
	c.remotes = nil
	c.pending = nil
	return errors.Join(errs...)
}

func (c *proxiedConn) LocalAddr() net.Addr {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cone != nil {
		return c.cone.LocalAddr()
	}
	return &net.UDPAddr{}
}

func (c *proxiedConn) SetDeadline(t time.Time) error      { return errors.ErrUnsupported }
func (c *proxiedConn) SetReadDeadline(t time.Time) error  { return errors.ErrUnsupported }
func (c *proxiedConn) SetWriteDeadline(t time.Time) error { return errors.ErrUnsupported }
