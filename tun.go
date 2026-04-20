package jumpway

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"

	"github.com/wzshiming/bridge"
	wgtun "golang.zx2c4.com/wireguard/tun"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const (
	tunNIC       = 1
	tunMTU       = 1500
	tunQueueSize = 1024
)

// TUNProxy manages a TUN-based global proxy that captures network traffic
// through a TUN device and forwards it via the provided dialer.
type TUNProxy struct {
	dev    wgtun.Device
	ep     *channel.Endpoint
	stack  *stack.Stack
	dialer bridge.Dialer

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// RunTUNProxy creates a TUN device and forwards all TCP traffic through the dialer.
// It returns a TUNProxy that can be stopped by calling Close().
func RunTUNProxy(ctx context.Context, tunName string, tunAddr netip.Prefix, dialer bridge.Dialer) (*TUNProxy, error) {
	dev, err := wgtun.CreateTUN(tunName, tunMTU)
	if err != nil {
		return nil, fmt.Errorf("create TUN device: %w", err)
	}

	name, err := dev.Name()
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("get TUN device name: %w", err)
	}

	err = configureTUN(name, tunAddr)
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("configure TUN device: %w", err)
	}

	ep := channel.New(tunQueueSize, tunMTU, "")

	opts := stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
	}
	s := stack.New(opts)

	sackEnabled := tcpip.TCPSACKEnabled(true)
	if tcpipErr := s.SetTransportProtocolOption(tcp.ProtocolNumber, &sackEnabled); tcpipErr != nil {
		dev.Close()
		return nil, fmt.Errorf("enable TCP SACK: %v", tcpipErr)
	}

	if tcpipErr := s.CreateNIC(tunNIC, ep); tcpipErr != nil {
		dev.Close()
		return nil, fmt.Errorf("create NIC: %v", tcpipErr)
	}

	addr := tunAddr.Addr()
	var protoNum tcpip.NetworkProtocolNumber
	if addr.Is4() {
		protoNum = ipv4.ProtocolNumber
	} else {
		protoNum = ipv6.ProtocolNumber
	}
	protoAddr := tcpip.ProtocolAddress{
		Protocol:          protoNum,
		AddressWithPrefix: tcpip.AddrFromSlice(addr.AsSlice()).WithPrefix(),
	}
	if tcpipErr := s.AddProtocolAddress(tunNIC, protoAddr, stack.AddressProperties{}); tcpipErr != nil {
		dev.Close()
		return nil, fmt.Errorf("add protocol address: %v", tcpipErr)
	}

	s.AddRoute(tcpip.Route{Destination: header.IPv4EmptySubnet, NIC: tunNIC})
	s.AddRoute(tcpip.Route{Destination: header.IPv6EmptySubnet, NIC: tunNIC})

	s.SetPromiscuousMode(tunNIC, true)
	s.SetSpoofing(tunNIC, true)

	ctx, cancel := context.WithCancel(ctx)
	tp := &TUNProxy{
		dev:    dev,
		ep:     ep,
		stack:  s,
		dialer: dialer,
		cancel: cancel,
	}

	tcpForwarder := tcp.NewForwarder(s, 0, 65535, func(r *tcp.ForwarderRequest) {
		id := r.ID()
		dstAddr := netip.AddrPortFrom(addrFromTCPIP(id.LocalAddress), id.LocalPort)
		go tp.handleTCP(ctx, r, dstAddr)
	})
	s.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	udpForwarder := udp.NewForwarder(s, func(r *udp.ForwarderRequest) {
		id := r.ID()
		dstAddr := netip.AddrPortFrom(addrFromTCPIP(id.LocalAddress), id.LocalPort)
		go tp.handleUDP(ctx, r, dstAddr)
	})
	s.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)

	// Relay packets between real TUN device and netstack.
	tp.wg.Add(2)
	go tp.tunToStack(ctx)
	go tp.stackToTUN(ctx)

	return tp, nil
}

// Close stops the TUN proxy and releases resources.
func (tp *TUNProxy) Close() error {
	tp.cancel()
	name, _ := tp.dev.Name()
	tp.dev.Close()
	tp.stack.Close()
	tp.ep.Close()
	tp.wg.Wait()
	if name != "" {
		unconfigureTUN(name)
	}
	return nil
}

// tunToStack reads packets from the real TUN device and injects them into the netstack.
func (tp *TUNProxy) tunToStack(ctx context.Context) {
	defer tp.wg.Done()

	batchSize := tp.dev.BatchSize()
	bufs := make([][]byte, batchSize)
	sizes := make([]int, batchSize)
	for i := range bufs {
		bufs[i] = make([]byte, tunMTU+header.EthernetMinimumSize)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := tp.dev.Read(bufs, sizes, 0)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}

		for i := 0; i < n; i++ {
			if sizes[i] == 0 {
				continue
			}
			pkt := bufs[i][:sizes[i]]
			v := buffer.NewViewWithData(pkt)
			tp.ep.InjectInbound(determineProtocol(pkt), stack.NewPacketBuffer(stack.PacketBufferOptions{
				Payload: buffer.MakeWithView(v),
			}))
		}
	}
}

// stackToTUN reads packets from the netstack and writes them to the real TUN device.
func (tp *TUNProxy) stackToTUN(ctx context.Context) {
	defer tp.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		pkt := tp.ep.ReadContext(ctx)
		if pkt == nil {
			return
		}

		pktView := pkt.ToView()
		pkt.DecRef()

		data := pktView.AsSlice()
		if len(data) == 0 {
			continue
		}

		bufs := [][]byte{data}
		if _, err := tp.dev.Write(bufs, 0); err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
	}
}

// handleTCP handles a forwarded TCP connection by dialing through the proxy.
func (tp *TUNProxy) handleTCP(ctx context.Context, r *tcp.ForwarderRequest, dst netip.AddrPort) {
	var wq waiter.Queue
	ep, tcpipErr := r.CreateEndpoint(&wq)
	if tcpipErr != nil {
		r.Complete(true)
		return
	}
	r.Complete(false)

	local := gonet.NewTCPConn(&wq, ep)
	defer local.Close()

	remote, err := tp.dialer.DialContext(ctx, "tcp", dst.String())
	if err != nil {
		return
	}
	defer remote.Close()

	relay(local, remote)
}

// handleUDP handles a forwarded UDP connection by dialing through the proxy.
func (tp *TUNProxy) handleUDP(ctx context.Context, r *udp.ForwarderRequest, dst netip.AddrPort) {
	var wq waiter.Queue
	ep, tcpipErr := r.CreateEndpoint(&wq)
	if tcpipErr != nil {
		return
	}

	local := gonet.NewUDPConn(&wq, ep)
	defer local.Close()

	remote, err := tp.dialer.DialContext(ctx, "udp", dst.String())
	if err != nil {
		return
	}
	defer remote.Close()

	relay(local, remote)
}

// closeWriter is implemented by connections that support half-close.
type closeWriter interface {
	CloseWrite() error
}

// relay copies data bidirectionally between two connections.
func relay(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(b, a)
		if cw, ok := b.(closeWriter); ok {
			cw.CloseWrite()
		}
	}()
	go func() {
		defer wg.Done()
		io.Copy(a, b)
		if cw, ok := a.(closeWriter); ok {
			cw.CloseWrite()
		}
	}()
	wg.Wait()
}

// determineProtocol determines the IP protocol version of a raw packet.
func determineProtocol(pkt []byte) tcpip.NetworkProtocolNumber {
	if len(pkt) == 0 {
		return 0
	}
	switch pkt[0] >> 4 {
	case 4:
		return header.IPv4ProtocolNumber
	case 6:
		return header.IPv6ProtocolNumber
	default:
		return 0
	}
}

// addrFromTCPIP converts a tcpip.Address to a netip.Addr.
func addrFromTCPIP(addr tcpip.Address) netip.Addr {
	b := addr.AsSlice()
	if len(b) == 4 {
		return netip.AddrFrom4([4]byte(b))
	}
	if len(b) == 16 {
		return netip.AddrFrom16([16]byte(b))
	}
	return netip.Addr{}
}
