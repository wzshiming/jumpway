package tundev

import (
	"net/netip"
	"testing"
)

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		edit    func(*Config)
		wantErr bool
	}{
		{"rejects no addresses", func(c *Config) { c.Addresses = nil }, true},
		{"rejects three addresses", func(c *Config) {
			c.Addresses = ttPrefixes("198.18.0.1/30", "fd00:6a77:7761::1/64", "198.18.0.5/30")
		}, true},
		{"rejects two IPv4 addresses", func(c *Config) { c.Addresses = ttPrefixes("198.18.0.1/30", "198.18.0.5/30") }, true},
		{"rejects an address without host bits", func(c *Config) { c.Addresses = ttPrefixes("198.18.0.0/30") }, true},
		{"rejects an IPv4-mapped IPv6 address", func(c *Config) { c.Addresses = ttPrefixes("::ffff:1.2.3.4/120") }, true},
		{"rejects a loopback address", func(c *Config) { c.Addresses = ttPrefixes("127.0.0.1/8") }, true},
		{"rejects a multicast address", func(c *Config) { c.Addresses = ttPrefixes("224.0.0.1/4") }, true},
		{"rejects the unspecified address", func(c *Config) { c.Addresses = ttPrefixes("0.0.0.0/0") }, true},
		{"rejects a link-local address", func(c *Config) { c.Addresses = ttPrefixes("fe80::1/64") }, true},
		{"rejects MTU 1279", func(c *Config) { c.MTU = 1279 }, true},
		{"rejects MTU 65536", func(c *Config) { c.MTU = 65536 }, true},
		{"accepts MTU 1280 with an unmasked bypass entry", func(c *Config) {
			c.MTU = 1280
			c.Bypass = append(c.Bypass, netip.MustParsePrefix("172.16.1.1/12"))
		}, false},
		{"accepts MTU 65535 with an unmasked bypass entry", func(c *Config) {
			c.MTU = 65535
			c.Bypass = append(c.Bypass, netip.MustParsePrefix("172.16.1.1/12"))
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := ttDefaultConfig()
			tc.edit(&cfg)
			err := validate(cfg)
			if tc.wantErr && err == nil {
				t.Errorf("validate accepted %+v, want an error", cfg)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validate(%+v) = %v, want nil", cfg, err)
			}
		})
	}
}
