package tray

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime"
	"slices"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/tun"
	"github.com/wzshiming/tun/bypass"
	"github.com/wzshiming/tun/stack"
)

var newBypassDialer = bypass.NewDialer

// tunAddresses are the TUN device's own addresses with their subnets.
var tunAddresses = []netip.Prefix{netip.MustParsePrefix("198.18.0.1/30"), netip.MustParsePrefix("fd00:6a77:7761::1/64")}

const tunMTU = 1500

// tunName names the TUN interface: jumpway0 and up on Linux, the JumpWay adapter on Windows; macOS picks a utunN.
func tunName() string {
	if runtime.GOOS == "windows" {
		return "JumpWay"
	}
	return "jumpway"
}

// tunSupported reports whether this platform can open a TUN; tun.Open returns ErrUnsupported elsewhere.
var tunSupported = runtime.GOOS == "darwin" || runtime.GOOS == "linux" || runtime.GOOS == "windows"

// bypassBase is the first-hop base dialer while a TUN captures the destinations in capture.
func bypassBase(capture []netip.Prefix) *local.Local {
	if !tunSupported {
		return local.LOCAL
	}
	// A *local.Local keeps command:// hops working.
	base := *local.LOCAL
	base.Dialer = newBypassDialer(capture)
	return &base
}

// selectTUN serves the TUN from the named rule, or from none when name is empty, and restarts the rules it moves between.
func (a *App) selectTUN(name string) {
	a.proxyMu.Lock()
	if a.stopped {
		a.proxyMu.Unlock()
		return
	}
	a.mu.Lock()
	previous := a.tunRule
	a.tunRule = name
	a.mu.Unlock()
	// reload rebuilds the menu, which takes proxyMu again.
	a.proxyMu.Unlock()
	log.Info(i18n.TUN(), "rule", name)
	if err := a.reload(); errors.As(err, new(configError)) {
		// Nothing was applied, so a later reload must not pick up this selection.
		a.mu.Lock()
		a.tunRule = previous
		a.mu.Unlock()
		a.updateStatus()
	}
}

// syncTUNSelection clears a selection whose rule is no longer enabled or TUN-capable, and returns the effective one.
func (a *App) syncTUNSelection(rules []config.Rule) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tunRule == "" {
		return ""
	}
	for _, rule := range rules {
		if rule.Name == a.tunRule && !rule.Disabled && newRuleState(rule).tunCapable() {
			return a.tunRule
		}
	}
	log.Info("TUN rule removed", "rule", a.tunRule)
	a.tunRule = ""
	return ""
}

// serveTUN opens a TUN for the rule with open and serves it until ctx ends; the error explains an earlier stop.
func (a *App) serveTUN(ctx context.Context, open func(context.Context, tun.Config) (*tun.Device, error), rule config.Rule, state *ruleState, dialer bridge.Dialer, noProxy []string) error {
	dev, err := open(ctx, tunConfig(noProxy))
	if err != nil {
		message := i18n.TUNUnavailable()
		if errors.Is(err, tun.ErrDenied) {
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
	a.updateStatus()
	err = stack.Serve(ctx, dev, stack.DefaultMux(tunDialer{inner: dialer}), stack.Options{MTU: tunMTU, Addresses: tunAddresses})
	a.mu.Lock()
	state.tun = ""
	a.mu.Unlock()
	return fmt.Errorf("%s: %w", i18n.TUNUnavailable(), err)
}

type tunDialer struct{ inner bridge.Dialer }

// DialContext hands the captured flow's source to the rule dialer, so that metrics attribute it to its process.
func (d tunDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.inner.DialContext(jumpway.WithClientAddr(ctx, stack.ClientAddr(ctx)), network, address)
}

func tunConfig(noProxy []string) tun.Config {
	return tun.Config{Name: tunName(), Addresses: slices.Clone(tunAddresses), MTU: tunMTU, Bypass: bypassPrefixes(noProxy)}
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
