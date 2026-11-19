package metrics

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/wzshiming/jumpway"
)

// WrapPacketConn accounts a UDP session's socket as one connection of the rule from the client in ctx, from its first
// WriteTo, whose destination is its target, until Close.
func (r *Rule) WrapPacketConn(ctx context.Context, inner net.PacketConn) net.PacketConn {
	return &packetConn{PacketConn: inner, rule: r, client: jumpway.ClientAddr(ctx)}
}

type packetConn struct {
	net.PacketConn
	rule   *Rule
	client string

	mu         sync.Mutex
	closed     bool
	count      *counter
	generation uint64
	// id and live are the session's live connection, absent when the rule was replaced.
	id   uint64
	live *counter

	once sync.Once
	err  error
}

func (c *packetConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	c.open(addr)
	size, err := c.PacketConn.WriteTo(p, addr)
	c.record(size, true)
	return size, err
}

func (c *packetConn) ReadFrom(p []byte) (int, net.Addr, error) {
	size, addr, err := c.PacketConn.ReadFrom(p)
	c.record(size, false)
	return size, addr, err
}

// open starts the session's entries with addr as target, unless started or closed.
func (c *packetConn) open(addr net.Addr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.count != nil || c.closed {
		return
	}
	address := addr.String()
	if udp, ok := addr.(*net.UDPAddr); ok {
		address = netip.AddrPortFrom(udp.AddrPort().Addr().Unmap(), udp.AddrPort().Port()).String()
	}
	registry := c.rule.registry
	registry.mu.Lock()
	defer registry.mu.Unlock()
	path := c.rule.path(nil)
	var via string
	if len(path) != 0 {
		via = endpoint(path[0].URL)
	}
	c.count = c.rule.target(targetKey{address: address, via: via})
	c.generation = c.count.open()
	if registry.rules[c.rule.name] != c.rule {
		return
	}
	entry := &live{
		id:      registry.nextID.Add(1),
		rule:    c.rule,
		client:  c.client,
		pending: c.rule.pending(c.client),
		target:  address,
		via:     via,
		path:    path,
		started: time.Now(),
		count:   newCounter(registry.lastTick),
		conn:    c,
	}
	registry.live[entry.id] = entry
	c.id, c.live = entry.id, entry.count
}

func (c *packetConn) record(size int, up bool) {
	if size <= 0 {
		return
	}
	c.mu.Lock()
	count, live := c.count, c.live
	c.mu.Unlock()
	if count == nil {
		return
	}
	now := time.Now().UnixNano()
	count.addBytes(up, int64(size), now)
	if live != nil {
		live.addBytes(up, int64(size), now)
	}
}

func (c *packetConn) Close() error {
	c.once.Do(func() {
		c.err = c.PacketConn.Close()
		c.mu.Lock()
		defer c.mu.Unlock()
		c.closed = true
		if c.count == nil {
			return
		}
		registry := c.rule.registry
		registry.mu.Lock()
		defer registry.mu.Unlock()
		c.count.close(c.generation)
		if c.live != nil {
			delete(registry.live, c.id)
		}
	})
	return c.err
}
