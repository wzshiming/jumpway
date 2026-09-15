package metrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/wzshiming/bridge"
)

type dialer struct {
	inner bridge.Dialer
	count *counter
	url   string
}

func (d *dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if trace, ok := ctx.Value(traceKey{}).(*pathTrace); ok && d.url != "" {
		trace.via.CompareAndSwap(nil, &d.url)
	}
	started := time.Now()
	connected, err := d.inner.DialContext(ctx, network, address)
	now := time.Now()
	d.count.dial(now.Sub(started), now, err)
	if err != nil {
		return nil, err
	}
	return newConn(connected, d.count, false), nil
}

func (d *dialer) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	config, ok := d.inner.(bridge.ListenConfig)
	if !ok {
		return nil, fmt.Errorf("metrics: dialer does not support listening: %w", errors.ErrUnsupported)
	}
	listening, err := config.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return &listener{Listener: listening, count: d.count}, nil
}

type traceKey struct{}

type pathTrace struct {
	via atomic.Pointer[string]
}

type ruleDialer struct {
	inner bridge.Dialer
	rule  *Rule
}

func (r *Rule) WrapDialer(inner bridge.Dialer) bridge.Dialer {
	return &ruleDialer{inner: inner, rule: r}
}

func (d *ruleDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	trace := &pathTrace{}
	ctx = context.WithValue(ctx, traceKey{}, trace)
	started := time.Now()
	connected, err := d.inner.DialContext(ctx, network, address)
	now := time.Now()
	key := targetKey{address: address}
	if via := trace.via.Load(); via != nil {
		key.via = *via
	}
	d.rule.registry.mu.Lock()
	defer d.rule.registry.mu.Unlock()
	d.rule.count.dial(now.Sub(started), now, err)
	count := d.rule.target(key)
	count.dial(now.Sub(started), now, err)
	if err != nil {
		return nil, err
	}
	return newConn(connected, count, false), nil
}
