package jumpway

import (
	"context"
	"errors"
	"net"
	"time"
)

const (
	DefaultListenBackoff = 100 * time.Millisecond
	MaxListenBackoff     = 30 * time.Second
)

// Event reports a listen address or failure, with consecutive failures and the wait before retrying.
type Event struct {
	Addr    net.Addr
	Err     error
	Attempt int
	Backoff time.Duration
}

// Serve listens, serves, and relistens until cancellation, returning ctx.Err(); report may be nil.
func Serve(ctx context.Context, listen func(ctx context.Context) (net.Listener, error), serve func(ctx context.Context, listener net.Listener) error, report func(Event)) error {
	return serveWithBackoff(ctx, listen, serve, report, DefaultListenBackoff, MaxListenBackoff)
}

func serveWithBackoff(ctx context.Context, listen func(ctx context.Context) (net.Listener, error), serve func(ctx context.Context, listener net.Listener) error, report func(Event), initial, maximum time.Duration) error {
	backoff := initial
	attempt := 0
	for ctx.Err() == nil {
		listener, err := listen(ctx)
		if err == nil {
			backoff = initial
			attempt = 0
			if report != nil {
				report(Event{Addr: listener.Addr()})
			}
			stop := context.AfterFunc(ctx, func() { listener.Close() })
			err = serve(ctx, listener)
			stop()
			listener.Close()
			if err == nil {
				err = errors.New("listener closed")
			}
		} else if listener != nil {
			listener.Close()
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		attempt++
		if report != nil {
			report(Event{Err: err, Attempt: attempt, Backoff: backoff})
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			timer.Stop()
		}
		backoff = min(backoff*2, maximum)
	}
	return ctx.Err()
}
