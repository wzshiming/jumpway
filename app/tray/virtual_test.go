package tray

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func dialVirtual(test *testing.T, network *virtualNetwork, name string) <-chan error {
	test.Helper()
	result := make(chan error, 1)
	go func() {
		conn, err := network.Dial(context.Background(), name)
		if err == nil {
			_ = conn.Close()
		}
		result <- err
	}()
	return result
}

func waitErr(test *testing.T, result <-chan error, what string) error {
	test.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		test.Fatalf("%s did not return", what)
		return nil
	}
}

func TestVirtualNetworkRoundTrip(test *testing.T) {
	var network virtualNetwork
	listener, err := network.Listen("x")
	if err != nil {
		test.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	if addr := listener.Addr(); addr.Network() != "virtual" || addr.String() != "virtual://x" {
		test.Fatalf("listener addr = %s %q", addr.Network(), addr)
	}
	type dialed struct {
		conn net.Conn
		err  error
	}
	dials := make(chan dialed, 1)
	go func() {
		conn, err := network.Dial(context.Background(), "x")
		dials <- dialed{conn, err}
	}()
	server, err := listener.Accept()
	if err != nil {
		test.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	var client net.Conn
	select {
	case result := <-dials:
		if result.err != nil {
			test.Fatal(result.err)
		}
		client = result.conn
	case <-time.After(2 * time.Second):
		test.Fatal("dial did not return")
	}
	defer func() { _ = client.Close() }()
	for _, conn := range []net.Conn{client, server} {
		if conn.LocalAddr().String() != "virtual://x" || conn.RemoteAddr().String() != "virtual://x" || conn.RemoteAddr().Network() != "virtual" {
			test.Fatalf("conn addrs = %v %v", conn.LocalAddr(), conn.RemoteAddr())
		}
		if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			test.Fatal(err)
		}
	}
	for _, direction := range []struct {
		from, to net.Conn
		payload  string
	}{{client, server, "ping"}, {server, client, "pong"}} {
		go io.WriteString(direction.from, direction.payload)
		reply := make([]byte, len(direction.payload))
		if _, err := io.ReadFull(direction.to, reply); err != nil {
			test.Fatal(err)
		}
		if string(reply) != direction.payload {
			test.Fatalf("payload = %q, want %q", reply, direction.payload)
		}
	}
	if err := listener.Close(); err != nil {
		test.Fatal(err)
	}
	go io.WriteString(client, "after close")
	reply := make([]byte, len("after close"))
	if _, err := io.ReadFull(server, reply); err != nil {
		test.Fatalf("established pair broken by listener close: %v", err)
	}
}

func TestVirtualNetworkMissingListener(test *testing.T) {
	var network virtualNetwork
	if _, err := network.Dial(context.Background(), "missing"); err == nil || !strings.Contains(err.Error(), "missing") {
		test.Fatalf("Dial(missing) = %v, want an error naming the channel", err)
	}
}

func TestVirtualNetworkDuplicateListener(test *testing.T) {
	var network virtualNetwork
	first, err := network.Listen("x")
	if err != nil {
		test.Fatal(err)
	}
	if _, err := network.Listen("x"); err == nil {
		test.Fatal("second Listen(x) succeeded")
	}
	if err := first.Close(); err != nil {
		test.Fatal(err)
	}
	if err := first.Close(); err != nil {
		test.Fatalf("second Close = %v", err)
	}
	second, err := network.Listen("x")
	if err != nil {
		test.Fatalf("Listen(x) after Close = %v", err)
	}
	defer func() { _ = second.Close() }()
	if err := first.Close(); err != nil {
		test.Fatal(err)
	}
	accepts := make(chan error, 1)
	go func() {
		conn, err := second.Accept()
		if err == nil {
			_ = conn.Close()
		}
		accepts <- err
	}()
	if err := waitErr(test, dialVirtual(test, &network, "x"), "dial"); err != nil {
		test.Fatalf("dial after re-listen = %v", err)
	}
	if err := waitErr(test, accepts, "Accept"); err != nil {
		test.Fatalf("Accept after stale Close = %v", err)
	}
}

func TestVirtualNetworkCloseReleasesPending(test *testing.T) {
	var network virtualNetwork
	listener, err := network.Listen("x")
	if err != nil {
		test.Fatal(err)
	}
	accepts := make(chan error, 1)
	go func() {
		_, err := listener.Accept()
		accepts <- err
	}()
	if err := listener.Close(); err != nil {
		test.Fatal(err)
	}
	if err := waitErr(test, accepts, "Accept"); !errors.Is(err, net.ErrClosed) {
		test.Fatalf("Accept after Close = %v, want net.ErrClosed", err)
	}
	listener, err = network.Listen("x")
	if err != nil {
		test.Fatal(err)
	}
	pending := dialVirtual(test, &network, "x")
	select {
	case err := <-pending:
		test.Fatalf("dial returned %v before Accept", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := listener.Close(); err != nil {
		test.Fatal(err)
	}
	if err := waitErr(test, pending, "pending dial"); err == nil {
		test.Fatal("pending dial succeeded after the listener closed")
	}
	if _, err := network.Dial(context.Background(), "x"); err == nil {
		test.Fatal("Dial(x) succeeded after Close")
	}
}

func TestVirtualNetworkDialCanceled(test *testing.T) {
	var network virtualNetwork
	listener, err := network.Listen("x")
	if err != nil {
		test.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := network.Dial(ctx, "x")
		result <- err
	}()
	cancel()
	if err := waitErr(test, result, "canceled dial"); !errors.Is(err, context.Canceled) {
		test.Fatalf("canceled Dial = %v, want context.Canceled", err)
	}
}

func TestVirtualNetworkIsolation(test *testing.T) {
	var first, second virtualNetwork
	listener, err := first.Listen("x")
	if err != nil {
		test.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	if _, err := second.Dial(context.Background(), "x"); err == nil {
		test.Fatal("Dial crossed networks")
	}
	other, err := second.Listen("x")
	if err != nil {
		test.Fatalf("same name on another network = %v", err)
	}
	_ = other.Close()
}

func TestVirtualNetworkPreCanceledDial(test *testing.T) {
	var network virtualNetwork
	listener, err := network.Listen("x")
	if err != nil {
		test.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	handoff := make(chan net.Conn, 1)
	listener.(*virtualListener).conns = handoff
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for attempt := range 64 {
		conn, err := network.Dial(ctx, "x")
		if conn != nil {
			_ = conn.Close()
		}
		select {
		case accepted := <-handoff:
			_ = accepted.Close()
			test.Fatalf("attempt %d: pre-canceled dial delivered a connection", attempt)
		default:
		}
		if !errors.Is(err, context.Canceled) {
			test.Fatalf("attempt %d: Dial = %v, want context.Canceled", attempt, err)
		}
	}
}
