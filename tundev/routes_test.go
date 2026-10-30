package tundev

import (
	"net/netip"
	"slices"
	"testing"

	"go4.org/netipx"
)

// ttHoles are the ranges no capture may ever route into the TUN.
var ttHoles = ttPrefixes("127.0.0.0/8", "169.254.0.0/16", "224.0.0.0/4", "255.255.255.255/32", "::1/128", "fe80::/10", "ff00::/8")

func ttSet(t *testing.T, add, remove []netip.Prefix) *netipx.IPSet {
	t.Helper()
	var b netipx.IPSetBuilder
	for _, p := range add {
		b.AddPrefix(p)
	}
	for _, p := range remove {
		b.RemovePrefix(p)
	}
	set, err := b.IPSet()
	if err != nil {
		t.Fatalf("IP set of %v minus %v: %v", add, remove, err)
	}
	return set
}

func ttSplit(routes []netip.Prefix) (v4, v6 []netip.Prefix) {
	for _, p := range routes {
		if p.Addr().Is4() {
			v4 = append(v4, p)
		} else {
			v6 = append(v6, p)
		}
	}
	return v4, v6
}

func TestCaptureRoutesCoverEachFamilyMinusBypassSubnetsAndHoles(t *testing.T) {
	v4, v6 := ttSplit(CaptureRoutes(ttDefaultConfig()))
	bypass := ttPrefixes("192.168.0.0/16", "10.0.0.0/8", "fe80::/64", "fd00::/8")
	ownSubnets := ttPrefixes("198.18.0.0/30", "fd00:6a77:7761::/64")
	carved := slices.Concat(bypass, ownSubnets, ttHoles)
	for _, tc := range []struct {
		family string
		routes []netip.Prefix
		all    netip.Prefix
	}{
		{"IPv4", v4, netip.MustParsePrefix("0.0.0.0/0")},
		{"IPv6", v6, netip.MustParsePrefix("::/0")},
	} {
		want := ttSet(t, []netip.Prefix{tc.all}, carved)
		if got := ttSet(t, tc.routes, nil); !got.Equal(want) {
			t.Errorf("%s routes = %v\nwant the set %v", tc.family, tc.routes, want.Prefixes())
		}
	}
}

func TestCaptureRoutesAreSortedDisjointMaskedPrefixes(t *testing.T) {
	routes := CaptureRoutes(ttDefaultConfig())
	if len(routes) == 0 {
		t.Fatal("CaptureRoutes returned no routes for the default config")
	}
	for i, p := range routes {
		if !p.IsValid() || p != p.Masked() {
			t.Errorf("route %d = %v, want a valid masked prefix", i, p)
		}
		if i > 0 && !routes[i-1].Addr().Less(p.Addr()) {
			t.Errorf("route %d = %v does not sort after route %d = %v; want IPv4 then IPv6, ascending", i, p, i-1, routes[i-1])
		}
		for j, q := range routes[:i] {
			if p.Overlaps(q) {
				t.Errorf("route %d = %v overlaps route %d = %v", i, p, j, q)
			}
		}
	}
}

func TestCaptureRoutesOmitFamilyWithoutAddress(t *testing.T) {
	for _, tc := range []struct {
		name    string
		address netip.Prefix
	}{
		{"IPv4 only", netip.MustParsePrefix("198.18.0.1/30")},
		{"IPv6 only", netip.MustParsePrefix("fd00:6a77:7761::1/64")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := ttDefaultConfig()
			cfg.Addresses = []netip.Prefix{tc.address}
			routes := CaptureRoutes(cfg)
			if len(routes) == 0 {
				t.Fatalf("CaptureRoutes with only %v returned no routes", tc.address)
			}
			for _, p := range routes {
				if p.Addr().Is4() != tc.address.Addr().Is4() {
					t.Errorf("CaptureRoutes with only %v returned %v from the family without an address", tc.address, p)
				}
			}
		})
	}
}

func TestCaptureRoutesBypassingAllIPv4KeepsOnlyIPv6(t *testing.T) {
	cfg := ttDefaultConfig()
	cfg.Bypass = append(cfg.Bypass, netip.MustParsePrefix("0.0.0.0/0"))
	v4, v6 := ttSplit(CaptureRoutes(cfg))
	if len(v4) != 0 {
		t.Errorf("IPv4 routes = %v, want none when 0.0.0.0/0 is bypassed", v4)
	}
	if len(v6) == 0 {
		t.Error("no IPv6 routes; bypassing 0.0.0.0/0 must leave IPv6 captured")
	}
}
