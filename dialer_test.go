package jumpway

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
)

func TestRetryDialerSucceedsAfterFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	failures := []error{errors.New("first failure"), errors.New("second failure")}
	calls := 0
	var attempts []int
	var retryErrors []error
	dialer := NewRetryDialer(bridge.DialFunc(func(callCtx context.Context, network, address string) (net.Conn, error) {
		if callCtx != ctx || network != "tcp" || address != "example.com:443" {
			t.Error("dial arguments were not forwarded")
		}
		calls++
		if calls <= len(failures) {
			return nil, failures[calls-1]
		}
		return client, nil
	}), 3, time.Millisecond, func(retryCtx context.Context, network, address string, attempt int, err error) {
		if retryCtx != ctx || network != "tcp" || address != "example.com:443" {
			t.Error("retry arguments were not forwarded")
		}
		attempts = append(attempts, attempt)
		retryErrors = append(retryErrors, err)
	})

	conn, err := dialer.DialContext(ctx, "tcp", "example.com:443")
	if err != nil || conn != client {
		t.Fatalf("DialContext = (%v, %v), want client connection and nil error", conn, err)
	}
	if calls != 3 {
		t.Errorf("dial calls = %d, want 3", calls)
	}
	if !slices.Equal(attempts, []int{1, 2}) {
		t.Errorf("retry attempts = %v, want [1 2]", attempts)
	}
	if !slices.Equal(retryErrors, failures) {
		t.Errorf("retry errors = %v, want %v", retryErrors, failures)
	}
}

func TestRetryDialerExhaustsRetries(t *testing.T) {
	sentinel := errors.New("dial failed")
	calls := 0
	var attempts []int
	dialer := NewRetryDialer(bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		calls++
		return nil, sentinel
	}), 3, time.Millisecond, func(ctx context.Context, network, address string, attempt int, err error) {
		attempts = append(attempts, attempt)
		if err != sentinel {
			t.Errorf("retry error = %v, want %v", err, sentinel)
		}
	})

	conn, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")
	if conn != nil || !errors.Is(err, sentinel) {
		t.Fatalf("DialContext = (%v, %v), want nil connection and joined dial error", conn, err)
	}
	if calls != 4 {
		t.Errorf("dial calls = %d, want 4", calls)
	}
	if !slices.Equal(attempts, []int{1, 2, 3, 4}) {
		t.Errorf("retry attempts = %v, want [1 2 3 4]", attempts)
	}
}

func TestRetryDialerContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sentinel := errors.New("dial failed")
	calls := 0
	var attempts []int
	dialer := NewRetryDialer(bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		calls++
		return nil, sentinel
	}), 3, 10*time.Second, func(ctx context.Context, network, address string, attempt int, err error) {
		attempts = append(attempts, attempt)
		if attempt == 1 {
			cancel()
		}
	})

	started := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", "example.com:443")
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Errorf("canceled dial took %v, want less than 1s", elapsed)
	}
	if conn != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, sentinel) {
		t.Fatalf("DialContext = (%v, %v), want nil connection and joined cancellation and dial errors", conn, err)
	}
	if calls != 1 {
		t.Errorf("dial calls = %d, want 1", calls)
	}
	if !slices.Equal(attempts, []int{1}) {
		t.Errorf("retry attempts = %v, want [1]", attempts)
	}
}

func TestRetryDialerCanceledDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sentinel := errors.New("dial failed")
	calls := 0
	dialer := NewRetryDialer(bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		calls++
		return nil, sentinel
	}), 3, 10*time.Second, nil)

	// Cancel while DialContext is parked in the backoff timer.
	time.AfterFunc(20*time.Millisecond, cancel)
	started := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", "example.com:443")
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Errorf("canceled dial took %v, want less than 1s", elapsed)
	}
	if conn != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, sentinel) {
		t.Fatalf("DialContext = (%v, %v), want nil connection and joined cancellation and dial errors", conn, err)
	}
	if calls != 1 {
		t.Errorf("dial calls = %d, want 1", calls)
	}
}

func TestRetryDialerDoesNotDuplicateContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dialer := NewRetryDialer(bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, ctx.Err()
	}), 3, time.Millisecond, nil)

	_, err := dialer.DialContext(ctx, "tcp", "example.com:443")
	if err == nil || err.Error() != context.Canceled.Error() {
		t.Fatalf("error = %q, want exactly %q", err, context.Canceled)
	}
}

func TestRetryDialerNilOnRetry(t *testing.T) {
	sentinel := errors.New("dial failed")
	calls := 0
	dialer := NewRetryDialer(bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		calls++
		return nil, sentinel
	}), 3, time.Millisecond, nil)

	conn, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")
	if conn != nil || !errors.Is(err, sentinel) {
		t.Fatalf("DialContext = (%v, %v), want nil connection and joined dial error", conn, err)
	}
	if calls != 4 {
		t.Errorf("dial calls = %d, want 4", calls)
	}
}

func TestRetryDialerNonPositiveRetries(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		retries int
	}{
		{name: "zero", retries: 0},
		{name: "negative", retries: -1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			sentinel := errors.New("dial failed")
			calls := 0
			var attempts []int
			dialer := NewRetryDialer(bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
				calls++
				return nil, sentinel
			}), testCase.retries, time.Millisecond, func(ctx context.Context, network, address string, attempt int, err error) {
				attempts = append(attempts, attempt)
				if err != sentinel {
					t.Errorf("retry error = %v, want %v", err, sentinel)
				}
			})

			conn, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")
			if conn != nil || !errors.Is(err, sentinel) {
				t.Fatalf("DialContext = (%v, %v), want nil connection and joined dial error", conn, err)
			}
			if calls != 1 {
				t.Errorf("dial calls = %d, want 1", calls)
			}
			if !slices.Equal(attempts, []int{1}) {
				t.Errorf("retry attempts = %v, want [1]", attempts)
			}
		})
	}
}
