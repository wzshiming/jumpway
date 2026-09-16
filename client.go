package jumpway

import (
	"context"
	"net"
)

type clientAddrKey struct{}

func WithClientAddr(ctx context.Context, addr net.Addr) context.Context {
	var client string
	if addr != nil {
		client = addr.String()
	}
	return context.WithValue(ctx, clientAddrKey{}, client)
}

func ClientAddr(ctx context.Context) string {
	client, _ := ctx.Value(clientAddrKey{}).(string)
	return client
}
