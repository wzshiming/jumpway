package tray

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/tunproxy"
	"github.com/wzshiming/tun/stack"
)

var newDirectUDP = tunproxy.Direct

// newUDPListenConfig opens the sockets of rule's UDP sessions by its forward.udp options: through dialer, its TCP
// dialer, for mode proxy; from policy's base for mode direct or without a way, accounted by account unless nil; or none.
func newUDPListenConfig(rule config.Rule, policy exit, dialer bridge.Dialer, account func(context.Context, net.PacketConn) net.PacketConn) stack.ListenConfig {
	options := rule.Forward.UDP
	idle := time.Duration(options.Idle) * time.Second
	switch {
	case options.Mode == config.UDPModeBlock:
		return udpBlocked(rule.Name)
	case options.Mode == config.UDPModeDirect || rule.Forward.Target() == "" && len(rule.Forward.Way) == 0:
		if options.NAT == config.NATSymmetric {
			return udpListenConfig{ListenConfig: tunproxy.Proxied(policy.base, tunproxy.ProxiedOptions{Symmetric: true, Idle: idle}), account: account}
		}
		return udpListenConfig{ListenConfig: newDirectUDP(policy.capture), account: account}
	case rule.Forward.Target() != "":
		return udpUnsupported(rule.Name)
	}
	// buildDialer shunts these rules per destination, so by default their UDP stays symmetric to follow it.
	symmetric := options.NAT == config.NATSymmetric ||
		options.NAT == "" && (chain.OnlyProxy != nil || chain.NoProxy != nil || noProxyMatchesCaptured(policy.noProxy))
	return udpListenConfig{ListenConfig: tunproxy.Proxied(dialer, tunproxy.ProxiedOptions{Symmetric: symmetric, Idle: idle})}
}

// noProxyMatchesCaptured reports whether a "*" or IP:port no_proxy entry can match a captured address.
func noProxyMatchesCaptured(list []string) bool {
	return slices.ContainsFunc(list, func(entry string) bool {
		entry = strings.TrimSpace(entry)
		if entry == "*" {
			return true
		}
		if strings.Contains(entry, "/") {
			return false
		}
		host, _, err := net.SplitHostPort(entry)
		if err != nil {
			return false
		}
		_, err = netip.ParseAddr(host)
		return err == nil
	})
}

// udpListenConfig gives each UDP session of a rule one socket from ListenConfig, accounted by account unless nil.
type udpListenConfig struct {
	stack.ListenConfig
	account func(context.Context, net.PacketConn) net.PacketConn
}

// ListenPacket hands a TUN session's client to the rule, so that metrics attribute it to its process; other consumers keep theirs.
func (l udpListenConfig) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	if client := stack.ClientAddr(ctx); client != nil {
		ctx = jumpway.WithClientAddr(ctx, client)
	}
	conn, err := l.ListenConfig.ListenPacket(ctx, network, address)
	if err != nil || l.account == nil {
		return conn, err
	}
	return l.account(ctx, conn), nil
}

// packetDialer dials through Dialer and opens UDP sockets through ListenPacketConfig.
type packetDialer struct {
	bridge.Dialer
	bridge.ListenPacketConfig
}

// udpUnsupported refuses the UDP sessions of the named rule, whose way cannot carry them.
type udpUnsupported string

func (udpUnsupported) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return nil, fmt.Errorf("tun: listen %s %s: %w", network, address, errors.ErrUnsupported)
}

func (name udpUnsupported) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	return nil, fmt.Errorf("rule %q cannot carry UDP sessions: %w", string(name), errors.ErrUnsupported)
}

// udpBlocked refuses the UDP sessions of the named rule, whose udp.mode is block, opening nothing.
type udpBlocked string

func (udpBlocked) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return nil, fmt.Errorf("tun: listen %s %s: %w", network, address, errors.ErrUnsupported)
}

func (name udpBlocked) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	return nil, fmt.Errorf("rule %q blocks UDP", string(name))
}
