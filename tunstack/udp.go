package tunstack

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/wzshiming/jumpway"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

// handleUDP relays a new flow to port 53 over TCP; other ports stay unhandled, so the stack answers ICMP port unreachable.
func (g *gateway) handleUDP(r *udp.ForwarderRequest) bool {
	id := r.ID()
	if id.LocalPort != 53 || !g.start() {
		return false
	}
	var wq waiter.Queue
	ep, err := r.CreateEndpoint(&wq)
	if err != nil {
		g.flows.Done()
		return true
	}
	go g.relayDNS(gonet.NewUDPConn(&wq, ep), id)
	return true
}

func (g *gateway) relayDNS(conn *gonet.UDPConn, id stack.TransportEndpointID) {
	defer g.flows.Done()
	defer conn.Close()
	client := net.UDPAddrFromAddrPort(addrPort(id.RemoteAddress, id.RemotePort))
	remote, err := g.dialer.DialContext(jumpway.WithClientAddr(g.ctx, client), "tcp", addrPort(id.LocalAddress, id.LocalPort).String())
	if err != nil {
		return
	}
	closeBoth := func() {
		conn.Close()
		remote.Close()
	}
	defer closeBoth()
	stop := context.AfterFunc(g.ctx, closeBoth)
	defer stop()
	// Datagrams either way push back the idle timer, whose expiry closes both conns wherever the relay blocks.
	idle := time.AfterFunc(g.dnsIdle, closeBoth)
	defer idle.Stop()
	touch := func() { idle.Reset(g.dnsIdle) }
	replies := make(chan struct{})
	go func() {
		defer close(replies)
		defer closeBoth()
		relayReplies(conn, remote, touch)
	}()
	relayQueries(conn, remote, touch)
	closeBoth()
	<-replies
}

func relayQueries(conn *gonet.UDPConn, remote net.Conn, touch func()) {
	buf := make([]byte, 2+65535)
	for {
		n, err := conn.Read(buf[2:])
		if err != nil {
			return
		}
		binary.BigEndian.PutUint16(buf, uint16(n))
		if _, err := remote.Write(buf[:2+n]); err != nil {
			return
		}
		touch()
	}
}

func relayReplies(conn *gonet.UDPConn, remote net.Conn, touch func()) {
	buf := make([]byte, 65535)
	for {
		if _, err := io.ReadFull(remote, buf[:2]); err != nil {
			return
		}
		reply := buf[:binary.BigEndian.Uint16(buf)]
		if _, err := io.ReadFull(remote, reply); err != nil {
			return
		}
		if _, err := conn.Write(reply); err != nil {
			return
		}
		touch()
	}
}
