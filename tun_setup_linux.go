package jumpway

import (
	"bytes"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

// tunRouteTable is the routing table number for TUN bypass routes.
const tunRouteTable = "100"

func configureTUN(name string, addr netip.Prefix) error {
	// Assign IP address to TUN device and bring it up.
	if err := exec.Command("ip", "addr", "add", addr.String(), "dev", name).Run(); err != nil {
		return fmt.Errorf("set address: %w", err)
	}
	if err := exec.Command("ip", "link", "set", "dev", name, "up").Run(); err != nil {
		return fmt.Errorf("set link up: %w", err)
	}

	// Discover the current default gateway so marked packets can bypass the TUN.
	origGW, origDev, err := defaultGateway()
	if err != nil {
		return fmt.Errorf("get default gateway: %w", err)
	}

	// Table 100: original default route for proxy's own outgoing traffic (fwmark'd).
	if err := exec.Command("ip", "route", "add", "default",
		"via", origGW, "dev", origDev, "table", tunRouteTable).Run(); err != nil {
		return fmt.Errorf("add bypass route table: %w", err)
	}

	// Rule: packets with our fwmark use the bypass table.
	if err := exec.Command("ip", "rule", "add",
		"fwmark", fmt.Sprintf("0x%x", tunFWMark),
		"lookup", tunRouteTable).Run(); err != nil {
		return fmt.Errorf("add fwmark rule: %w", err)
	}

	// Split routes through TUN that cover all IPv4 addresses.
	// 0.0.0.0/1 and 128.0.0.0/1 are more specific than the default 0.0.0.0/0
	// so they take priority, but the original default route is preserved.
	if err := exec.Command("ip", "route", "add", "0.0.0.0/1", "dev", name).Run(); err != nil {
		return fmt.Errorf("add split route 0/1: %w", err)
	}
	if err := exec.Command("ip", "route", "add", "128.0.0.0/1", "dev", name).Run(); err != nil {
		return fmt.Errorf("add split route 128/1: %w", err)
	}

	return nil
}

func unconfigureTUN(name string) {
	// Remove split routes.
	exec.Command("ip", "route", "del", "0.0.0.0/1", "dev", name).Run()
	exec.Command("ip", "route", "del", "128.0.0.0/1", "dev", name).Run()

	// Remove fwmark rule and bypass route table.
	exec.Command("ip", "rule", "del",
		"fwmark", fmt.Sprintf("0x%x", tunFWMark),
		"lookup", tunRouteTable).Run()
	exec.Command("ip", "route", "del", "default", "table", tunRouteTable).Run()

	// Bring the interface down.
	exec.Command("ip", "link", "set", "dev", name, "down").Run()
}

// defaultGateway returns the gateway IP and device of the current default route.
func defaultGateway() (gateway, device string, err error) {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return "", "", err
	}
	// Example output: "default via 192.168.1.1 dev eth0 proto dhcp metric 100"
	fields := strings.Fields(strings.TrimSpace(string(bytes.SplitN(out, []byte("\n"), 2)[0])))
	for i, f := range fields {
		switch f {
		case "via":
			if i+1 < len(fields) {
				gateway = fields[i+1]
			}
		case "dev":
			if i+1 < len(fields) {
				device = fields[i+1]
			}
		}
	}
	if gateway == "" || device == "" {
		return "", "", fmt.Errorf("could not parse default route from: %s", string(out))
	}
	return gateway, device, nil
}
