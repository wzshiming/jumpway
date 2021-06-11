package jumpway

import (
	"context"
	"net"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/hostmatcher"
)

type noProxy struct {
	dialer  bridge.Dialer
	matcher hostmatcher.Matcher
}

func newNoProxy(dialer bridge.Dialer, matcher hostmatcher.Matcher) bridge.Dialer {
	if matcher == nil {
		return dialer
	}
	return &noProxy{
		dialer:  dialer,
		matcher: matcher,
	}
}

func (n *noProxy) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if n.matcher.Match(address) {
		return local.LOCAL.DialContext(ctx, network, address)
	}
	return n.dialer.DialContext(ctx, network, address)
}
