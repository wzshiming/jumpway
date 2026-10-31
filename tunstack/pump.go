package tunstack

import (
	"context"
	"errors"

	"golang.zx2c4.com/wireguard/tun"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// offset is the headroom kept in front of every packet, enough for the virtio-net header of Linux TUN offloads.
const offset = 16

func readPackets(dev tun.Device, ep *channel.Endpoint) error {
	bufs := make([][]byte, dev.BatchSize())
	for i := range bufs {
		bufs[i] = make([]byte, offset+65535)
	}
	sizes := make([]int, len(bufs))
	for {
		n, err := dev.Read(bufs, sizes, offset)
		if errors.Is(err, tun.ErrTooManySegments) {
			continue
		}
		for i := range n {
			injectPacket(ep, bufs[i][offset:offset+sizes[i]])
		}
		if err != nil {
			return err
		}
	}
}

func injectPacket(ep *channel.Endpoint, packet []byte) {
	var protocol tcpip.NetworkProtocolNumber
	switch header.IPVersion(packet) {
	case header.IPv4Version:
		protocol = header.IPv4ProtocolNumber
	case header.IPv6Version:
		protocol = header.IPv6ProtocolNumber
	default:
		return
	}
	pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(packet)})
	ep.InjectInbound(protocol, pkt)
	pkt.DecRef()
}

func writePackets(ctx context.Context, dev tun.Device, ep *channel.Endpoint) {
	bufs := make([][]byte, dev.BatchSize())
	for i := range bufs {
		bufs[i] = make([]byte, offset+65535)
	}
	batch := make([][]byte, 0, len(bufs))
	for pkt := ep.ReadContext(ctx); pkt != nil; pkt = ep.ReadContext(ctx) {
		batch = batch[:0]
		for pkt != nil {
			buf, n := bufs[len(batch)], offset
			for _, view := range pkt.AsSlices() {
				n += copy(buf[n:], view)
			}
			pkt.DecRef()
			batch = append(batch, buf[:n])
			if len(batch) == len(bufs) {
				break
			}
			pkt = ep.Read()
		}
		dev.Write(batch, offset)
	}
}

func drainEvents(events <-chan tun.Event, stop <-chan struct{}) {
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return
			}
		case <-stop:
			return
		}
	}
}
