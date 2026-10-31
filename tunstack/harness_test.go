package tunstack

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzshiming/jumpway"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
)

const (
	testMTU   = 1500
	testBatch = 4
	// testWait bounds every wait for something that must happen.
	testWait = 10 * time.Second
	// testQuiet is how long the client stack must stay silent before it is closed.
	testQuiet = 100 * time.Millisecond
)

var (
	testClient4   = netip.MustParseAddr("10.0.0.2")
	testClient6   = netip.MustParseAddr("fd00::2")
	testAddresses = []netip.Prefix{netip.MustParsePrefix("10.0.0.1/24"), netip.MustParsePrefix("fd00::1/64")}
)

// fakeDevice is the tun.Device handed to Serve, wired back-to-back to a netstack client device.
type fakeDevice struct {
	client     tun.Device
	inbound    chan []byte
	failNext   chan error
	readFailed chan struct{}
	readErr    error
	failOnce   sync.Once
	closed     chan struct{}
	closeOnce  sync.Once
	closeCalls atomic.Int32
	events     chan tun.Event
	feeding    sync.Mutex
}

func (d *fakeDevice) File() *os.File           { return nil }
func (d *fakeDevice) Name() (string, error)    { return "fake0", nil }
func (d *fakeDevice) MTU() (int, error)        { return testMTU, nil }
func (d *fakeDevice) Events() <-chan tun.Event { return d.events }
func (d *fakeDevice) BatchSize() int           { return testBatch }

func (d *fakeDevice) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	var packet []byte
	select {
	case packet = <-d.inbound:
	case err := <-d.failNext:
		return 0, err
	case <-d.readFailed:
		return 0, d.readErr
	case <-d.closed:
		return 0, os.ErrClosed
	}
	n := 0
	for {
		sizes[n] = copy(bufs[n][offset:], packet)
		n++
		if n == len(bufs) {
			return n, nil
		}
		select {
		case packet = <-d.inbound:
		default:
			return n, nil
		}
	}
}

func (d *fakeDevice) Write(bufs [][]byte, offset int) (int, error) {
	select {
	case <-d.closed:
		return 0, os.ErrClosed
	default:
	}
	for _, buf := range bufs {
		d.client.Write([][]byte{buf[offset:]}, 0)
	}
	return len(bufs), nil
}

func (d *fakeDevice) Close() error {
	d.closeCalls.Add(1)
	d.release()
	return nil
}

// release stops the device without counting as a Close by Serve.
func (d *fakeDevice) release() {
	d.closeOnce.Do(func() {
		close(d.closed)
		// A send pending on a closed channel panics, so a feeder must stop before events closes.
		d.feeding.Lock()
		close(d.events)
		d.feeding.Unlock()
	})
}

// inject queues a client packet for Serve's next Read, dropping it once the device is closed.
func (d *fakeDevice) inject(packet []byte) {
	select {
	case d.inbound <- packet:
	case <-d.closed:
	}
}

// failRead makes every Read from now on fail with err.
func (d *fakeDevice) failRead(err error) {
	d.failOnce.Do(func() {
		d.readErr = err
		close(d.readFailed)
	})
}

// feedEvents sends n events, each kind in turn, on Events from a goroutine, counting those received and closing all after the last; release ends it.
func (d *fakeDevice) feedEvents(n int) (received *atomic.Int32, all <-chan struct{}) {
	count, done := new(atomic.Int32), make(chan struct{})
	go func() {
		d.feeding.Lock()
		defer d.feeding.Unlock()
		// release may have closed events already.
		select {
		case <-d.closed:
			return
		default:
		}
		kinds := []tun.Event{tun.EventUp, tun.EventDown, tun.EventMTUUpdate}
		for i := range n {
			select {
			case d.events <- kinds[i%len(kinds)]:
				count.Add(1)
			case <-d.closed:
				return
			}
		}
		close(done)
	}()
	return count, done
}

func (d *fakeDevice) closedByServe() bool { return d.closeCalls.Load() > 0 }

// fakeDialer records every dial and answers it with one end of a fresh loopback TCP connection.
type fakeDialer struct {
	mu      sync.Mutex
	dials   []fakeDial
	err     error
	opened  []net.Conn
	targets chan net.Conn
}

// fakeDial is one recorded DialContext call; client is jumpway.ClientAddr of its context.
type fakeDial struct {
	network, address, client string
}

func (d *fakeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d.mu.Lock()
	d.dials = append(d.dials, fakeDial{network: network, address: address, client: jumpway.ClientAddr(ctx)})
	err := d.err
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	conn, target, err := testLoopbackPair()
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	d.opened = append(d.opened, target)
	d.mu.Unlock()
	select {
	case d.targets <- target:
	default:
	}
	return conn, nil
}

func (d *fakeDialer) fail(err error) {
	d.mu.Lock()
	d.err = err
	d.mu.Unlock()
}

func (d *fakeDialer) recorded() []fakeDial {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.dials)
}

// expectOnly expects exactly one dial, of "tcp" to address, tagged with client's address.
func (d *fakeDialer) expectOnly(t *testing.T, address string, client net.Addr) {
	t.Helper()
	dials := d.recorded()
	if len(dials) != 1 {
		t.Fatalf("dialer.DialContext called %d times %+v, want once", len(dials), dials)
	}
	if got := dials[0]; got.network != "tcp" || got.address != address {
		t.Fatalf(`dialer.DialContext(%q, %q), want ("tcp", %q)`, got.network, got.address, address)
	}
	if got, want := dials[0].client, client.String(); got != want {
		t.Fatalf("jumpway.ClientAddr(dial ctx) = %q, want the client's %q", got, want)
	}
}

func (d *fakeDialer) closeTargets() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, conn := range d.opened {
		conn.Close()
	}
}

// testLoopbackPair returns both ends of a real loopback TCP connection.
func testLoopbackPair() (conn, target net.Conn, err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	defer listener.Close()
	conn, err = net.Dial("tcp", listener.Addr().String())
	if err != nil {
		return nil, nil, err
	}
	target, err = listener.Accept()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, target, nil
}

// testEnv is one running Serve whose device is wired to a netstack client.
type testEnv struct {
	dev    *fakeDevice
	dialer *fakeDialer
	client *netstack.Net
	cancel context.CancelFunc
	// alive ends when Serve returns, so client operations fail fast if it returns early.
	alive context.Context
	done  chan struct{}
	err   error
}

func testServe(t *testing.T, opts Options) *testEnv {
	t.Helper()
	clientDev, client, err := netstack.CreateNetTUN([]netip.Addr{testClient4, testClient6}, nil, testMTU)
	if err != nil {
		t.Fatalf("netstack.CreateNetTUN: %v", err)
	}
	dev := &fakeDevice{
		client:     clientDev,
		inbound:    make(chan []byte, 1024),
		failNext:   make(chan error),
		readFailed: make(chan struct{}),
		closed:     make(chan struct{}),
		events:     make(chan tun.Event), // unbuffered: a feedEvents send completes only once Serve receives it
	}
	dialer := &fakeDialer{targets: make(chan net.Conn, 16)}
	t.Cleanup(dialer.closeTargets)

	var lastSent atomic.Int64
	pumped := make(chan struct{})
	go func() {
		defer close(pumped)
		bufs := [][]byte{make([]byte, 65535)}
		sizes := make([]int, 1)
		for {
			n, err := clientDev.Read(bufs, sizes, 0)
			if err != nil {
				return
			}
			lastSent.Store(time.Now().UnixNano())
			for i := range n {
				dev.inject(bytes.Clone(bufs[i][:sizes[i]]))
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	alive, stop := context.WithCancel(context.Background())
	env := &testEnv{dev: dev, dialer: dialer, client: client, cancel: cancel, alive: alive, done: make(chan struct{})}
	go func() {
		env.err = Serve(ctx, dev, dialer, opts)
		close(env.done)
		stop()
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-env.done:
		case <-time.After(testWait):
			t.Errorf("Serve did not return within %v after its context ended", testWait)
		}
		dev.release()
		// netstack's Close panics a packet send still in flight, so let the client stack fall silent first.
		for limit := time.Now().Add(2 * time.Second); time.Now().Before(limit) && time.Since(time.Unix(0, lastSent.Load())) < testQuiet; {
			time.Sleep(10 * time.Millisecond)
		}
		clientDev.Close()
		select {
		case <-pumped:
		case <-time.After(testWait):
			t.Errorf("client packet pump did not stop")
		}
	})
	return env
}

// returned describes Serve's result for failure messages once it has returned.
func (e *testEnv) returned() string {
	select {
	case <-e.done:
		return fmt.Sprintf(" (Serve already returned %v)", e.err)
	default:
		return ""
	}
}

func (e *testEnv) wait(t *testing.T) error {
	t.Helper()
	select {
	case <-e.done:
		return e.err
	case <-time.After(testWait):
		t.Fatalf("Serve did not return within %v", testWait)
		return nil
	}
}

func (e *testEnv) dialTCP(t *testing.T, address string) *gonet.TCPConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(e.alive, testWait)
	defer cancel()
	conn, err := e.client.DialContextTCPAddrPort(ctx, netip.MustParseAddrPort(address))
	if err != nil {
		t.Fatalf("client dial tcp %s through Serve: %v%s", address, err, e.returned())
	}
	conn.SetDeadline(time.Now().Add(testWait))
	return conn
}

func (e *testEnv) dialUDP(t *testing.T, address string) *gonet.UDPConn {
	t.Helper()
	conn, err := e.client.DialUDPAddrPort(netip.AddrPort{}, netip.MustParseAddrPort(address))
	if err != nil {
		t.Fatalf("client dial udp %s: %v", address, err)
	}
	conn.SetDeadline(time.Now().Add(testWait))
	return conn
}

// nextTarget returns the target end of the next connection Serve dialed.
func (e *testEnv) nextTarget(t *testing.T) net.Conn {
	t.Helper()
	select {
	case target := <-e.dialer.targets:
		target.SetDeadline(time.Now().Add(testWait))
		return target
	case <-e.done:
		t.Fatalf("Serve returned %v without dialing through the dialer", e.err)
	case <-time.After(testWait):
		t.Fatalf("Serve did not dial through the dialer within %v", testWait)
	}
	return nil
}

// failOneRead makes one device Read return err instead of packets, returning once Serve's Read has taken it.
func (e *testEnv) failOneRead(t *testing.T, err error) {
	t.Helper()
	select {
	case e.dev.failNext <- err:
	case <-e.done:
		t.Fatalf("Serve returned %v before its device Read could fail with %v", e.err, err)
	case <-time.After(testWait):
		t.Fatalf("Serve did not Read the device within %v", testWait)
	}
}

// testRelay writes payload on from and expects it to arrive on to.
func testRelay(t *testing.T, from, to net.Conn, payload string) {
	t.Helper()
	if _, err := io.WriteString(from, payload); err != nil {
		t.Fatalf("write %q: %v", payload, err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(to, got); err != nil {
		t.Fatalf("read %q on the other side of Serve: %v", payload, err)
	}
	if string(got) != payload {
		t.Fatalf("relayed %q, want %q", got, payload)
	}
}

func testReadEOF(t *testing.T, conn net.Conn, what string) {
	t.Helper()
	n, err := conn.Read(make([]byte, 1))
	if n != 0 || err != io.EOF {
		t.Fatalf("%s: Read() = %d, %v, want 0, EOF", what, n, err)
	}
}
