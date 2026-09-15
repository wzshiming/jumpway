package metrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/jumpway"
)

type dialer struct {
	inner bridge.Dialer
	count *counter
	url   string
	exit  bool
}

func (d *dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	started := time.Now()
	connected, err := d.inner.DialContext(ctx, network, address)
	now := time.Now()
	d.count.dial(now.Sub(started), now, err)
	// Bridge retries other LB URLs of the exit node inside one dial; the last attempt is the one that connected.
	if trace, ok := ctx.Value(traceKey{}).(*pathTrace); ok && d.exit {
		trace.via.Store(&d.url)
	}
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
	registry := d.rule.registry
	registry.mu.Lock()
	defer registry.mu.Unlock()
	d.rule.count.dial(now.Sub(started), now, err)
	count := d.rule.target(key)
	count.dial(now.Sub(started), now, err)
	if err != nil {
		return nil, err
	}
	wrapped := newConn(connected, count, false)
	if registry.rules[d.rule.name] != d.rule {
		return wrapped, nil
	}
	entry := &live{
		id:      registry.nextID.Add(1),
		rule:    d.rule,
		client:  jumpway.ClientAddr(ctx),
		target:  address,
		via:     key.via,
		started: now,
		count:   newCounter(now),
		conn:    wrapped,
	}
	wrapped.extra = entry.count
	registry.live[entry.id] = entry
	wrapped.onClose = func() {
		registry.mu.Lock()
		delete(registry.live, entry.id)
		registry.mu.Unlock()
	}
	return wrapped, nil
}
