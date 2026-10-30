// Package tunstack terminates the IP packets of a TUN device in a gVisor netstack and dials what it accepts through a bridge.Dialer.
package tunstack

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/wzshiming/bridge"
	"golang.zx2c4.com/wireguard/tun"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

const nicID tcpip.NICID = 1

// Options tunes Serve.
type Options struct {
	// MTU is the link MTU of the stack; 0 takes the device's.
	MTU int
	// Addresses are the gateway's own addresses, the only ones answering ICMP echo.
	Addresses []netip.Prefix
	// DNSIdle ends a DNS flow after this long without datagrams; 0 means 10s.
	DNSIdle time.Duration
}

// Serve dials the TCP connections and DNS flows arriving on dev through dialer until ctx ends or reading dev fails, and always closes dev.
func Serve(ctx context.Context, dev tun.Device, dialer bridge.Dialer, opts Options) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	g := &gateway{ctx: runCtx, dialer: dialer, dnsIdle: cmp.Or(opts.DNSIdle, 10*time.Second)}
	s, ep, err := g.newStack(dev, opts)
	if err != nil {
		dev.Close()
		return err
	}

	readErr := make(chan error, 1)
	var pumps sync.WaitGroup
	pumps.Go(func() { readErr <- readPackets(dev, ep) })
	pumps.Go(func() { writePackets(runCtx, dev, ep) })
	select {
	case <-ctx.Done():
	case err = <-readErr:
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	} else {
		err = fmt.Errorf("tunstack: read packets: %w", err)
	}

	// No flow starts once the pumps are gone; without the NIC a late handshake fails at once, and Close aborts those in progress.
	cancel()
	dev.Close()
	pumps.Wait()
	g.stop()
	s.RemoveNIC(nicID)
	s.Close()
	g.flows.Wait()
	s.Wait()
	return err
}

// gateway dials the flows the stack accepts and counts them so Serve can wait for them.
type gateway struct {
	ctx     context.Context
	dialer  bridge.Dialer
	dnsIdle time.Duration

	mu      sync.Mutex
	stopped bool
	flows   sync.WaitGroup
}

// start counts a new flow in; it reports false once Serve is shutting down.
func (g *gateway) start() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.stopped {
		g.flows.Add(1)
	}
	return !g.stopped
}

func (g *gateway) stop() {
	g.mu.Lock()
	g.stopped = true
	g.mu.Unlock()
}

func (g *gateway) newStack(dev tun.Device, opts Options) (_ *stack.Stack, _ *channel.Endpoint, err error) {
	mtu := opts.MTU
	if mtu == 0 {
		if mtu, err = dev.MTU(); err != nil {
			return nil, nil, fmt.Errorf("tunstack: device MTU: %w", err)
		}
	}
	s := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol, icmp.NewProtocol4, icmp.NewProtocol6},
	})
	defer func() {
		if err != nil {
			s.Destroy()
		}
	}()
	sack := tcpip.TCPSACKEnabled(true)
	if e := s.SetTransportProtocolOption(tcp.ProtocolNumber, &sack); e != nil {
		return nil, nil, fmt.Errorf("tunstack: enable TCP SACK: %s", e)
	}
	ep := channel.New(1024, uint32(mtu), "")
	if e := s.CreateNIC(nicID, ep); e != nil {
		return nil, nil, fmt.Errorf("tunstack: create NIC: %s", e)
	}
	s.SetPromiscuousMode(nicID, true)
	s.SetSpoofing(nicID, true)
	s.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: nicID},
		{Destination: header.IPv6EmptySubnet, NIC: nicID},
	})
	own := make(map[tcpip.Address]bool, len(opts.Addresses))
	for _, prefix := range opts.Addresses {
		if !prefix.IsValid() {
			return nil, nil, fmt.Errorf("tunstack: invalid address %v", prefix)
		}
		address := tcpip.ProtocolAddress{
			Protocol: ipv4.ProtocolNumber,
			AddressWithPrefix: tcpip.AddressWithPrefix{
				Address:   tcpip.AddrFromSlice(prefix.Addr().AsSlice()),
				PrefixLen: prefix.Bits(),
			},
		}
		if prefix.Addr().Is6() {
			address.Protocol = ipv6.ProtocolNumber
		}
		if e := s.AddProtocolAddress(nicID, address, stack.AddressProperties{}); e != nil {
			return nil, nil, fmt.Errorf("tunstack: add address %v: %s", prefix, e)
		}
		own[address.AddressWithPrefix.Address] = true
	}
	// Under spoofing ICMPv6 would answer echo requests to any destination, so those not sent to the gateway are consumed.
	consumeEcho := func(id stack.TransportEndpointID, _ *stack.PacketBuffer) bool { return !own[id.LocalAddress] }
	s.SetTransportProtocolHandler(icmp.ProtocolNumber4, consumeEcho)
	s.SetTransportProtocolHandler(icmp.ProtocolNumber6, consumeEcho)
	s.SetTransportProtocolHandler(tcp.ProtocolNumber, tcp.NewForwarder(s, 0, 2048, g.handleTCP).HandlePacket)
	s.SetTransportProtocolHandler(udp.ProtocolNumber, udp.NewForwarder(s, g.handleUDP).HandlePacket)
	return s, ep, nil
}

func addrPort(addr tcpip.Address, port uint16) netip.AddrPort {
	ip, _ := netip.AddrFromSlice(addr.AsSlice())
	return netip.AddrPortFrom(ip, port)
}
