package metrics

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestConnDirection(t *testing.T) {
	for _, test := range []struct {
		name     string
		accepted bool
		up       int64
		down     int64
	}{
		{name: "accepted", accepted: true, up: 3, down: 5},
		{name: "dialed", up: 5, down: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			inner, peer := net.Pipe()
			closeOnCleanup(t, peer)
			count := &counter{}
			wrapped := newConn(inner, count, test.accepted)
			closeOnCleanup(t, wrapped)
			setDeadlines(t, inner, peer)
			done := make(chan error, 1)
			go func() {
				_, err := peer.Write([]byte("abc"))
				done <- err
			}()
			if _, err := io.ReadFull(wrapped, make([]byte, 3)); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			go func() {
				_, err := io.ReadFull(peer, make([]byte, 5))
				done <- err
			}()
			if _, err := wrapped.Write([]byte("hello")); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if up, down := count.up.Load(), count.down.Load(); up != test.up || down != test.down {
				t.Fatalf("bytes = (%d, %d), want (%d, %d)", up, down, test.up, test.down)
			}
			if count.active.Load() != 1 || count.total.Load() != 1 || count.lastActive.Load() == 0 {
				t.Fatal("connection event not recorded")
			}
			if _, ok := interface{}(wrapped).(io.ReaderFrom); ok {
				t.Fatal("wrapper must not implement io.ReaderFrom")
			}
			if _, ok := interface{}(wrapped).(io.WriterTo); ok {
				t.Fatal("wrapper must not implement io.WriterTo")
			}
			var closing sync.WaitGroup
			for range 8 {
				closing.Go(func() {
					if err := wrapped.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			closing.Wait()
			if err := wrapped.Close(); err != nil {
				t.Fatal(err)
			}
			if count.active.Load() != 0 || count.total.Load() != 1 {
				t.Fatal("repeated Close changed connection counts")
			}
		})
	}
}

func TestConnCloseWrite(t *testing.T) {
	t.Run("unsupported", func(t *testing.T) {
		inner, peer := net.Pipe()
		closeOnCleanup(t, peer)
		wrapped := newConn(inner, &counter{}, false)
		closeOnCleanup(t, wrapped)
		if err := wrapped.CloseWrite(); !errors.Is(err, errors.ErrUnsupported) {
			t.Fatalf("CloseWrite = %v, want ErrUnsupported", err)
		}
	})
	t.Run("tcp", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		closeOnCleanup(t, listener)
		inner, err := net.DialTimeout("tcp", listener.Addr().String(), 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		wrapped := newConn(inner, &counter{}, false)
		closeOnCleanup(t, wrapped)
		peer, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		closeOnCleanup(t, peer)
		setDeadlines(t, wrapped, peer)
		if err := wrapped.CloseWrite(); err != nil {
			t.Fatal(err)
		}
		if _, err := peer.Read(make([]byte, 1)); err != io.EOF {
			t.Fatalf("peer Read = %v, want EOF", err)
		}
		if _, err := peer.Write([]byte("ok")); err != nil {
			t.Fatal(err)
		}
		if _, err := io.ReadFull(wrapped, make([]byte, 2)); err != nil {
			t.Fatalf("read after CloseWrite: %v", err)
		}
	})
}

type pipeListener struct {
	connection net.Conn
	err        error
	closed     bool
}

func (l *pipeListener) Accept() (net.Conn, error) { return l.connection, l.err }
func (l *pipeListener) Addr() net.Addr            { return l.connection.LocalAddr() }
func (l *pipeListener) Close() error {
	l.closed = true
	return nil
}

func TestListener(t *testing.T) {
	inner, peer := net.Pipe()
	closeOnCleanup(t, peer)
	count := &counter{}
	base := &pipeListener{connection: inner}
	wrapped := &listener{Listener: base, count: count}
	if wrapped.Addr() != base.Addr() {
		t.Fatal("listener address was not forwarded")
	}
	accepted, err := wrapped.Accept()
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, accepted)
	if count.active.Load() != 1 || count.total.Load() != 1 || count.lastActive.Load() == 0 {
		t.Fatal("accept not counted")
	}
	if measured, ok := accepted.(*conn); !ok || !measured.accepted {
		t.Fatal("accepted connection has wrong direction")
	}
	if err := accepted.Close(); err != nil {
		t.Fatal(err)
	}
	if count.active.Load() != 0 {
		t.Fatal("accepted connection remains active after close")
	}
	base.err = net.ErrClosed
	if accepted, err := wrapped.Accept(); accepted != nil || !errors.Is(err, net.ErrClosed) {
		t.Fatalf("failed Accept = (%v, %v)", accepted, err)
	}
	if count.total.Load() != 1 {
		t.Fatal("failed accept changed total")
	}
	if err := wrapped.Close(); err != nil || !base.closed {
		t.Fatal("listener close was not forwarded")
	}
}

func closeOnCleanup(t *testing.T, closer io.Closer) {
	t.Helper()
	t.Cleanup(func() {
		if err := closer.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	})
}

func setDeadlines(t *testing.T, connections ...net.Conn) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for _, connection := range connections {
		if err := connection.SetDeadline(deadline); err != nil {
			t.Fatal(err)
		}
	}
}

func transferBytes(t *testing.T, writer, reader net.Conn, size int) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := writer.Write(make([]byte, size))
		done <- err
	}()
	if _, err := io.ReadFull(reader, make([]byte, size)); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
