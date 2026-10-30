// Package tundev opens a configured local TUN device, obtaining it from a privileged helper when the process is not root.
package tundev

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"go4.org/netipx"
	"golang.zx2c4.com/wireguard/tun"
)

// Config describes the TUN to open.
type Config struct {
	// Addresses are the TUN's own addresses with their subnets: one or two, at most one per family.
	Addresses []netip.Prefix
	// MTU is the TUN's MTU, 1280 to 65535.
	MTU int
	// Bypass lists the destinations left to the existing routes.
	Bypass []netip.Prefix
}

// Device is an open, configured TUN.
type Device struct {
	tun.Device
	// Interface is the TUN's interface name.
	Interface string
	// Warnings describe the configuration that failed without failing Open.
	Warnings []string
}

// ErrDenied reports that the user declined the administrator prompt.
var ErrDenied = errors.New("tundev: administrator privileges denied")

// ErrUnsupported reports a platform without TUN support.
var ErrUnsupported = errors.ErrUnsupported

// fixedHoles are the destinations never routed into the TUN.
var fixedHoles = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("255.255.255.255/32"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

func validate(cfg Config) error {
	if n := len(cfg.Addresses); n < 1 || n > 2 {
		return fmt.Errorf("tundev: want 1 or 2 addresses, got %d", n)
	}
	for _, p := range cfg.Addresses {
		a := p.Addr()
		switch {
		case !p.IsValid():
			return fmt.Errorf("tundev: invalid address %v", p)
		case a.Is4In6():
			return fmt.Errorf("tundev: address %v is IPv4-mapped", p)
		case a.IsLoopback(), a.IsMulticast(), a.IsUnspecified(), a.IsLinkLocalUnicast():
			return fmt.Errorf("tundev: address %v is loopback, multicast, unspecified or link-local", p)
		case a == p.Masked().Addr():
			return fmt.Errorf("tundev: address %v has no host bits", p)
		}
	}
	if len(cfg.Addresses) == 2 && cfg.Addresses[0].Addr().Is4() == cfg.Addresses[1].Addr().Is4() {
		return fmt.Errorf("tundev: addresses %v and %v share a family", cfg.Addresses[0], cfg.Addresses[1])
	}
	if cfg.MTU < 1280 || cfg.MTU > 65535 {
		return fmt.Errorf("tundev: MTU %d outside 1280..65535", cfg.MTU)
	}
	for _, p := range cfg.Bypass {
		if !p.IsValid() {
			return fmt.Errorf("tundev: invalid bypass prefix %v", p)
		}
	}
	return nil
}

// CaptureRoutes returns the routes into the TUN: for each family with an address, all of it except Bypass, the TUN's own subnets and fixedHoles; sorted, IPv4 first.
func CaptureRoutes(cfg Config) []netip.Prefix {
	var b netipx.IPSetBuilder
	for _, p := range cfg.Addresses {
		switch {
		case p.Addr().Is4():
			b.AddPrefix(netip.PrefixFrom(netip.IPv4Unspecified(), 0))
		case p.Addr().Is6():
			b.AddPrefix(netip.PrefixFrom(netip.IPv6Unspecified(), 0))
		}
	}
	for _, p := range slices.Concat(cfg.Bypass, cfg.Addresses, fixedHoles) {
		b.RemovePrefix(p.Masked())
	}
	// Invalid prefixes are only recorded as errors, which validate reports instead.
	set, _ := b.IPSet()
	return set.Prefixes()
}
