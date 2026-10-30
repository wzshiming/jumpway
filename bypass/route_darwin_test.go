//go:build darwin

package bypass

import (
	"context"
	"errors"
	"net/netip"
	"syscall"
	"testing"
	"time"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

const (
	ttFlagsGateway = syscall.RTF_UP | syscall.RTF_GATEWAY | syscall.RTF_STATIC
	ttFlagsScoped  = ttFlagsGateway | syscall.RTF_IFSCOPE
	ttFlagsHost    = ttFlagsGateway | syscall.RTF_HOST
	ttFlagsDown    = ttFlagsGateway &^ syscall.RTF_UP
	ttFlagsLink    = syscall.RTF_UP | syscall.RTF_STATIC
)

// ttRouteMsg shapes a RouteMessage like route.ParseRIB output, with Addrs indexed by RTAX_*.
func ttRouteMsg(flags, index int, dst, gateway, netmask route.Addr) *route.RouteMessage {
	addrs := make([]route.Addr, syscall.RTAX_MAX)
	addrs[syscall.RTAX_DST] = dst
	addrs[syscall.RTAX_GATEWAY] = gateway
	addrs[syscall.RTAX_NETMASK] = netmask
	return &route.RouteMessage{Version: syscall.RTM_VERSION, Type: syscall.RTM_GET, Flags: flags, Index: index, Addrs: addrs}
}

func ttIn4(a, b, c, d byte) *route.Inet4Addr { return &route.Inet4Addr{IP: [4]byte{a, b, c, d}} }

func ttIn6(s string) *route.Inet6Addr { return &route.Inet6Addr{IP: netip.MustParseAddr(s).As16()} }

// ttDefault4 is 0.0.0.0/0 via 192.168.1.1 on index, with the all-zero netmask ParseRIB yields.
func ttDefault4(flags, index int) *route.RouteMessage {
	return ttRouteMsg(flags, index, ttIn4(0, 0, 0, 0), ttIn4(192, 168, 1, 1), ttIn4(0, 0, 0, 0))
}

// ttDefault6 is ::/0 via fe80::1 on index, with the all-zero netmask ParseRIB yields.
func ttDefault6(flags, index int) *route.RouteMessage {
	gateway := &route.Inet6Addr{IP: netip.MustParseAddr("fe80::1").As16(), ZoneID: index}
	return ttRouteMsg(flags, index, ttIn6("::"), gateway, ttIn6("::"))
}

// ttPointToPoint4 is a gatewayless 0.0.0.0/0 on a tunnel interface.
func ttPointToPoint4(index int) *route.RouteMessage {
	return ttRouteMsg(ttFlagsLink, index, ttIn4(0, 0, 0, 0), &route.LinkAddr{Index: index}, ttIn4(0, 0, 0, 0))
}

func ttWantIndex(t *testing.T, msgs []route.Message, ipv6 bool, want int) {
	t.Helper()
	if got, err := defaultRouteIndex(msgs, ipv6); err != nil || got != want {
		t.Errorf("defaultRouteIndex(ipv6=%t) = (%d, %v), want (%d, nil)", ipv6, got, err, want)
	}
}

func TestDefaultRouteIndexPrefersUnscopedDefaultOverEarlierScopedOne(t *testing.T) {
	ttWantIndex(t, []route.Message{ttDefault4(ttFlagsScoped, 20), ttDefault4(ttFlagsGateway, 4)}, false, 4)
}

func TestDefaultRouteIndexIgnoresHostRoute(t *testing.T) {
	host := ttRouteMsg(ttFlagsHost, 9, ttIn4(0, 0, 0, 0), ttIn4(192, 168, 1, 1), nil)
	ttWantIndex(t, []route.Message{host, ttDefault4(ttFlagsGateway, 4)}, false, 4)
}

func TestDefaultRouteIndexAcceptsDefaultWithoutNetmask(t *testing.T) {
	noMask := ttRouteMsg(ttFlagsGateway, 4, ttIn4(0, 0, 0, 0), ttIn4(192, 168, 1, 1), nil)
	ttWantIndex(t, []route.Message{noMask}, false, 4)
}

func TestDefaultRouteIndexAcceptsLonePointToPointDefault(t *testing.T) {
	ttWantIndex(t, []route.Message{ttPointToPoint4(12)}, false, 12)
}

func TestDefaultRouteIndexPrefersGatewayDefaultOverEarlierPointToPoint(t *testing.T) {
	ttWantIndex(t, []route.Message{ttPointToPoint4(12), ttDefault4(ttFlagsGateway, 4)}, false, 4)
}

func TestDefaultRouteIndexIPv6PrefersUnscopedDefaultOverEarlierScopedOne(t *testing.T) {
	ttWantIndex(t, []route.Message{ttDefault6(ttFlagsScoped, 20), ttDefault6(ttFlagsGateway, 4)}, true, 4)
}

func TestDefaultRouteIndexWithoutDefaultReturnsErrNoDefaultRoute(t *testing.T) {
	split := func(dst *route.Inet4Addr) *route.RouteMessage {
		return ttRouteMsg(ttFlagsLink, 12, dst, &route.LinkAddr{Index: 12}, ttIn4(128, 0, 0, 0))
	}
	tests := []struct {
		name string
		ipv6 bool
		msgs []route.Message
	}{
		{"no routes", false, nil},
		{"subnet and 0/1 128/1 split routes only", false, []route.Message{
			ttRouteMsg(syscall.RTF_UP, 4, ttIn4(10, 0, 0, 0), &route.LinkAddr{Index: 4}, ttIn4(255, 0, 0, 0)),
			split(ttIn4(0, 0, 0, 0)),
			split(ttIn4(128, 0, 0, 0)),
		}},
		{"scoped default only", false, []route.Message{ttDefault4(ttFlagsScoped, 20)}},
		{"default not up", false, []route.Message{ttDefault4(ttFlagsDown, 4)}},
		{"IPv6 default only, asked for IPv4", false, []route.Message{ttDefault6(ttFlagsGateway, 4)}},
		{"IPv4 default only, asked for IPv6", true, []route.Message{ttDefault4(ttFlagsGateway, 4)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := defaultRouteIndex(tc.msgs, tc.ipv6); !errors.Is(err, ErrNoDefaultRoute) {
				t.Errorf("defaultRouteIndex(ipv6=%t) = (%d, %v), want ErrNoDefaultRoute", tc.ipv6, got, err)
			}
		})
	}
}

func TestNewDialerBindsUDPToIPv4DefaultInterface(t *testing.T) {
	iface, err := DefaultInterface(false)
	if errors.Is(err, ErrNoDefaultRoute) {
		t.Skip("no IPv4 default route on this host")
	}
	if err != nil {
		t.Fatalf("DefaultInterface(false) error = %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	// Connecting a UDP socket only records the peer; no packet leaves the host.
	conn, err := NewDialer(ttCaptureLab).DialContext(ctx, "udp4", "198.18.0.1:9")
	if err != nil {
		t.Fatalf("DialContext(udp4, 198.18.0.1:9) error = %v", err)
	}
	defer conn.Close()
	sc, ok := conn.(syscall.Conn)
	if !ok {
		t.Fatalf("%T is not a syscall.Conn", conn)
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn error = %v", err)
	}
	var bound int
	var optErr error
	if err := raw.Control(func(fd uintptr) {
		bound, optErr = unix.GetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF)
	}); err != nil {
		t.Fatalf("RawConn.Control error = %v", err)
	}
	if optErr != nil {
		t.Fatalf("getsockopt(IP_BOUND_IF) error = %v", optErr)
	}
	if bound != iface.Index {
		t.Errorf("IP_BOUND_IF = %d, want %d (%s holds the IPv4 default route)", bound, iface.Index, iface.Name)
	}
}
