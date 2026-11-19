package jumpway

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/wzshiming/bridge"
)

const (
	DefaultDialRetries = 3
	DefaultDialBackoff = 100 * time.Millisecond
)

type logDialer struct {
	dialer bridge.Dialer
	log    func(ctx context.Context, network, address string)
}

// NewLogDialer logs each dial; it keeps the dialer's ListenPacket, which is not logged.
func NewLogDialer(dialer bridge.Dialer, log func(ctx context.Context, network, address string)) bridge.Dialer {
	if log == nil {
		return dialer
	}
	logged := &logDialer{
		dialer: dialer,
		log:    log,
	}
	if listen, ok := dialer.(bridge.ListenPacketConfig); ok {
		return packetDialer{Dialer: logged, ListenPacketConfig: listen}
	}
	return logged
}

// packetDialer dials through Dialer and listens for datagrams through ListenPacketConfig.
type packetDialer struct {
	bridge.Dialer
	bridge.ListenPacketConfig
}

func (l *logDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	l.log(ctx, network, address)
	return l.dialer.DialContext(ctx, network, address)
}

type retryDialer struct {
	dialer  bridge.Dialer
	retries int
	backoff time.Duration
	onRetry func(ctx context.Context, network, address string, attempt int, err error)
}

// NewRetryDialer retries failed dials up to retries times with exponential backoff, but never an unsupported one.
func NewRetryDialer(dialer bridge.Dialer, retries int, backoff time.Duration, onRetry func(ctx context.Context, network, address string, attempt int, err error)) bridge.Dialer {
	return &retryDialer{
		dialer:  dialer,
		retries: retries,
		backoff: backoff,
		onRetry: onRetry,
	}
}

func (r *retryDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var errs []error
	backoff := r.backoff
	for attempt := 1; ; attempt++ {
		conn, err := r.dialer.DialContext(ctx, network, address)
		if err == nil {
			return conn, nil
		}
		errs = append(errs, err)
		if r.onRetry != nil {
			r.onRetry(ctx, network, address, attempt, err)
		}
		if ctx.Err() != nil {
			return nil, joinContextErr(errs, ctx.Err())
		}
		if attempt > r.retries || errors.Is(err, errors.ErrUnsupported) {
			return nil, errors.Join(errs...)
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, joinContextErr(errs, ctx.Err())
		case <-timer.C:
		}
		backoff *= 2
	}
}

// joinContextErr appends ctxErr unless the last dial error already wraps it.
func joinContextErr(errs []error, ctxErr error) error {
	if !errors.Is(errs[len(errs)-1], ctxErr) {
		errs = append(errs, ctxErr)
	}
	return errors.Join(errs...)
}
