package jumpway

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/protocols/local"
)

func NewListenConfig(ctx context.Context, way []config.Node) (bridge.ListenConfig, error) {
	if len(way) == 0 {
		return local.LOCAL, nil
	}
	for _, address := range way[0].LB {
		hop, err := url.Parse(address)
		if err != nil {
			return nil, fmt.Errorf("the first hop %q cannot listen: %w", address, err)
		}
		switch strings.ToLower(hop.Scheme) {
		case "socks4", "socks4a", "socks5", "socks5h":
			return nil, fmt.Errorf("the first hop %q cannot listen: SOCKS BIND is not a listener", address)
		}
	}
	// NewEnvDialer wraps the chain for NO_PROXY/ONLY_PROXY and loses ListenConfig.
	listenChain := *chain.Default
	listenChain.DialerFunc = nil
	dialer, err := listenChain.BridgeChainWithConfig(ctx, local.LOCAL, way...)
	if err != nil {
		return nil, err
	}
	listenConfig, ok := dialer.(bridge.ListenConfig)
	if !ok {
		return nil, fmt.Errorf("the first hop %q cannot listen", strings.Join(way[0].LB, "|"))
	}
	return listenConfig, nil
}
