package jumpway

import (
	"fmt"
	"net/netip"
	"os/exec"
)

func configureTUN(name string, addr netip.Prefix) error {
	ip := addr.Addr().String()
	prefixLen := fmt.Sprintf("%d", addr.Bits())
	if err := exec.Command("netsh", "interface", "ip", "set", "address",
		name, "static", ip, prefixLen).Run(); err != nil {
		return fmt.Errorf("set address: %w", err)
	}
	return nil
}

func unconfigureTUN(name string) {
	// On Windows, deleting the TUN device handles cleanup.
}
