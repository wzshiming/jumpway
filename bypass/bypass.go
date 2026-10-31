// Package bypass keeps jumpway's own sockets off the TUN by binding them to the physical default-route interface.
package bypass

import (
	"errors"
	"net"
	"net/netip"
	"sync"
	"syscall"
	"time"
)

// ErrNoDefaultRoute reports that no interface holds the family's default route.
var ErrNoDefaultRoute = errors.New("bypass: no default route")

type boundIface struct {
	Index int
	Name  string
}

type lookupFunc func(ipv6 bool) (boundIface, error)

type bindFunc func(c syscall.RawConn, iface boundIface, ipv6 bool) error

// NewDialer returns a dialer that binds sockets for destinations in capture to the default-route interface current at dial time.
func NewDialer(capture []netip.Prefix) *net.Dialer {
	// Platforms without interface binding leave bindDefault nil.
	if bindDefault == nil {
		return &net.Dialer{}
	}
	return newDialer(capture, lookupDefault, bindDefault, time.Now)
}

func newDialer(capture []netip.Prefix, lookup lookupFunc, bind bindFunc, now func() time.Time) *net.Dialer {
	var cache routeCache
	d := &net.Dialer{
		Control: func(network, address string, c syscall.RawConn) error {
			ipv6, ok := shouldBind(network, address, capture)
			if !ok {
				return nil
			}
			iface, err := cache.get(ipv6, now(), lookup)
			if err != nil {
				return err
			}
			if err := bind(c, iface, ipv6); err != nil {
				// The interface may be gone, so the next dial looks it up again.
				cache.invalidate(ipv6)
				return err
			}
			return nil
		},
	}
	// DNS server addresses are IP literals, so resolving through d never recurses.
	d.Resolver = &net.Resolver{PreferGo: true, Dial: d.DialContext}
	return d
}

func lookupDefault(ipv6 bool) (boundIface, error) {
	iface, err := DefaultInterface(ipv6)
	if err != nil {
		return boundIface{}, err
	}
	return boundIface{Index: iface.Index, Name: iface.Name}, nil
}

func shouldBind(network, address string, capture []netip.Prefix) (ipv6 bool, ok bool) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		// Control receives ip networks' addresses without a port.
		host = address
	}
	literal, err := netip.ParseAddr(host)
	if err != nil {
		return false, false
	}
	ip := literal.Unmap()
	switch network {
	case "tcp", "udp", "ip":
		ipv6 = ip.Is6()
	case "tcp4", "udp4", "ip4":
		if !ip.Is4() {
			return false, false
		}
	case "tcp6", "udp6", "ip6":
		// The socket is IPv6 even when the literal is IPv4-mapped.
		if !literal.Is6() {
			return false, false
		}
		ipv6 = true
	default:
		return false, false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false, false
	}
	for _, prefix := range capture {
		if prefix.Contains(ip) {
			return ipv6, true
		}
	}
	return false, false
}

const routeTTL = time.Second

type routeEntry struct {
	iface   boundIface
	err     error
	expires time.Time
}

// routeCache caches failed lookups too, so a missing route costs one lookup per TTL rather than one per dial.
type routeCache struct {
	mu      sync.Mutex
	entries map[bool]routeEntry
}

func (c *routeCache) get(ipv6 bool, now time.Time, lookup lookupFunc) (boundIface, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[ipv6]; ok && now.Before(e.expires) {
		return e.iface, e.err
	}
	iface, err := lookup(ipv6)
	if c.entries == nil {
		c.entries = make(map[bool]routeEntry, 2)
	}
	c.entries[ipv6] = routeEntry{iface: iface, err: err, expires: now.Add(routeTTL)}
	return iface, err
}

func (c *routeCache) invalidate(ipv6 bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, ipv6)
}
