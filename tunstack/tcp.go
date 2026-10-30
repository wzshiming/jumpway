package tunstack

import (
	"net"

	"github.com/wzshiming/jumpway"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/waiter"
)

// handleTCP dials the destination of a connection request before completing its handshake, refusing it with RST when the dial fails.
func (g *gateway) handleTCP(r *tcp.ForwarderRequest) {
	if !g.start() {
		r.Complete(true)
		return
	}
	defer g.flows.Done()
	id := r.ID()
	client := net.TCPAddrFromAddrPort(addrPort(id.RemoteAddress, id.RemotePort))
	remote, err := g.dialer.DialContext(jumpway.WithClientAddr(g.ctx, client), "tcp", addrPort(id.LocalAddress, id.LocalPort).String())
	if err != nil {
		r.Complete(true)
		return
	}
	var wq waiter.Queue
	ep, e := r.CreateEndpoint(&wq)
	if e != nil {
		remote.Close()
		r.Complete(true)
		return
	}
	r.Complete(false)
	jumpway.Tunnel(g.ctx, gonet.NewTCPConn(&wq, ep), remote)
}
