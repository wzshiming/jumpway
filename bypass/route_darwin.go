//go:build darwin

package bypass

import (
	"net"
	"os"
	"syscall"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

// DefaultInterface returns the interface holding the family's default route, or ErrNoDefaultRoute when there is none.
func DefaultInterface(ipv6 bool) (*net.Interface, error) {
	af := unix.AF_INET
	if ipv6 {
		af = unix.AF_INET6
	}
	rib, err := route.FetchRIB(af, route.RIBTypeRoute, 0)
	if err != nil {
		return nil, err
	}
	msgs, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil, err
	}
	index, err := defaultRouteIndex(msgs, ipv6)
	if err != nil {
		return nil, err
	}
	return net.InterfaceByIndex(index)
}

func defaultRouteIndex(msgs []route.Message, ipv6 bool) (int, error) {
	family := unix.AF_INET
	if ipv6 {
		family = unix.AF_INET6
	}
	index, found := 0, false
	for _, msg := range msgs {
		m, ok := msg.(*route.RouteMessage)
		// RTF_IFSCOPE defaults only serve sockets already bound to their interface.
		if !ok || m.Flags&unix.RTF_UP == 0 || m.Flags&(unix.RTF_IFSCOPE|unix.RTF_HOST) != 0 {
			continue
		}
		if len(m.Addrs) <= unix.RTAX_DST || !zeroAddr(m.Addrs[unix.RTAX_DST]) || m.Addrs[unix.RTAX_DST].Family() != family {
			continue
		}
		if len(m.Addrs) > unix.RTAX_NETMASK && m.Addrs[unix.RTAX_NETMASK] != nil && !zeroAddr(m.Addrs[unix.RTAX_NETMASK]) {
			continue
		}
		if m.Flags&unix.RTF_GATEWAY != 0 {
			return m.Index, nil
		}
		// A /0 without a gateway is point-to-point and only used when no gateway default exists.
		if !found {
			index, found = m.Index, true
		}
	}
	if !found {
		return 0, ErrNoDefaultRoute
	}
	return index, nil
}

func zeroAddr(a route.Addr) bool {
	switch a := a.(type) {
	case *route.Inet4Addr:
		return a.IP == [4]byte{}
	case *route.Inet6Addr:
		return a.IP == [16]byte{}
	}
	return false
}

var bindDefault bindFunc = func(c syscall.RawConn, iface boundIface, ipv6 bool) error {
	level, opt := unix.IPPROTO_IP, unix.IP_BOUND_IF
	if ipv6 {
		level, opt = unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF
	}
	var err error
	if cerr := c.Control(func(fd uintptr) {
		err = unix.SetsockoptInt(int(fd), level, opt, iface.Index)
	}); cerr != nil {
		return cerr
	}
	return os.NewSyscallError("setsockopt", err)
}
