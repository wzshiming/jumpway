package jumpway

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/protocols/local"
)

func NewListenConfig(ctx context.Context, way []config.Node, wrap HopWrapper) (bridge.ListenConfig, error) {
	if len(way) == 0 {
		return local.LOCAL, nil
	}
	for _, address := range way[0].LB {
		hop, err := url.Parse(address)
		if err != nil {
			return nil, fmt.Errorf("the first hop cannot listen: %w", err)
		}
		switch strings.ToLower(hop.Scheme) {
		case "socks4", "socks4a", "socks5", "socks5h":
			// SOCKS BIND accepts one connection per request and only fails at Accept time.
			return nil, fmt.Errorf("the first hop %q cannot listen: SOCKS BIND is not a listener", hop.Redacted())
		}
	}
	dialer, err := NewChainDialer(ctx, local.LOCAL, way, wrap)
	if err != nil {
		return nil, err
	}
	// The chain always implements Listen; hops that cannot bind fail inside Listen itself.
	listenConfig, ok := dialer.(bridge.ListenConfig)
	if !ok {
		return nil, fmt.Errorf("the first hop %T cannot listen", dialer)
	}
	return listenConfig, nil
}
