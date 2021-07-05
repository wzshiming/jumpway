package jumpway

import (
	"context"
	"net"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/hostmatcher"
)

type shuntDialer struct {
	dialer      bridge.Dialer
	matchDialer bridge.Dialer
	matcher     hostmatcher.Matcher
}

func NewShuntDialer(dialer bridge.Dialer, matchDialer bridge.Dialer, matcher hostmatcher.Matcher) bridge.Dialer {
	if matcher == nil || matchDialer == nil {
		return dialer
	}
	return &shuntDialer{
		dialer:      dialer,
		matchDialer: matchDialer,
		matcher:     matcher,
	}
}

func (s *shuntDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if s.matcher.Match(address) {
		return s.matchDialer.DialContext(ctx, network, address)
	}
	return s.dialer.DialContext(ctx, network, address)
}

type logDialer struct {
	dialer bridge.Dialer
	log    func(ctx context.Context, network, address string)
}

func NewLogDialer(dialer bridge.Dialer, log func(ctx context.Context, network, address string)) bridge.Dialer {
	if log == nil {
		return dialer
	}
	return &logDialer{
		dialer: dialer,
		log:    log,
	}
}

func (l *logDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	l.log(ctx, network, address)
	return l.dialer.DialContext(ctx, network, address)
}
