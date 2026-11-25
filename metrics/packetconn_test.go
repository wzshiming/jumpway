package metrics

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/jumpway/config"
)

func TestHopDialerKeepsFullConePacketConn(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}}, "", nil)
	rule := registry.Rule("rule")
	socket := ttFullConeSocket(t)
	connected, err := rule.HopWrapper(Forward)(0, "socks5://hop:1080", ttDialing(socket)).DialContext(context.Background(), "udp", "192.0.2.1:53")
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, connected)
	ttEchoThrough(t, connected, ttUDPEcho(t), "first", 3)
	if stats := registry.Snapshot().Rules[0].Forward[0].URLs[0].Stats; stats.Up != 6 || stats.Down != 6 || stats.Active != 1 {
		t.Fatalf("hop = %+v, want active with 6 bytes each way from WriteTo, ReadFrom and Read", stats)
	}
}

func TestRuleDialerKeepsFullConePacketConn(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}}, "", nil)
	rule := registry.Rule("rule")
	socket := ttFullConeSocket(t)
	echo := ttUDPEcho(t)
	connected, err := rule.WrapDialer(ttDialing(socket)).DialContext(context.Background(), "udp", echo.String())
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, connected)
	ttEchoThrough(t, connected, echo, "first", 3)
	snapshot := registry.Snapshot().Rules[0]
	if len(snapshot.Targets) != 1 || snapshot.Targets[0].Stats.Up != 6 || snapshot.Targets[0].Stats.Down != 6 {
		t.Fatalf("targets = %+v, want one with 6 bytes each way from WriteTo, ReadFrom and Read", snapshot.Targets)
	}
	if len(snapshot.Connections) != 1 || snapshot.Connections[0].Stats.Up != 6 || snapshot.Connections[0].Stats.Down != 6 {
		t.Fatalf("connections = %+v, want one with 6 bytes each way from WriteTo, ReadFrom and Read", snapshot.Connections)
	}
}

func TestDialedConnsWithoutFullConeAreNotPacketConns(t *testing.T) {
	plain, peer := net.Pipe()
	closeOnCleanup(t, peer)
	connected, err := net.Dial("udp", ttUDPEcho(t).String())
	if err != nil {
		t.Fatal(err)
	}
	for name, inner := range map[string]net.Conn{"plain": plain, "connected UDP": connected} {
		registry := NewRegistry()
		registry.Sync([]config.Rule{{Name: "rule"}}, "", nil)
		rule := registry.Rule("rule")
		for level, dialer := range map[string]bridge.Dialer{
			"hop":  rule.HopWrapper(Forward)(0, "socks5://hop:1080", ttDialing(inner)),
			"rule": rule.WrapDialer(ttDialing(inner)),
		} {
			wrapped, err := dialer.DialContext(context.Background(), "udp", "192.0.2.1:53")
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := wrapped.(net.PacketConn); ok {
				t.Errorf("%s conn wrapped at the %s level is a net.PacketConn, want not", name, level)
			}
		}
		inner.Close()
	}
}

func TestRuleWrapConnKeepsAddressedDatagrams(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "rule"}}, "", nil)
	inner, peer := net.Pipe()
	closeOnCleanup(t, peer)
	client := &net.UDPAddr{IP: net.IPv4(198, 18, 0, 2), Port: 50000}
	wrapped := registry.Rule("rule").WrapConn(&ttAddressedConn{Conn: inner, client: client, read: "abc"})
	closeOnCleanup(t, wrapped)
	packet, ok := wrapped.(net.PacketConn)
	if !ok {
		t.Fatalf("wrapped %T is not a net.PacketConn", wrapped)
	}
	if n, from, err := packet.ReadFrom(make([]byte, 64)); err != nil || n != 3 || from != client {
		t.Fatalf("ReadFrom = %d, %v, %v; want 3 bytes from %v", n, from, err, client)
	}
	if n, err := packet.WriteTo([]byte("hello"), client); err != nil || n != 5 {
		t.Fatalf("WriteTo = %d, %v; want 5 bytes", n, err)
	}
	if stats := registry.Snapshot().Rules[0].Stats; stats.Up != 3 || stats.Down != 5 {
		t.Fatalf("rule stats = %+v, want 3 bytes up from ReadFrom and 5 down from WriteTo", stats)
	}
}

// ttAddressedConn is an accepted conn that also addresses datagrams: ReadFrom returns read from client, WriteTo takes any.
type ttAddressedConn struct {
	net.Conn
	client net.Addr
	read   string
}

func (c *ttAddressedConn) ReadFrom(b []byte) (int, net.Addr, error) {
	return copy(b, c.read), c.client, nil
}

func (c *ttAddressedConn) WriteTo(b []byte, _ net.Addr) (int, error) {
	return len(b), nil
}

// ttFullConeSocket is an unconnected UDP socket on IPv4 loopback that is not a *net.UDPConn.
func ttFullConeSocket(t *testing.T) net.Conn {
	t.Helper()
	socket, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, socket)
	return struct{ *net.UDPConn }{socket}
}

func ttDialing(conn net.Conn) bridge.Dialer {
	return bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) { return conn, nil })
}

// ttEchoThrough sends payload's first size bytes to echo twice by WriteTo and reads the echoes by ReadFrom and Read.
func ttEchoThrough(t *testing.T, connected net.Conn, echo *net.UDPAddr, payload string, size int) {
	t.Helper()
	packet, ok := connected.(net.PacketConn)
	if !ok {
		t.Fatalf("wrapped %T is not a net.PacketConn", connected)
	}
	connected.SetDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 64)
	if _, err := packet.WriteTo([]byte(payload[:size]), echo); err != nil {
		t.Fatal(err)
	}
	if n, source, err := packet.ReadFrom(buf); err != nil || n != size || source.String() != echo.String() {
		t.Fatalf("ReadFrom = %d, %v, %v; want %d bytes from %s", n, source, err, size, echo)
	}
	if _, err := packet.WriteTo([]byte(payload[:size]), echo); err != nil {
		t.Fatal(err)
	}
	if n, err := connected.Read(buf); err != nil || n != size {
		t.Fatalf("Read = %d, %v; want %d bytes", n, err, size)
	}
}

// ttUDPEcho serves a UDP echo on IPv4 loopback until the test ends and returns its address.
func ttUDPEcho(t *testing.T) *net.UDPAddr {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, conn)
	go func() {
		buf := make([]byte, 64)
		for {
			n, source, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			conn.WriteTo(buf[:n], source)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr)
}
