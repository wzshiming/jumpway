package jumpway

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestServeRelistensAfterServeReturns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	initial := time.Millisecond
	failure := errors.New("serve failed")
	var listeners []net.Listener
	var events []Event
	calls := 0
	err := serveWithBackoff(ctx, func(callCtx context.Context) (net.Listener, error) {
		if callCtx != ctx {
			t.Error("listen context was not forwarded")
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err == nil {
			listeners = append(listeners, listener)
			t.Cleanup(func() { listener.Close() })
		}
		return listener, err
	}, func(callCtx context.Context, listener net.Listener) error {
		if callCtx != ctx || listener != listeners[len(listeners)-1] {
			t.Error("serve arguments were not forwarded")
		}
		calls++
		if calls == 1 {
			return failure
		}
		timer := time.AfterFunc(initial, cancel)
		defer timer.Stop()
		<-callCtx.Done()
		return callCtx.Err()
	}, func(event Event) {
		events = append(events, event)
	}, initial, 4*initial)
	if err != ctx.Err() || !errors.Is(err, context.Canceled) {
		t.Fatalf("Serve() = %v, want context cancellation", err)
	}
	if calls != 2 || len(listeners) != 2 {
		t.Fatalf("serve calls = %d, listeners = %d, want 2 of each", calls, len(listeners))
	}
	if len(events) != 3 {
		t.Fatalf("events = %v, want listening, failed, listening", events)
	}
	if events[0] != (Event{Addr: listeners[0].Addr()}) || events[2] != (Event{Addr: listeners[1].Addr()}) {
		t.Fatalf("listening events = %v and %v, want listener addresses and reset retry fields", events[0], events[2])
	}
	if events[1] != (Event{Err: failure, Attempt: 1, Backoff: initial}) {
		t.Fatalf("failure event = %v, want attempt 1 and backoff %v", events[1], initial)
	}
	for _, listener := range listeners {
		conn, err := net.DialTimeout("tcp", listener.Addr().String(), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			t.Errorf("listener %s is still open", listener.Addr())
		}
	}
}

func TestServeRetriesInitialListen(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	initial := time.Millisecond
	maximum := 3 * initial
	failure := errors.New("listen failed")
	calls := 0
	var listener net.Listener
	var events []Event
	err := serveWithBackoff(ctx, func(callCtx context.Context) (net.Listener, error) {
		calls++
		if calls <= 3 {
			return nil, failure
		}
		var err error
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err == nil {
			t.Cleanup(func() { listener.Close() })
		}
		return listener, err
	}, func(callCtx context.Context, listener net.Listener) error {
		return nil
	}, func(event Event) {
		events = append(events, event)
		if len(events) == 5 {
			cancel()
		}
	}, initial, maximum)
	if err != ctx.Err() || !errors.Is(err, context.Canceled) {
		t.Fatalf("Serve() = %v, want context cancellation", err)
	}
	if calls != 4 || len(events) != 5 {
		t.Fatalf("listen calls = %d, events = %v, want 4 calls and 5 events", calls, events)
	}
	for index, wantBackoff := range []time.Duration{initial, 2 * initial, maximum} {
		if events[index] != (Event{Err: failure, Attempt: index + 1, Backoff: wantBackoff}) {
			t.Errorf("event %d = %v, want attempt %d and backoff %v", index, events[index], index+1, wantBackoff)
		}
	}
	if events[3] != (Event{Addr: listener.Addr()}) {
		t.Errorf("listening event = %v, want listener address and reset retry fields", events[3])
	}
	if events[4].Addr != nil || events[4].Err == nil || events[4].Attempt != 1 || events[4].Backoff != initial {
		t.Errorf("failure after nil serve return = %v, want error, attempt 1 and backoff %v", events[4], initial)
	}
}

func TestServeCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failure := errors.New("listen failed")
	failures := make(chan Event, 1)
	done := make(chan error, 1)
	calls := 0
	go func() {
		done <- serveWithBackoff(ctx, func(callCtx context.Context) (net.Listener, error) {
			calls++
			return nil, failure
		}, func(callCtx context.Context, listener net.Listener) error {
			return errors.New("unexpected serve call")
		}, func(event Event) {
			failures <- event
		}, 10*time.Second, 10*time.Second)
	}()
	select {
	case event := <-failures:
		if event != (Event{Err: failure, Attempt: 1, Backoff: 10 * time.Second}) {
			t.Fatalf("failure event = %v, want attempt 1 and 10s backoff", event)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not report the initial listen failure")
	}
	timer := time.AfterFunc(10*time.Millisecond, cancel)
	defer timer.Stop()
	select {
	case err := <-done:
		if err != ctx.Err() || !errors.Is(err, context.Canceled) {
			t.Fatalf("Serve() = %v, want context cancellation", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Serve did not cancel its backoff within 100ms")
	}
	if calls != 1 {
		t.Errorf("listen calls = %d, want 1", calls)
	}
}

func TestServeClosesListenerOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	started := make(chan struct{})
	accepted := make(chan error, 1)
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, func(callCtx context.Context) (net.Listener, error) {
			return listener, nil
		}, func(callCtx context.Context, listener net.Listener) error {
			close(started)
			conn, err := listener.Accept()
			if conn != nil {
				conn.Close()
			}
			accepted <- err
			return err
		}, nil)
	}()
	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("Serve did not start accepting connections")
	}
	select {
	case err := <-done:
		if err != ctx.Err() || !errors.Is(err, context.Canceled) {
			t.Fatalf("Serve() = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not stop after cancellation")
	}
	if err := <-accepted; !errors.Is(err, net.ErrClosed) {
		t.Errorf("Accept() = %v, want closed listener error", err)
	}
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("listener is still open after cancellation")
	}
}
