package jumpway

import (
	"context"
	"net"

	"github.com/wzshiming/bridge"
)

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
