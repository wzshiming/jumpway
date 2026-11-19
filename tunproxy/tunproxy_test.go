package tunproxy

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzshiming/tun"
	"github.com/wzshiming/tun/stack"
	wgtun "golang.zx2c4.com/wireguard/tun"
)

// ttWait bounds every wait for something that must happen.
const ttWait = 5 * time.Second

// ttConfig is the device config every Serve test opens.
var ttConfig = tun.Config{Name: "tt", Addresses: []netip.Prefix{netip.MustParsePrefix("198.18.0.1/30")}, MTU: 1500}

func TestBypassPrefixes(test *testing.T) {
	got := BypassPrefixes([]string{"10.0.0.0/8", "192.0.2.7", "2001:db8::/32", "3fff::9", "example.invalid", "10.1.2.3/33", "*"})
	want := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.0.2.7/32"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("3fff::9/128"),
	}
	if !slices.Equal(got, want) {
		test.Fatalf("BypassPrefixes = %v, want %v", got, want)
	}
	if got := BypassPrefixes(nil); got != nil {
		test.Fatalf("BypassPrefixes(nil) = %v, want nil", got)
	}
}

func TestHandlerRoutesFlows(test *testing.T) {
	client := netip.MustParseAddrPort("198.18.0.2:50000")
	for _, scenario := range []struct {
		name, network, target, wantNetwork string
	}{
		{name: "tcp relayed", network: "tcp", target: "192.0.2.1:443", wantNetwork: "tcp"},
		{name: "dns over tcp", network: "udp", target: "192.0.2.1:53", wantNetwork: "tcp"},
		{name: "udp relayed", network: "udp", target: "192.0.2.1:443", wantNetwork: "udp"},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			dials := make(chan ttDial, 1)
			flow := &ttFlow{network: scenario.network, target: netip.MustParseAddrPort(scenario.target), client: client}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				defer close(done)
				Handler(Options{Dialer: ttRecordingDialer(dials)}).ServeFlow(ctx, flow)
			}()
			defer func() {
				cancel()
				<-done
			}()
			want := ttDial{network: scenario.wantNetwork, address: scenario.target, client: client.String()}
			select {
			case got := <-dials:
				if got != want {
					test.Fatalf("dialed %+v, want %+v", got, want)
				}
			case <-time.After(ttWait):
				test.Fatalf("no dial, want %+v", want)
			}
		})
	}
}

func TestHandlerListensUDPThroughListenConfig(test *testing.T) {
	client := netip.MustParseAddrPort("198.18.0.2:50000")
	dials := make(chan ttDial, 1)
	listens := make(chan ttDial, 1)
	flow := &ttFlow{network: "udp", target: netip.MustParseAddrPort("192.0.2.1:443"), client: client}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Handler(Options{Dialer: ttRecordingDialer(dials), ListenConfig: ttRecordingListenConfig(listens)}).ServeFlow(ctx, flow)
	}()
	defer func() {
		cancel()
		<-done
	}()
	want := ttDial{network: "udp4", address: ":0", client: client.String()}
	select {
	case got := <-listens:
		if got != want {
			test.Fatalf("listened %+v, want %+v", got, want)
		}
	case got := <-dials:
		test.Fatalf("dialed %+v, want a listen %+v", got, want)
	case <-time.After(ttWait):
		test.Fatalf("no listen, want %+v", want)
	}
}

func TestHandlerDNSOptions(test *testing.T) {
	client := netip.MustParseAddrPort("198.18.0.2:50000")
	for _, scenario := range []struct {
		name   string
		dns    DNS
		listen bool
		want   ttDial
	}{
		{name: "server", dns: DNS{Server: "192.0.2.53:5353"}, want: ttDial{network: "tcp", address: "192.0.2.53:5353", client: client.String()}},
		{name: "as udp per destination", dns: DNS{AsUDP: true}, want: ttDial{network: "udp", address: "192.0.2.1:53", client: client.String()}},
		{name: "as udp full cone", dns: DNS{AsUDP: true}, listen: true, want: ttDial{network: "udp4", address: ":0", client: client.String()}},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			calls := make(chan ttDial, 1)
			opts := Options{Dialer: ttRecordingDialer(calls), DNS: scenario.dns}
			if scenario.listen {
				opts.ListenConfig = ttRecordingListenConfig(calls)
			}
			flow := &ttFlow{network: "udp", target: netip.MustParseAddrPort("192.0.2.1:53"), client: client}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				defer close(done)
				Handler(opts).ServeFlow(ctx, flow)
			}()
			defer func() {
				cancel()
				<-done
			}()
			select {
			case got := <-calls:
				if got != scenario.want {
					test.Fatalf("served %+v, want %+v", got, scenario.want)
				}
			case <-time.After(ttWait):
				test.Fatalf("nothing served, want %+v", scenario.want)
			}
		})
	}
}

func TestHandlerIdleOptionsEndSessions(test *testing.T) {
	client := netip.MustParseAddrPort("198.18.0.2:50000")
	for _, scenario := range []struct {
		name   string
		opts   Options
		target string
	}{
		{name: "udp", opts: Options{UDPIdle: 10 * time.Millisecond}, target: "192.0.2.1:443"},
		{name: "dns", opts: Options{DNS: DNS{Idle: 10 * time.Millisecond}}, target: "192.0.2.1:53"},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			opts := scenario.opts
			opts.Dialer = ttRecordingDialer(make(chan ttDial, 1))
			flow := &ttFlow{network: "udp", target: netip.MustParseAddrPort(scenario.target), client: client}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() {
				defer close(done)
				Handler(opts).ServeFlow(ctx, flow)
			}()
			select {
			case <-done:
			case <-time.After(ttWait):
				cancel()
				<-done
				test.Fatalf("session still served %v after it idled, want it ended by the 10ms idle", ttWait)
			}
		})
	}
}

func TestServeReturnsOpenError(test *testing.T) {
	openErr := fmt.Errorf("tt helper: %w", tun.ErrDenied)
	var requested []tun.Config
	var ups atomic.Int32
	err := Serve(context.Background(), Options{
		Config: ttConfig,
		Open: func(ctx context.Context, cfg tun.Config) (*tun.Device, error) {
			requested = append(requested, cfg)
			return nil, openErr
		},
		Up: func(*tun.Device) { ups.Add(1) },
	})
	if !errors.Is(err, tun.ErrDenied) || err.Error() != openErr.Error() {
		test.Fatalf("Serve = %v, want the open error %v", err, openErr)
	}
	if len(requested) != 1 || requested[0].Name != ttConfig.Name {
		test.Fatalf("opened with %+v, want once with %+v", requested, ttConfig)
	}
	if n := ups.Load(); n != 0 {
		test.Fatalf("Up called %d times after a failed open, want none", n)
	}
}

func TestServeCallsUpBeforeServing(test *testing.T) {
	device := ttNewDevice()
	opened := &tun.Device{Device: device, Interface: "tt9"}
	var ups []*tun.Device
	var early bool
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, Options{
			Config: ttConfig,
			Open:   func(context.Context, tun.Config) (*tun.Device, error) { return opened, nil },
			Up: func(dev *tun.Device) {
				ups = append(ups, dev)
				select {
				case <-device.read:
					early = true
				default:
				}
			},
		})
	}()
	select {
	case <-device.read:
	case <-time.After(ttWait):
		test.Fatal("the device was never read")
	}
	cancel()
	ttServed(test, done)
	if len(ups) != 1 || ups[0] != opened || early {
		test.Fatalf("Up got %v, read before Up = %v; want the opened device once, before the first read", ups, early)
	}
}

func TestServeWithoutUpServesDevice(test *testing.T) {
	client := netip.MustParseAddrPort("198.18.0.2:50000")
	device := ttNewDevice()
	dials := make(chan ttDial, 1)
	opts := ttOptions(device, ttRecordingDialer(dials), nil)
	opts.Up = nil
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, opts) }()
	device.inbound <- ttUDPPacket(client, netip.MustParseAddrPort("192.0.2.1:53"), []byte("query"))
	select {
	case <-dials:
	case err := <-done:
		test.Fatalf("Serve without Up = %v, want it serving", err)
	case <-time.After(ttWait):
		test.Fatal("nothing served without Up")
	}
	cancel()
	ttServed(test, done)
}

func TestServeEndsWithContext(test *testing.T) {
	device := ttNewDevice()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, ttOptions(device, nil, nil)) }()
	<-device.read
	cancel()
	ttServed(test, done)
	if !device.isClosed() {
		test.Fatal("the device is open after Serve returned, want it closed")
	}
}

func TestServeEndsWhenDeviceFails(test *testing.T) {
	device := ttNewDevice()
	device.readErr = errors.New("tt device gone")
	done := make(chan error, 1)
	go func() { done <- Serve(context.Background(), ttOptions(device, nil, nil)) }()
	if err := ttServed(test, done); err == nil {
		test.Fatal("Serve = nil after the device failed, want an error")
	}
}

func TestServeRelaysFlowsThroughHandler(test *testing.T) {
	client := netip.MustParseAddrPort("198.18.0.2:50000")
	for _, scenario := range []struct {
		name, target string
		cone         bool
		want         ttDial
	}{
		{name: "dns over tcp", target: "192.0.2.1:53", want: ttDial{network: "tcp", address: "192.0.2.1:53", client: client.String()}},
		{name: "udp per destination", target: "192.0.2.1:443", want: ttDial{network: "udp", address: "192.0.2.1:443", client: client.String()}},
		{name: "udp full cone", target: "192.0.2.1:443", cone: true, want: ttDial{network: "udp4", address: ":0", client: client.String()}},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			calls := make(chan ttDial, 1)
			var listen stack.ListenConfig
			if scenario.cone {
				listen = ttRecordingListenConfig(calls)
			}
			device := ttNewDevice()
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- Serve(ctx, ttOptions(device, ttRecordingDialer(calls), listen)) }()
			defer func() {
				cancel()
				ttServed(test, done)
			}()
			device.inbound <- ttUDPPacket(client, netip.MustParseAddrPort(scenario.target), []byte("query"))
			select {
			case got := <-calls:
				if got != scenario.want {
					test.Fatalf("served %+v, want %+v", got, scenario.want)
				}
			case <-time.After(ttWait):
				test.Fatalf("nothing served, want %+v", scenario.want)
			}
		})
	}
}

// ttOptions serves device with dialer and listen.
func ttOptions(device *ttDevice, dialer stack.Dialer, listen stack.ListenConfig) Options {
	return Options{
		Config:       ttConfig,
		Open:         func(context.Context, tun.Config) (*tun.Device, error) { return &tun.Device{Device: device}, nil },
		Dialer:       dialer,
		ListenConfig: listen,
	}
}

// ttServed waits for Serve to return and returns its error.
func ttServed(test *testing.T, done <-chan error) error {
	test.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(ttWait):
		test.Fatal("Serve did not return")
		return nil
	}
}

// ttUDPPacket is an IPv4 UDP datagram from source to target, without a UDP checksum.
func ttUDPPacket(source, target netip.AddrPort, payload []byte) []byte {
	packet := make([]byte, 28+len(payload))
	packet[0] = 0x45
	binary.BigEndian.PutUint16(packet[2:], uint16(len(packet)))
	packet[8] = 64
	packet[9] = 17
	from, to := source.Addr().As4(), target.Addr().As4()
	copy(packet[12:16], from[:])
	copy(packet[16:20], to[:])
	var sum uint32
	for index := 0; index < 20; index += 2 {
		sum += uint32(binary.BigEndian.Uint16(packet[index:]))
	}
	for sum>>16 != 0 {
		sum = sum&0xffff + sum>>16
	}
	binary.BigEndian.PutUint16(packet[10:], ^uint16(sum))
	binary.BigEndian.PutUint16(packet[20:], source.Port())
	binary.BigEndian.PutUint16(packet[22:], target.Port())
	binary.BigEndian.PutUint16(packet[24:], uint16(8+len(payload)))
	copy(packet[28:], payload)
	return packet
}

// ttDevice is a TUN that reads the packets sent on inbound, or fails with readErr, and drops every packet written.
type ttDevice struct {
	inbound  chan []byte
	readErr  error
	events   chan wgtun.Event
	read     chan struct{}
	readOnce sync.Once
	done     chan struct{}
	once     sync.Once
}

func ttNewDevice() *ttDevice {
	return &ttDevice{inbound: make(chan []byte), events: make(chan wgtun.Event), read: make(chan struct{}), done: make(chan struct{})}
}

func (device *ttDevice) File() *os.File                               { return nil }
func (device *ttDevice) Write(bufs [][]byte, offset int) (int, error) { return len(bufs), nil }
func (device *ttDevice) MTU() (int, error)                            { return 1500, nil }
func (device *ttDevice) Name() (string, error)                        { return "tt9", nil }
func (device *ttDevice) Events() <-chan wgtun.Event                   { return device.events }
func (device *ttDevice) BatchSize() int                               { return 1 }

func (device *ttDevice) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	device.readOnce.Do(func() { close(device.read) })
	if device.readErr != nil {
		return 0, device.readErr
	}
	select {
	case packet := <-device.inbound:
		sizes[0] = copy(bufs[0][offset:], packet)
		return 1, nil
	case <-device.done:
		return 0, os.ErrClosed
	}
}

func (device *ttDevice) Close() error {
	device.once.Do(func() {
		close(device.done)
		close(device.events)
	})
	return nil
}

func (device *ttDevice) isClosed() bool {
	select {
	case <-device.done:
		return true
	default:
		return false
	}
}

type ttDial struct{ network, address, client string }

// ttRecordingDialer sends each dial, with the captured flow's client, to its channel and fails it.
type ttRecordingDialer chan ttDial

func (dials ttRecordingDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	select {
	case dials <- ttDial{network: network, address: address, client: ttClient(ctx)}:
	default:
	}
	return nil, errors.New("refused")
}

// ttRecordingListenConfig sends each packet listen, with the captured session's client, to its channel and fails it.
type ttRecordingListenConfig chan ttDial

func (listens ttRecordingListenConfig) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return nil, errors.ErrUnsupported
}

func (listens ttRecordingListenConfig) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	select {
	case listens <- ttDial{network: network, address: address, client: ttClient(ctx)}:
	default:
	}
	return nil, errors.New("refused")
}

// ttClient is the captured flow's client in ctx, or empty.
func ttClient(ctx context.Context) string {
	if client := stack.ClientAddr(ctx); client != nil {
		return client.String()
	}
	return ""
}

// ttFlow is a captured flow whose session sends one datagram to its target.
type ttFlow struct {
	network        string
	target, client netip.AddrPort
}

func (flow *ttFlow) Network() string        { return flow.network }
func (flow *ttFlow) Target() netip.AddrPort { return flow.target }
func (flow *ttFlow) Client() netip.AddrPort { return flow.client }
func (flow *ttFlow) Reject()                {}

func (flow *ttFlow) Accept() (net.Conn, error) {
	return &ttSessionConn{target: flow.target, closed: make(chan struct{})}, nil
}

// ttSessionConn is a UDP session that reads one datagram, then blocks until closed.
type ttSessionConn struct {
	net.Conn
	target    netip.AddrPort
	sent      atomic.Bool
	closed    chan struct{}
	closeOnce sync.Once
}

func (conn *ttSessionConn) ReadFrom(b []byte) (int, net.Addr, error) {
	if !conn.sent.Swap(true) {
		return copy(b, "query"), net.UDPAddrFromAddrPort(conn.target), nil
	}
	<-conn.closed
	return 0, nil, net.ErrClosed
}

func (conn *ttSessionConn) WriteTo(b []byte, _ net.Addr) (int, error) { return len(b), nil }

func (conn *ttSessionConn) Close() error {
	conn.closeOnce.Do(func() { close(conn.closed) })
	return nil
}
