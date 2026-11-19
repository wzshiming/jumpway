package jumpway

import (
	"context"
	"net/url"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/config"
)

// HopWrapper decorates the dialer bridge builds for one proxy URL; index is the hop's position in the way.
type HopWrapper func(index int, url string, dialer bridge.Dialer) bridge.Dialer

// NewChainDialer composes a chain with lazy per-URL wrapping.
// It does not apply chain.NewEnvDialer; callers decide whether to enable environment routing.
func NewChainDialer(ctx context.Context, base bridge.Dialer, way []config.Node, wrap HopWrapper) (bridge.Dialer, error) {
	if len(way) == 0 {
		return base, nil
	}
	plain := *chain.Default
	plain.DialerFunc = nil
	listen, _ := base.(bridge.ListenPacketConfig)
	dialer := base
	for index := len(way) - 1; index >= 0; index-- {
		node := chain.NewBridgeChain()
		node.DialerFunc = nil
		node.RegisterDefault(bridge.BridgeFunc(func(ctx context.Context, previous bridge.Dialer, address string) (bridge.Dialer, error) {
			if listen != nil && isSOCKS5(address) {
				// The relay socket is opened on the previous dialer, so every SOCKS5 hop gets base's.
				previous = packetDialer{Dialer: previous, ListenPacketConfig: listen}
			}
			hop, err := plain.BridgeChainWithConfig(ctx, previous, config.Node{LB: []string{address}})
			if err != nil {
				return nil, err
			}
			if wrap != nil {
				hop = wrap(index, address, hop)
			}
			return hop, nil
		}))
		var err error
		dialer, err = node.BridgeChainWithConfig(ctx, dialer, way[index])
		if err != nil {
			return nil, err
		}
	}
	return dialer, nil
}

func isSOCKS5(address string) bool {
	hop, err := url.Parse(address)
	if err != nil {
		return false
	}
	switch strings.ToLower(hop.Scheme) {
	case "socks5", "socks5h":
		return true
	}
	return false
}
