package jumpway

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

func configureTUN(name string, addr netip.Prefix) error {
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

	if err := exec.Command("ifconfig", name, "inet", ip, peer.String(), "netmask",
		prefixLenToNetmask(prefixLen), "up").Run(); err != nil {
		return fmt.Errorf("set address: %w", err)
	}
	return nil
}

func unconfigureTUN(name string) {
	exec.Command("ifconfig", name, "down").Run()
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
