package metrics

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
)

// ttPacket is one datagram of a ttPacketConn, with its peer.
type ttPacket struct {
	size int
	peer net.Addr
}

// ttPacketConn reads the datagrams queued in reads and discards every write.
type ttPacketConn struct {
	net.PacketConn
	reads  chan ttPacket
	closed bool
}

func (c *ttPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	packet, ok := <-c.reads
	if !ok {
		return 0, nil, net.ErrClosed
	}
	return packet.size, packet.peer, nil
}

func (c *ttPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	return len(p), nil
}

func (c *ttPacketConn) Close() error {
	c.closed = true
	return nil
}

type ttEntry struct {
	client, target string
	up, down       int64
}

func TestRuleWrapPacketConnAccountsSessionAsOneEntry(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "tun"}}, "tun", ttTUNPrefixes)
	registry.lookup = nil
	client := netip.MustParseAddrPort("198.18.0.1:50000")
	first := net.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.1:53"))
	second := net.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.2:443"))
	inner := &ttPacketConn{reads: make(chan ttPacket, 3)}
	ctx := jumpway.WithClientAddr(context.Background(), net.UDPAddrFromAddrPort(client))
	conn := registry.Rule("tun").WrapPacketConn(ctx, inner)
	inner.reads <- ttPacket{size: 13, peer: second}
	if _, _, err := conn.ReadFrom(make([]byte, 64)); err != nil {
		t.Fatal(err)
	}
	if rule := registry.Snapshot().Rules[0]; len(rule.Connections) != 0 || len(rule.Targets) != 0 {
		t.Fatalf("before the first write connections %+v and targets %+v, want none", rule.Connections, rule.Targets)
	}
	for _, write := range []struct {
		size int
		to   net.Addr
	}{{3, first}, {5, second}, {2, first}} {
		if _, err := conn.WriteTo(make([]byte, write.size), write.to); err != nil {
			t.Fatal(err)
		}
	}
	inner.reads <- ttPacket{size: 7, peer: first}
	inner.reads <- ttPacket{size: 11, peer: second}
	for range 2 {
		if _, _, err := conn.ReadFrom(make([]byte, 64)); err != nil {
			t.Fatal(err)
		}
	}
	want := ttEntry{client: client.String(), target: first.String(), up: 10, down: 18}
	rule := registry.Snapshot().Rules[0]
	if len(rule.Connections) != 1 {
		t.Fatalf("connections = %+v, want one for the session", rule.Connections)
	}
	if connection := rule.Connections[0]; (ttEntry{connection.Client, connection.Target, connection.Stats.Up, connection.Stats.Down}) != want {
		t.Fatalf("connection = %+v, want %+v", connection, want)
	}
	if len(rule.Targets) != 1 || rule.Targets[0].Address != want.target || rule.Targets[0].Stats.Active != 1 || rule.Targets[0].Stats.Up != want.up || rule.Targets[0].Stats.Down != want.down {
		t.Fatalf("targets = %+v, want only %s, active with %d bytes up and %d down", rule.Targets, want.target, want.up, want.down)
	}

	if err := conn.Close(); err != nil || !inner.closed {
		t.Fatalf("Close = %v, inner closed = %v; want nil and closed", err, inner.closed)
	}
	rule = registry.Snapshot().Rules[0]
	if len(rule.Connections) != 0 || rule.Targets[0].Stats.Active != 0 || rule.Targets[0].Stats.Total != 1 {
		t.Fatalf("after Close connections %+v and target %+v, want none and active 0 of total 1", rule.Connections, rule.Targets[0])
	}
}

func TestRegistryDisconnectClosesPacketConn(t *testing.T) {
	registry := NewRegistry()
	registry.Sync([]config.Rule{{Name: "tun"}}, "", nil)
	inner := &ttPacketConn{reads: make(chan ttPacket)}
	conn := registry.Rule("tun").WrapPacketConn(context.Background(), inner)
	if _, err := conn.WriteTo([]byte("x"), net.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.1:53"))); err != nil {
		t.Fatal(err)
	}
	connections := registry.Snapshot().Rules[0].Connections
	if len(connections) != 1 {
		t.Fatalf("connections = %+v, want one", connections)
	}
	if err := registry.Disconnect(connections[0].ID); err != nil || !inner.closed {
		t.Fatalf("Disconnect = %v, inner closed = %v; want nil and closed", err, inner.closed)
	}
	if connections := registry.Snapshot().Rules[0].Connections; len(connections) != 0 {
		t.Fatalf("connections after Disconnect = %+v, want none", connections)
	}
}
