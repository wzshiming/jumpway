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
	prefixLen := addr.Bits()
	// Calculate a peer address for the point-to-point link.
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
	peerStr := peer.String()

	if err := exec.Command("ifconfig", name, "inet", ip, peerStr, "netmask",
		prefixLenToNetmask(prefixLen), "up").Run(); err != nil {
		return fmt.Errorf("set address: %w", err)
	}

	// Add host routes for proxy server IPs via the original gateway so
	// the proxy's own connections bypass the TUN split routes.
	if len(bypassAddrs) > 0 {
		origGW, err := defaultGatewayDarwin()
		if err != nil {
			return fmt.Errorf("get default gateway: %w", err)
		}
		for _, addr := range bypassAddrs {
			if err := exec.Command("route", "add", "-host", addr.String(), origGW).Run(); err != nil {
				return fmt.Errorf("add bypass route for %s: %w", addr, err)
			}
		}
	}

	// Split routes through the TUN device. These are more specific than
	// the default 0.0.0.0/0 route so they capture all traffic.
	if err := exec.Command("route", "add", "-net", "0.0.0.0/1", peerStr).Run(); err != nil {
		return fmt.Errorf("add split route 0/1: %w", err)
	}
	if err := exec.Command("route", "add", "-net", "128.0.0.0/1", peerStr).Run(); err != nil {
		return fmt.Errorf("add split route 128/1: %w", err)
	}

	return nil
}

func unconfigureTUN(name string, bypassAddrs []netip.Addr) {
	// Remove split routes.
	exec.Command("route", "delete", "-net", "0.0.0.0/1").Run()
	exec.Command("route", "delete", "-net", "128.0.0.0/1").Run()

	// Remove bypass host routes.
	for _, addr := range bypassAddrs {
		exec.Command("route", "delete", "-host", addr.String()).Run()
	}

	exec.Command("ifconfig", name, "down").Run()
}

// defaultGatewayDarwin returns the current default gateway IP.
func defaultGatewayDarwin() (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "", err
	}
	for _, line := range bytes.Split(out, []byte("\n")) {
		s := strings.TrimSpace(string(line))
		if strings.HasPrefix(s, "gateway:") {
			return strings.TrimSpace(strings.TrimPrefix(s, "gateway:")), nil
		}
	}
	return "", fmt.Errorf("could not parse default gateway")
}

func prefixLenToNetmask(bits int) string {
	if bits <= 0 {
		bits = 0
	} else if bits > 32 {
		bits = 32
	}
	mask := uint32(0xFFFFFFFF) << (32 - bits)
	parts := []string{
		fmt.Sprintf("%d", (mask>>24)&0xFF),
		fmt.Sprintf("%d", (mask>>16)&0xFF),
		fmt.Sprintf("%d", (mask>>8)&0xFF),
		fmt.Sprintf("%d", mask&0xFF),
	}
	return strings.Join(parts, ".")
}
