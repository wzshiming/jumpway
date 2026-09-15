package jumpway

import (
	"context"

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
	dialer := base
	for index := len(way) - 1; index >= 0; index-- {
		node := chain.NewBridgeChain()
		node.DialerFunc = nil
		node.RegisterDefault(bridge.BridgeFunc(func(ctx context.Context, dialer bridge.Dialer, address string) (bridge.Dialer, error) {
			dialer, err := plain.BridgeChainWithConfig(ctx, dialer, config.Node{LB: []string{address}})
			if err != nil {
				return nil, err
			}
			if wrap != nil {
				dialer = wrap(index, address, dialer)
			}
			return dialer, nil
		}))
		var err error
		dialer, err = node.BridgeChainWithConfig(ctx, dialer, way[index])
		if err != nil {
			return nil, err
		}
	}
	return dialer, nil
}
