package tundev

import (
	"net/netip"
	"os"
	"path"
	"testing"
)

func ttDefaultConfig() Config {
	return Config{
		Addresses: ttPrefixes("198.18.0.1/30", "fd00:6a77:7761::1/64"),
		MTU:       1500,
		Bypass:    ttPrefixes("192.168.0.0/16", "10.0.0.0/8", "fe80::/64", "fd00::/8"),
	}
}

func ttPrefixes(s ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, len(s))
	for i, p := range s {
		prefixes[i] = netip.MustParsePrefix(p)
	}
	return prefixes
}

func TestDefaultToolsLinuxIPIsAbsoluteOrEmpty(t *testing.T) {
	ip := defaultTools("linux").ip
	if ip == "" {
		return
	}
	if !path.IsAbs(ip) {
		t.Fatalf("defaultTools(linux).ip = %q, want an absolute path or empty, never a name looked up in PATH", ip)
	}
	if _, err := os.Stat(ip); err != nil {
		t.Errorf("defaultTools(linux).ip = %q, want an existing file: %v", ip, err)
	}
}
