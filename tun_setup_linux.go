package jumpway

import (
	"fmt"
	"net/netip"
	"os/exec"
)

func configureTUN(name string, addr netip.Prefix) error {
	if err := exec.Command("ip", "addr", "add", addr.String(), "dev", name).Run(); err != nil {
		return fmt.Errorf("set address: %w", err)
	}
	if err := exec.Command("ip", "link", "set", "dev", name, "up").Run(); err != nil {
		return fmt.Errorf("set link up: %w", err)
	}
	return nil
}

func unconfigureTUN(name string) {
	exec.Command("ip", "link", "set", "dev", name, "down").Run()
}
