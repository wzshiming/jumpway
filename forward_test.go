package jumpway

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/jumpway/utils"
)

func TestRunForwardEcho(t *testing.T) {
	target := startForwardTarget(t, func(conn net.Conn) { io.Copy(conn, conn) })
	listener := startForward(t, context.Background(), target)
	first := dialForward(t, listener.Addr().String())
	checkForwardEcho(t, first, "hello")
	second := dialForward(t, listener.Addr().String())
	checkForwardEcho(t, second, "hello")
	checkForwardEcho(t, first, "still connected")
}

func TestRunForwardHalfClose(t *testing.T) {
	for _, afterEOF := range []bool{false, true} {
		name := "echo"
		if afterEOF {
			name = "response_after_eof"
		}
		t.Run(name, func(t *testing.T) {
			target := startForwardTarget(t, func(conn net.Conn) {
				if !afterEOF {
					io.Copy(conn, conn)
					return
				}
				request, err := io.ReadAll(conn)
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := conn.Write(request); err != nil {
					t.Error(err)
				}
			})
			listener := startForward(t, context.Background(), target)
			client := dialForward(t, listener.Addr().String())
			payload := strings.Repeat("half-close", 32*1024)
			if _, err := io.WriteString(client, payload); err != nil {
				t.Fatal(err)
			}
			if err := client.CloseWrite(); err != nil {
				t.Fatal(err)
			}
			response, err := io.ReadAll(client)
			if err != nil {
				t.Fatal(err)
			}
			if string(response) != payload {
				t.Fatalf("response = %d bytes, want the original %d bytes", len(response), len(payload))
			}
		})
	}
	t.Run("target_closes", func(t *testing.T) {
		target := startForwardTarget(t, func(conn net.Conn) {})
		listener := startForward(t, context.Background(), target)
		client := dialForward(t, listener.Addr().String())
		count, err := client.Read(make([]byte, 1))
		if count != 0 || err != io.EOF {
			t.Fatalf("Read() = %d, %v, want 0, EOF", count, err)
		}
	})
	t.Run("without_close_write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		client, inbound := net.Pipe()
		remote, outbound := net.Pipe()
		t.Cleanup(func() {
			client.Close()
			inbound.Close()
			remote.Close()
			outbound.Close()
		})
		if err := remote.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() {
			tunnel(ctx, inbound, outbound)
			close(done)
		}()
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
		count, err := remote.Read(make([]byte, 1))
		if count != 0 || err != io.EOF {
			t.Fatalf("Read() = %d, %v, want 0, EOF", count, err)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("tunnel did not stop without CloseWrite support")
		}
	})
}

func TestRunForwardDialFailure(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { target.Close() })
	address := target.Addr().String()
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	listener := startForward(t, context.Background(), address)
	client := dialForward(t, listener.Addr().String())
	count, err := client.Read(make([]byte, 1))
	if count != 0 || err == nil {
		t.Fatalf("Read() = %d, %v, want an error without data", count, err)
	}
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatalf("client was not closed after a failed dial: %v", err)
	}
}

func TestRunForwardContextCancel(t *testing.T) {
	remoteDone := make(chan struct{})
	target := startForwardTarget(t, func(conn net.Conn) {
		defer close(remoteDone)
		io.Copy(conn, conn)
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	listener := startForward(t, ctx, target)
	client := dialForward(t, listener.Addr().String())
	checkForwardEcho(t, client, "ready")
	reading := make(chan struct{})
	readDone := make(chan error, 1)
	go func() {
		close(reading)
		_, err := client.Read(make([]byte, 1))
		readDone <- err
	}()
	<-reading
	cancel()
	select {
	case err := <-readDone:
		if err == nil {
			t.Fatal("Read returned no error after cancellation")
		}
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			t.Fatalf("client was not closed after cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("client Read did not stop after cancellation")
	}
	select {
	case <-remoteDone:
	case <-time.After(time.Second):
		t.Fatal("target connection was not closed after cancellation")
	}
}

func TestRunForwardEmptyTarget(t *testing.T) {
	err := RunForward(context.Background(), nil, local.LOCAL, "")
	if err == nil || err.Error() != "forward target is empty" {
		t.Fatalf("RunForward() = %v, want forward target is empty", err)
	}
}

func TestRunForwardThroughServe(t *testing.T) {
	target := startForwardTarget(t, func(conn net.Conn) { io.Copy(conn, conn) })
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan Event, 1)
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, func(callCtx context.Context) (net.Listener, error) {
			return local.LOCAL.Listen(callCtx, "tcp", "127.0.0.1:0")
		}, func(callCtx context.Context, listener net.Listener) error {
			return RunForward(callCtx, listener, local.LOCAL, target)
		}, func(event Event) {
			select {
			case events <- event:
			case <-ctx.Done():
			}
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != ctx.Err() || err != context.Canceled {
				t.Errorf("Serve() = %v, want context cancellation", err)
			}
		case <-time.After(time.Second):
			t.Error("Serve did not stop after cancellation")
		}
	})
	var address string
	select {
	case event := <-events:
		if event.Err != nil || event.Addr == nil {
			t.Fatalf("first event = %v, want listening address", event)
		}
		address = event.Addr.String()
	case <-time.After(time.Second):
		t.Fatal("Serve did not report a listening address")
	}
	client := dialForward(t, address)
	checkForwardEcho(t, client, "through Serve")
	cancel()
}

func startForwardTarget(t *testing.T, serve func(net.Conn)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		var connections sync.WaitGroup
		defer connections.Wait()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			go func() {
				defer connections.Done()
				defer conn.Close()
				stop := context.AfterFunc(ctx, func() { conn.Close() })
				defer stop()
				serve(conn)
			}()
		}
	}()
	t.Cleanup(func() {
		cancel()
		listener.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("target server did not stop")
		}
	})
	return listener.Addr().String()
}

func startForward(t *testing.T, ctx context.Context, target string) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- RunForward(ctx, listener, local.LOCAL, target) }()
	t.Cleanup(func() {
		cancel()
		listener.Close()
		select {
		case err := <-done:
			if err == nil || !utils.IsClosedConnError(err) {
				t.Errorf("RunForward() = %v, want closed connection error", err)
			}
		case <-time.After(time.Second):
			t.Error("RunForward did not stop after closing its listener")
		}
	})
	return listener
}

func dialForward(t *testing.T, address string) *net.TCPConn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return conn.(*net.TCPConn)
}

func checkForwardEcho(t *testing.T, conn net.Conn, payload string) {
	t.Helper()
	if _, err := io.WriteString(conn, payload); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != payload {
		t.Fatalf("response = %q, want %q", response, payload)
	}
}
