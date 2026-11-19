package jumpway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/socks5"
)

const udpTestWait = 5 * time.Second

// recordingPacketConn records the destinations written to and signals its first Close.
type recordingPacketConn struct {
	net.PacketConn
	mu           sync.Mutex
	destinations []net.Addr
	closeOnce    sync.Once
	closed       chan struct{}
}

func newRecordingPacketConn(conn net.PacketConn) *recordingPacketConn {
	return &recordingPacketConn{PacketConn: conn, closed: make(chan struct{})}
}

func (c *recordingPacketConn) WriteTo(payload []byte, addr net.Addr) (int, error) {
	c.mu.Lock()
	c.destinations = append(c.destinations, addr)
	c.mu.Unlock()
	return c.PacketConn.WriteTo(payload, addr)
}

func (c *recordingPacketConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.PacketConn.Close()
}

func (c *recordingPacketConn) Destinations() []net.Addr {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]net.Addr(nil), c.destinations...)
}

// startUDPEcho answers every datagram with "<payload>|<sender port>".
func startUDPEcho(test *testing.T) net.Addr {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { conn.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, from, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			reply := fmt.Sprintf("%s|%d", buf[:n], from.(*net.UDPAddr).Port)
			conn.WriteTo([]byte(reply), from)
		}
	}()
	return conn.LocalAddr()
}

func startSOCKS5Proxy(test *testing.T, dialer bridge.Dialer) string {
	ctx, cancel := context.WithCancel(context.Background())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		test.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- RunProxy(ctx, listener, dialer, []Scheme{{Type: "socks5"}}) }()
	test.Cleanup(func() {
		cancel()
		listener.Close()
		select {
		case <-done:
		case <-time.After(udpTestWait):
			test.Error("RunProxy did not stop after closing its listener")
		}
	})
	return listener.Addr().String()
}

// associate opens a UDP association through the proxy and reports the control connection's local address.
func associate(test *testing.T, proxyAddress string, target net.Addr) (net.PacketConn, string, error) {
	client, err := socks5.NewDialer("socks5://" + proxyAddress)
	if err != nil {
		test.Fatal(err)
	}
	var controlAddr string
	client.ProxyDial = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		controlAddr = conn.LocalAddr().String()
		return conn, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), udpTestWait)
	defer cancel()
	conn, err := client.DialContext(ctx, "udp", target.String())
	if err != nil {
		return nil, controlAddr, err
	}
	test.Cleanup(func() { conn.Close() })
	packetConn, ok := conn.(net.PacketConn)
	if !ok {
		test.Fatalf("socks5 UDP conn %T is not a net.PacketConn", conn)
	}
	return packetConn, controlAddr, nil
}

// exchange sends payload to target through the association and returns the reply and its source.
func exchange(test *testing.T, conn net.PacketConn, target net.Addr, payload string) (string, net.Addr) {
	test.Helper()
	if err := conn.SetDeadline(time.Now().Add(udpTestWait)); err != nil {
		test.Fatal(err)
	}
	if _, err := conn.WriteTo([]byte(payload), target); err != nil {
		test.Fatal(err)
	}
	buf := make([]byte, 2048)
	n, from, err := conn.ReadFrom(buf)
	if err != nil {
		test.Fatalf("reading reply to %q: %v", payload, err)
	}
	return string(buf[:n]), from
}

type listenPacketCall struct {
	network, address, clientAddr string
	conn                         *recordingPacketConn
}

// recordingDialer opens real target sockets and reports every ListenPacket call.
func recordingDialer(calls *atomic.Int32, record chan<- listenPacketCall) packetDialer {
	return packetDialer{
		Dialer: local.LOCAL,
		ListenPacketConfig: bridge.ListenPacketConfigFunc(func(ctx context.Context, network, address string) (net.PacketConn, error) {
			calls.Add(1)
			conn, err := (&net.ListenConfig{}).ListenPacket(ctx, network, address)
			if err != nil {
				return nil, err
			}
			recorder := newRecordingPacketConn(conn)
			record <- listenPacketCall{network: network, address: address, clientAddr: ClientAddr(ctx), conn: recorder}
			return recorder, nil
		}),
	}
}

func receiveCall(test *testing.T, record <-chan listenPacketCall) listenPacketCall {
	test.Helper()
	select {
	case call := <-record:
		return call
	case <-time.After(udpTestWait):
		test.Fatal("the dialer's ListenPacket was not called")
		return listenPacketCall{}
	}
}

func replyPort(test *testing.T, reply string) int {
	test.Helper()
	_, port, ok := strings.Cut(reply, "|")
	if !ok {
		test.Fatalf("echo reply %q has no sender port", reply)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		test.Fatal(err)
	}
	return n
}

func TestRunProxySOCKS5AssociateCallsDialerListenPacketOnceBeforeReply(test *testing.T) {
	var calls atomic.Int32
	record := make(chan listenPacketCall, 4)
	proxy := startSOCKS5Proxy(test, recordingDialer(&calls, record))
	echo := startUDPEcho(test)

	conn, _, err := associate(test, proxy, echo)
	if err != nil {
		test.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		test.Fatalf("ListenPacket calls before the ASSOCIATE reply = %d, want 1", got)
	}
	call := receiveCall(test, record)
	if call.network != "udp" || call.address != ":0" {
		test.Errorf("ListenPacket(%q, %q), want (\"udp\", \":0\")", call.network, call.address)
	}
	exchange(test, conn, echo, "ping")
	if got := calls.Load(); got != 1 {
		test.Errorf("ListenPacket calls for one association = %d, want 1", got)
	}
}

func TestRunProxySOCKS5AssociateListenPacketContextCarriesClientAddr(test *testing.T) {
	var calls atomic.Int32
	record := make(chan listenPacketCall, 4)
	proxy := startSOCKS5Proxy(test, recordingDialer(&calls, record))
	echo := startUDPEcho(test)

	_, controlAddr, err := associate(test, proxy, echo)
	if err != nil {
		test.Fatal(err)
	}
	call := receiveCall(test, record)
	if call.clientAddr != controlAddr {
		test.Errorf("ClientAddr(ctx) in ListenPacket = %q, want the control connection's %q", call.clientAddr, controlAddr)
	}
}

func TestRunProxySOCKS5AssociateSendsAllDestinationsThroughDialerSocket(test *testing.T) {
	var calls atomic.Int32
	record := make(chan listenPacketCall, 4)
	proxy := startSOCKS5Proxy(test, recordingDialer(&calls, record))
	echoes := []net.Addr{startUDPEcho(test), startUDPEcho(test)}

	conn, _, err := associate(test, proxy, echoes[0])
	if err != nil {
		test.Fatal(err)
	}
	call := receiveCall(test, record)
	socketPort := call.conn.LocalAddr().(*net.UDPAddr).Port
	for i, echo := range echoes {
		payload := fmt.Sprintf("datagram-%d", i)
		reply, from := exchange(test, conn, echo, payload)
		if from.String() != echo.String() {
			test.Errorf("reply source = %s, want %s", from, echo)
		}
		if !strings.HasPrefix(reply, payload+"|") {
			test.Errorf("reply = %q, want echo of %q", reply, payload)
		}
		if port := replyPort(test, reply); port != socketPort {
			test.Errorf("echo %s saw sender port %d, want the dialer socket's %d", echo, port, socketPort)
		}
	}
	destinations := call.conn.Destinations()
	if len(destinations) != len(echoes) {
		test.Fatalf("dialer socket WriteTo destinations = %v, want %v", destinations, echoes)
	}
	for i, destination := range destinations {
		udpAddr, ok := destination.(*net.UDPAddr)
		if !ok {
			test.Errorf("WriteTo destination %v is %T, want *net.UDPAddr", destination, destination)
			continue
		}
		if udpAddr.String() != echoes[i].String() {
			test.Errorf("WriteTo destination %d = %s, want %s", i, udpAddr, echoes[i])
		}
	}
	if got := calls.Load(); got != 1 {
		test.Errorf("ListenPacket calls for two destinations = %d, want 1", got)
	}
}

func TestRunProxySOCKS5AssociateClosesDialerSocketWhenClientCloses(test *testing.T) {
	var calls atomic.Int32
	record := make(chan listenPacketCall, 4)
	proxy := startSOCKS5Proxy(test, recordingDialer(&calls, record))
	echo := startUDPEcho(test)

	conn, _, err := associate(test, proxy, echo)
	if err != nil {
		test.Fatal(err)
	}
	call := receiveCall(test, record)
	exchange(test, conn, echo, "ping")
	conn.Close()
	select {
	case <-call.conn.closed:
	case <-time.After(udpTestWait):
		test.Fatal("the dialer socket stayed open after the client closed its association")
	}
}

func TestRunProxySOCKS5AssociateFailsWhenListenPacketRefuses(test *testing.T) {
	var calls atomic.Int32
	dialer := packetDialer{
		Dialer: local.LOCAL,
		ListenPacketConfig: bridge.ListenPacketConfigFunc(func(ctx context.Context, network, address string) (net.PacketConn, error) {
			calls.Add(1)
			return nil, errors.New("udp refused by rule")
		}),
	}
	proxy := startSOCKS5Proxy(test, dialer)
	echo := startUDPEcho(test)

	conn, _, err := associate(test, proxy, echo)
	if err == nil {
		conn.Close()
		test.Fatal("ASSOCIATE succeeded although the dialer's ListenPacket refused")
	}
	if got := calls.Load(); got != 1 {
		test.Errorf("ListenPacket calls = %d, want 1", got)
	}
}

func TestRunProxySOCKS5AssociateClosesDialerSocketWhenRelayFails(test *testing.T) {
	var calls atomic.Int32
	record := make(chan listenPacketCall, 4)
	proxy := startSOCKS5Proxy(test, recordingDialer(&calls, record))

	control, err := net.DialTimeout("tcp", proxy, udpTestWait)
	if err != nil {
		test.Fatal(err)
	}
	defer control.Close()
	if err := control.SetDeadline(time.Now().Add(udpTestWait)); err != nil {
		test.Fatal(err)
	}
	// 192.0.2.1 (TEST-NET-1) is never local, so the relay socket cannot bind to it.
	request := []byte{5, 1, 0, 5, 3, 0, 1, 192, 0, 2, 1, 0, 0}
	if _, err := control.Write(request); err != nil {
		test.Fatal(err)
	}
	reply := make([]byte, 5)
	if _, err := io.ReadFull(control, reply); err != nil {
		test.Fatal(err)
	}
	if reply[3] == 0 {
		test.Fatalf("ASSOCIATE to an unbindable relay address got success reply %v", reply)
	}
	call := receiveCall(test, record)
	select {
	case <-call.conn.closed:
	case <-time.After(udpTestWait):
		test.Fatal("the dialer socket stayed open after the ASSOCIATE failed")
	}
}

func TestRunProxySOCKS5AssociateWithoutListenPacketRelaysLocally(test *testing.T) {
	dialer := bridge.DialFunc(local.LOCAL.DialContext)
	if _, ok := any(dialer).(bridge.ListenPacketConfig); ok {
		test.Fatal("test dialer must not implement bridge.ListenPacketConfig")
	}
	proxy := startSOCKS5Proxy(test, dialer)
	echoes := []net.Addr{startUDPEcho(test), startUDPEcho(test)}

	conn, _, err := associate(test, proxy, echoes[0])
	if err != nil {
		test.Fatal(err)
	}
	for i, echo := range echoes {
		payload := fmt.Sprintf("datagram-%d", i)
		reply, from := exchange(test, conn, echo, payload)
		if from.String() != echo.String() {
			test.Errorf("reply source = %s, want %s", from, echo)
		}
		if !strings.HasPrefix(reply, payload+"|") {
			test.Errorf("reply = %q, want echo of %q", reply, payload)
		}
	}
	relay := rawAssociate(test, proxy)
	reply := rawExchange(test, relay, echoes[1].(*net.UDPAddr), "raw")
	if port := replyPort(test, reply); port != relay.Port {
		test.Errorf("echo saw sender port %d, want BND.PORT %d of the relay socket the client was told", port, relay.Port)
	}
}

// rawAssociate opens a UDP association by hand and returns the relay address of its reply, on loopback.
func rawAssociate(test *testing.T, proxy string) *net.UDPAddr {
	test.Helper()
	control, err := net.DialTimeout("tcp", proxy, udpTestWait)
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { control.Close() })
	if err := control.SetDeadline(time.Now().Add(udpTestWait)); err != nil {
		test.Fatal(err)
	}
	if _, err := control.Write([]byte{5, 1, 0, 5, 3, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		test.Fatal(err)
	}
	reply := make([]byte, 2+4)
	if _, err := io.ReadFull(control, reply); err != nil {
		test.Fatal(err)
	}
	if reply[3] != 0 {
		test.Fatalf("ASSOCIATE reply %v, want success", reply)
	}
	size := map[byte]int{1: net.IPv4len, 4: net.IPv6len}[reply[5]]
	if size == 0 {
		test.Fatalf("ASSOCIATE reply address type %d, want an IP", reply[5])
	}
	bound := make([]byte, size+2)
	if _, err := io.ReadFull(control, bound); err != nil {
		test.Fatal(err)
	}
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(bound[size])<<8 | int(bound[size+1])}
}

// rawExchange sends payload to target through relay with a SOCKS5 UDP header and returns the reply's payload.
func rawExchange(test *testing.T, relay, target *net.UDPAddr, payload string) string {
	test.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(udpTestWait)); err != nil {
		test.Fatal(err)
	}
	datagram := append([]byte{0, 0, 0, 1}, target.IP.To4()...)
	datagram = append(datagram, byte(target.Port>>8), byte(target.Port))
	if _, err := conn.WriteTo(append(datagram, payload...), relay); err != nil {
		test.Fatal(err)
	}
	buf := make([]byte, 2048)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		test.Fatalf("reading the relayed reply to %q: %v", payload, err)
	}
	if n < len(datagram) {
		test.Fatalf("relayed reply %v is shorter than its header", buf[:n])
	}
	return string(buf[len(datagram):n])
}
