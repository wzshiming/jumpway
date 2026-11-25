// Package tunproxy serves the TCP and UDP flows a TUN device captures through a dialer and a UDP listen config.
package tunproxy

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/wzshiming/tun"
	"github.com/wzshiming/tun/stack"
	"github.com/wzshiming/tun/stack/dns"
	"github.com/wzshiming/tun/stack/tcp"
	"github.com/wzshiming/tun/stack/udp"
)

// BypassPrefixes keeps the entries of a NO_PROXY-style list that are IPs or CIDRs, as routes cannot match hostnames.
func BypassPrefixes(list []string) []netip.Prefix {
	var prefixes []netip.Prefix
	for _, entry := range list {
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			prefixes = append(prefixes, prefix)
		} else if addr, err := netip.ParseAddr(entry); err == nil {
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
		}
	}
	return prefixes
}

// Options configure Serve.
type Options struct {
	// Config is the device to open; its MTU and Addresses also tune the stack.
	Config tun.Config
	// Open opens the device; nil means tun.Open.
	Open func(context.Context, tun.Config) (*tun.Device, error)
	// Dialer carries TCP flows, DNS and, without ListenConfig, UDP per destination.
	Dialer stack.Dialer
	// ListenConfig, unless nil, opens each UDP session's socket.
	ListenConfig stack.ListenConfig
	// Up, unless nil, is called once with the open device before it is served.
	Up func(*tun.Device)
	// UDPIdle ends a UDP session after this long without datagrams; 0 means 60s.
	UDPIdle time.Duration
	// DNS configures the flows to UDP port 53.
	DNS DNS
	// WrapConn, unless nil, wraps the conn of every flow a handler accepts.
	WrapConn func(net.Conn) net.Conn
}

// DNS configures the flows to UDP port 53; the zero value relays them as DNS over TCP through the Dialer.
type DNS struct {
	// AsUDP serves port 53 like any other UDP port instead of relaying it as DNS over TCP.
	AsUDP bool
	// Server, unless empty, is the host:port every DNS-over-TCP connection dials instead of the addressed server.
	Server string
	// Idle ends a DNS session after this long without queries or replies; 0 means 10s.
	Idle time.Duration
}

// Serve opens the device and serves Handler on it until ctx ends or the device fails.
func Serve(ctx context.Context, opts Options) error {
	open := opts.Open
	if open == nil {
		open = tun.Open
	}
	dev, err := open(ctx, opts.Config)
	if err != nil {
		return err
	}
	if opts.Up != nil {
		opts.Up(dev)
	}
	return stack.Serve(ctx, dev, Handler(opts), stack.Options{MTU: opts.Config.MTU, Addresses: opts.Config.Addresses})
}

// Handler relays flows as opts says: TCP and DNS through Dialer, other UDP via ListenConfig if set; Config, Open and Up are unused.
func Handler(opts Options) stack.Handler {
	m := stack.NewMux()
	m.Handle("tcp", "", &tcp.Handler{Dialer: opts.Dialer})
	if !opts.DNS.AsUDP {
		m.Handle("udp", ":53", &dns.Handler{Dialer: opts.Dialer, Target: opts.DNS.Server, Idle: opts.DNS.Idle})
	}
	m.Handle("udp", "", &udp.Handler{Dialer: opts.Dialer, ListenConfig: opts.ListenConfig, Idle: opts.UDPIdle})
	if opts.WrapConn == nil {
		return m
	}
	return stack.HandlerFunc(func(ctx context.Context, f stack.Flow) {
		m.ServeFlow(ctx, wrappingFlow{Flow: f, wrap: opts.WrapConn})
	})
}

// wrappingFlow is a Flow whose accepted conn passes through wrap.
type wrappingFlow struct {
	stack.Flow
	wrap func(net.Conn) net.Conn
}

func (f wrappingFlow) Accept() (net.Conn, error) {
	conn, err := f.Flow.Accept()
	if err != nil {
		return nil, err
	}
	return f.wrap(f.wrap(conn)), nil
}

func (f wrappingFlow) Reject() {
	f.wrap(nil)
	f.Flow.Reject()
}
