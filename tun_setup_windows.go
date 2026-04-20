package jumpway

import (
	"bytes"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

func configureTUN(name string, addr netip.Prefix, bypassAddrs []netip.Addr) error {
	ip := addr.Addr().String()
	mask := prefixLenToSubnetMask(addr.Bits())
	if err := exec.Command("netsh", "interface", "ip", "set", "address",
		name, "static", ip, mask).Run(); err != nil {
		return fmt.Errorf("set address: %w", err)
	}

	// Calculate TUN gateway (peer) address for routes.
	peer := addr.Addr()
	if peer.Is4() {
		b := peer.As4()
		if b[3] < 255 {
			b[3]++
		} else {
			b[3]--
		}
		peer = netip.AddrFrom4(b)
	}

	// Add host routes for proxy server IPs via the original gateway so
	// the proxy's own connections bypass the TUN split routes.
	if len(bypassAddrs) > 0 {
		origGW, err := defaultGatewayWindows()
		if err != nil {
			return fmt.Errorf("get default gateway: %w", err)
		}
		for _, addr := range bypassAddrs {
			if err := exec.Command("route", "add", addr.String(), "mask", "255.255.255.255",
				origGW, "metric", "5").Run(); err != nil {
				return fmt.Errorf("add bypass route for %s: %w", addr, err)
			}
		}
	}

	// Split routes through TUN that cover all IPv4 addresses.
	if err := exec.Command("route", "add", "0.0.0.0", "mask", "128.0.0.0",
		peer.String(), "metric", "5").Run(); err != nil {
		return fmt.Errorf("add split route 0/1: %w", err)
	}
	if err := exec.Command("route", "add", "128.0.0.0", "mask", "128.0.0.0",
		peer.String(), "metric", "5").Run(); err != nil {
		return fmt.Errorf("add split route 128/1: %w", err)
	}

	return nil
}

func unconfigureTUN(name string, bypassAddrs []netip.Addr) {
	// Remove split routes.
	exec.Command("route", "delete", "0.0.0.0", "mask", "128.0.0.0").Run()
	exec.Command("route", "delete", "128.0.0.0", "mask", "128.0.0.0").Run()

	// Remove bypass host routes.
	for _, addr := range bypassAddrs {
		exec.Command("route", "delete", addr.String(), "mask", "255.255.255.255").Run()
	}
	// On Windows, deleting the TUN device handles interface cleanup.
}

// defaultGatewayWindows returns the current default gateway IP.
func defaultGatewayWindows() (string, error) {
	out, err := exec.Command("route", "print", "0.0.0.0").Output()
	if err != nil {
		return "", err
	}
	// Look for "0.0.0.0" in the routing table output.
	for _, line := range bytes.Split(out, []byte("\n")) {
		s := strings.TrimSpace(string(line))
		fields := strings.Fields(s)
		if len(fields) >= 3 && fields[0] == "0.0.0.0" && fields[1] == "0.0.0.0" {
			return fields[2], nil
		}
	}
	return "", fmt.Errorf("could not parse default gateway")
}

// prefixLenToSubnetMask converts a CIDR prefix length to a dotted subnet mask.
func prefixLenToSubnetMask(bits int) string {
	if bits <= 0 {
		bits = 0
	} else if bits > 32 {
		bits = 32
	}
	mask := uint32(0xFFFFFFFF) << (32 - bits)
	return fmt.Sprintf("%d.%d.%d.%d",
		(mask>>24)&0xFF, (mask>>16)&0xFF, (mask>>8)&0xFF, mask&0xFF)
}
