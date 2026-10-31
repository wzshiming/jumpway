package tray

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/bypass"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/jumpway/tundev"
	"github.com/wzshiming/jumpway/tunstack"
)

var newBypassDialer = bypass.NewDialer

// bypassBase is the first-hop base dialer while a TUN captures the destinations in capture.
func bypassBase(capture []netip.Prefix) *local.Local {
	// Only macOS and Linux open a TUN.
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return local.LOCAL
	}
	// A *local.Local keeps command:// hops working.
	base := *local.LOCAL
	base.Dialer = newBypassDialer(capture)
	return &base
}

// serveTUN opens the rule's TUN with open and serves it until ctx ends; the error explains an earlier stop.
func (a *App) serveTUN(ctx context.Context, open func(context.Context, tundev.Config) (*tundev.Device, error), rule config.Rule, state *ruleState, dialer bridge.Dialer, noProxy []string, report func(jumpway.Event)) error {
	row := rule.Listen.TUN()
	dev, err := open(ctx, tunConfig(*row, noProxy))
	if err != nil {
		message := i18n.TUNUnavailable()
		if errors.Is(err, tundev.ErrDenied) {
			message = i18n.TUNDenied()
		}
		return fmt.Errorf("%s: %w", message, err)
	}
	for _, warning := range dev.Warnings {
		log.Warn(warning, "rule", rule.Name)
	}
	a.mu.Lock()
	state.tun = dev.Interface
	a.mu.Unlock()
	if len(rule.Listen.Schemes()) == 0 {
		report(jumpway.Event{Addr: tunAddr(dev.Interface)})
	} else {
		a.updateStatus()
	}
	err = tunstack.Serve(ctx, dev, dialer, tunstack.Options{MTU: row.TUNMTU(), Addresses: row.TUNAddresses()})
	a.mu.Lock()
	state.tun = ""
	a.mu.Unlock()
	return fmt.Errorf("%s: %w", i18n.TUNUnavailable(), err)
}

func tunConfig(row config.Protocol, noProxy []string) tundev.Config {
	return tundev.Config{Addresses: row.TUNAddresses(), MTU: row.TUNMTU(), Bypass: bypassPrefixes(noProxy)}
}

// bypassPrefixes keeps the no-proxy entries that are IPs or CIDRs, as routes cannot match hostnames.
func bypassPrefixes(list []string) []netip.Prefix {
	var prefixes []netip.Prefix
	for _, entry := range list {
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			prefixes = append(prefixes, prefix)
		} else if addr, err := netip.ParseAddr(entry); err == nil {
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
		}
	}
	return prefixes
}

// tunInterface is the address a TUN half reports: its interface name.
type tunInterface string

func (tunInterface) Network() string { return config.ProtocolTUN }

func (i tunInterface) String() string { return string(i) }

func tunAddr(iface string) net.Addr {
	return tunInterface(iface)
}
