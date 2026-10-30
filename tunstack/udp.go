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

// relayDNS carries the datagrams of one flow over a single TCP connection, each framed by its 2-byte length as DNS over TCP is.
func (g *gateway) relayDNS(conn *gonet.UDPConn, id stack.TransportEndpointID) {
	defer g.flows.Done()
	defer conn.Close()
	client := net.UDPAddrFromAddrPort(addrPort(id.RemoteAddress, id.RemotePort))
	remote, err := g.dialer.DialContext(jumpway.WithClientAddr(g.ctx, client), "tcp", addrPort(id.LocalAddress, id.LocalPort).String())
	if err != nil {
		return
	}
	defer remote.Close()
	stop := context.AfterFunc(g.ctx, func() { conn.Close() })
	defer stop()
	// Datagrams either way push back the read deadline of conn, so its expiry is the idle timeout of the flow.
	touch := func() { conn.SetReadDeadline(time.Now().Add(g.dnsIdle)) }
	touch()
	replies := make(chan struct{})
	go func() {
		defer close(replies)
		defer conn.Close()
		relayReplies(conn, remote, touch)
	}()
	relayQueries(conn, remote, touch)
	remote.Close()
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
