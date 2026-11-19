package tray

import (
	"context"
	"net"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/socks5"
)

func TestReloadListenerTakesUDPFromTheRuleOnlyWithUDPMode(test *testing.T) {
	for _, scenario := range []struct {
		name   string
		mode   string
		packet bool
	}{
		{name: "absent"},
		{name: config.UDPModeProxy, mode: config.UDPModeProxy, packet: true},
		{name: config.UDPModeDirect, mode: config.UDPModeDirect, packet: true},
		{name: config.UDPModeBlock, mode: config.UDPModeBlock, packet: true},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			dialers := make(chan bridge.Dialer, 1)
			previous := runProxy
			test.Cleanup(func() { runProxy = previous })
			runProxy = func(ctx context.Context, listener net.Listener, dialer bridge.Dialer, schemes []jumpway.Scheme) error {
				dialers <- dialer
				return previous(ctx, listener, dialer, schemes)
			}
			ttListenerUDPApp(test, "127.0.0.1:1", config.UDP{Mode: scenario.mode})
			select {
			case dialer := <-dialers:
				if _, packet := dialer.(bridge.ListenPacketConfig); packet != scenario.packet {
					test.Fatalf("listener dialer %T implements bridge.ListenPacketConfig = %v, want %v", dialer, packet, scenario.packet)
				}
			case <-time.After(ttWait):
				test.Fatalf("the rule's listener was not served within %v", ttWait)
			}
		})
	}
}

func TestListenerUDPProxyFullConeIsOneWayAssociation(test *testing.T) {
	way, associations := ttStartWaySOCKS5(test)
	app, listener := ttListenerUDPApp(test, way, config.UDP{Mode: config.UDPModeProxy, NAT: config.NATFullCone})
	conn, control := ttListenerAssociate(test, listener)
	echoes := ttAssertListenerEchoes(test, conn)
	recorded := associations.recorded()
	if len(recorded) != 1 {
		test.Fatalf("way associations %d, want one", len(recorded))
	}
	if sent, want := slices.Sorted(slices.Values(recorded[0].sent())), ttAddrStrings(echoes); !slices.Equal(sent, want) {
		test.Fatalf("way association sent to %v, want %v", sent, want)
	}
	ttAssertListenerUDPConnection(test, app, control, echoes)
}

func TestListenerUDPSymmetricIsOneWayAssociationPerDestination(test *testing.T) {
	way, associations := ttStartWaySOCKS5(test)
	_, listener := ttListenerUDPApp(test, way, config.UDP{Mode: config.UDPModeProxy, NAT: config.NATSymmetric})
	conn, _ := ttListenerAssociate(test, listener)
	echoes := ttAssertListenerEchoes(test, conn)
	var sent []string
	for _, association := range associations.recorded() {
		destinations := association.sent()
		if len(destinations) != 1 {
			test.Fatalf("way association sent to %v, want one destination", destinations)
		}
		sent = append(sent, destinations[0])
	}
	if slices.Sort(sent); !slices.Equal(sent, ttAddrStrings(echoes)) {
		test.Fatalf("way associations sent to %v, want one per destination %v", sent, ttAddrStrings(echoes))
	}
}

func TestListenerUDPDirectLeavesWithoutTheWay(test *testing.T) {
	way, associations := ttStartWaySOCKS5(test)
	app, listener := ttListenerUDPApp(test, way, config.UDP{Mode: config.UDPModeDirect})
	conn, control := ttListenerAssociate(test, listener)
	echoes := ttAssertListenerEchoes(test, conn)
	if recorded := associations.recorded(); len(recorded) != 0 {
		test.Fatalf("way associations %d, want none", len(recorded))
	}
	ttAssertListenerUDPConnection(test, app, control, echoes)
}

func TestListenerUDPDirectSymmetricIsOneSocketPerDestination(test *testing.T) {
	way, associations := ttStartWaySOCKS5(test)
	app, listener := ttListenerUDPApp(test, way, config.UDP{Mode: config.UDPModeDirect, NAT: config.NATSymmetric})
	conn, control := ttListenerAssociate(test, listener)
	echoes := []*net.UDPAddr{ttUDPPortEcho(test), ttUDPPortEcho(test)}
	ports := map[string]string{}
	for _, echo := range []*net.UDPAddr{echoes[0], echoes[1], echoes[0]} {
		port := ttListenerSenderPort(test, conn, echo)
		if seen, ok := ports[echo.String()]; ok && seen != port {
			test.Fatalf("%s saw sender ports %s and %s, want one socket per destination", echo, seen, port)
		}
		ports[echo.String()] = port
	}
	if ports[echoes[0].String()] == ports[echoes[1].String()] {
		test.Fatalf("both destinations saw sender port %s, want a socket each", ports[echoes[0].String()])
	}
	if recorded := associations.recorded(); len(recorded) != 0 {
		test.Fatalf("way associations %d, want none", len(recorded))
	}
	ttAssertListenerUDPConnection(test, app, control, echoes)
}

func TestListenerUDPProxyWithoutWayLeavesDirectly(test *testing.T) {
	app, listener := ttListenerUDPApp(test, "", config.UDP{Mode: config.UDPModeProxy})
	conn, control := ttListenerAssociate(test, listener)
	echoes := []*net.UDPAddr{ttUDPPortEcho(test), ttUDPPortEcho(test)}
	first, second := ttListenerSenderPort(test, conn, echoes[0]), ttListenerSenderPort(test, conn, echoes[1])
	if first != second {
		test.Fatalf("destinations saw sender ports %s and %s, want the one socket of a direct session", first, second)
	}
	ttAssertListenerUDPConnection(test, app, control, echoes)
}

func TestListenerUDPBlockRefusesAssociate(test *testing.T) {
	way, associations := ttStartWaySOCKS5(test)
	_, listener := ttListenerUDPApp(test, way, config.UDP{Mode: config.UDPModeBlock})
	client, err := socks5.NewDialer("socks5://" + listener)
	if err != nil {
		test.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), ttWait)
	defer cancel()
	conn, err := client.DialContext(ctx, "udp", ttUDPEcho(test).String())
	if err == nil {
		conn.Close()
		test.Fatal("UDP dial through a block rule's listener succeeded, want it refused")
	}
	if recorded := associations.recorded(); len(recorded) != 0 {
		test.Fatalf("way associations %d, want none", len(recorded))
	}
}

func TestListenerUDPWithoutModeLeavesWithoutTheWay(test *testing.T) {
	way, associations := ttStartWaySOCKS5(test)
	_, listener := ttListenerUDPApp(test, way, config.UDP{})
	conn, _ := ttListenerAssociate(test, listener)
	ttAssertListenerEchoes(test, conn)
	if recorded := associations.recorded(); len(recorded) != 0 {
		test.Fatalf("way associations %d, want none", len(recorded))
	}
}

func TestListenerUDPIdleEndsIdleWayAssociation(test *testing.T) {
	way, associations := ttStartWaySOCKS5(test)
	_, listener := ttListenerUDPApp(test, way, config.UDP{Mode: config.UDPModeProxy, NAT: config.NATSymmetric, Idle: 1})
	conn, _ := ttListenerAssociate(test, listener)
	guard := time.AfterFunc(2*ttWait, func() { conn.Close() })
	defer guard.Stop()
	idled, other := ttUDPEcho(test), ttUDPEcho(test)
	exchange := func(to net.Addr) {
		if _, err := conn.WriteTo([]byte("x"), to); err != nil {
			test.Fatal(err)
		}
		if _, _, err := conn.ReadFrom(make([]byte, 8)); err != nil {
			test.Fatal(err)
		}
	}
	exchange(idled)
	recorded := associations.recorded()
	if len(recorded) != 1 {
		test.Fatalf("way associations %d after one destination, want one", len(recorded))
	}
	deadline := time.After(ttWait)
	for {
		select {
		case <-recorded[0].closed:
			return
		case <-time.After(100 * time.Millisecond):
			exchange(other)
		case <-deadline:
			test.Fatalf("way association still open %v after it idled past udp.idle of 1s", ttWait)
		}
	}
}

// ttListenerUDPApp reloads an app with one SOCKS5 rule whose way is the SOCKS5 proxy at way, or none when empty, and
// whose forward.udp is udp, and returns it with the rule's listener address.
func ttListenerUDPApp(test *testing.T, way string, udp config.UDP) (*App, string) {
	test.Helper()
	envShuntSetenv(test, "", "")
	forward := config.Forward{UDP: udp}
	if way != "" {
		forward.Way = []bridgeconfig.Node{{LB: []string{"socks5://" + way}}}
	}
	app := newTestApp(test, &config.Config{Rules: []config.Rule{{
		Name:    "rule",
		Listen:  config.Listen{Host: "127.0.0.1", Protocols: []config.Protocol{{Type: config.ProtocolSOCKS5}}},
		Forward: forward,
	}}})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	status := app.Status()
	if len(status.Rules) != 1 || !status.Rules[0].Running {
		test.Fatalf("rule not running: %+v", status)
	}
	return app, status.Rules[0].Address
}

// ttListenerAssociate opens a UDP association through the SOCKS5 listener and returns it with its control
// connection's local address, the association's client.
func ttListenerAssociate(test *testing.T, listener string) (net.PacketConn, string) {
	test.Helper()
	client, err := socks5.NewDialer("socks5://" + listener)
	if err != nil {
		test.Fatal(err)
	}
	var control string
	client.ProxyDial = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err == nil {
			control = conn.LocalAddr().String()
		}
		return conn, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), ttWait)
	defer cancel()
	conn, err := client.DialContext(ctx, "udp", ttUDPEcho(test).String())
	if err != nil {
		test.Fatalf("UDP dial through the rule's listener: %v", err)
	}
	test.Cleanup(func() { conn.Close() })
	packet, ok := conn.(net.PacketConn)
	if !ok {
		test.Fatalf("socks5 UDP conn %T is not a net.PacketConn", conn)
	}
	return packet, control
}

// ttAssertListenerEchoes writes from an association to two echo servers, wants each echo back from its address, and
// returns them in order.
func ttAssertListenerEchoes(test *testing.T, conn net.PacketConn) []*net.UDPAddr {
	test.Helper()
	targets := []*net.UDPAddr{ttUDPEcho(test), ttUDPEcho(test)}
	want := map[string]string{}
	for index, payload := range []string{"first", "second"} {
		want[targets[index].String()] = payload
		if _, err := conn.WriteTo([]byte(payload), targets[index]); err != nil {
			test.Fatalf("write to %s: %v", targets[index], err)
		}
	}
	// The wait ends by closing conn, whose reads may ignore deadlines.
	guard := time.AfterFunc(ttWait, func() { conn.Close() })
	defer guard.Stop()
	got := map[string]string{}
	buf := make([]byte, 64)
	for range len(want) {
		n, source, err := conn.ReadFrom(buf)
		if err != nil {
			test.Fatalf("read echoes %v: %v, want %v", got, err, want)
		}
		got[source.String()] = string(buf[:n])
	}
	if !reflect.DeepEqual(got, want) {
		test.Fatalf("echoes by source %v, want %v", got, want)
	}
	return targets
}

// ttAssertListenerUDPConnection fails unless the rule has one live connection, from client to one of targets.
func ttAssertListenerUDPConnection(test *testing.T, app *App, client string, targets []*net.UDPAddr) {
	test.Helper()
	connections := app.metrics.Snapshot().Rules[0].Connections
	if len(connections) != 1 || connections[0].Client != client || !slices.Contains(ttAddrStrings(targets), connections[0].Target) {
		test.Fatalf("connections = %+v, want one from %s to one of %v", connections, client, targets)
	}
}

func ttAddrStrings(addrs []*net.UDPAddr) []string {
	strings := make([]string, len(addrs))
	for index, addr := range addrs {
		strings[index] = addr.String()
	}
	slices.Sort(strings)
	return strings
}

// ttWaySOCKS5 records the target socket of each UDP association a SOCKS5 proxy serves.
type ttWaySOCKS5 struct {
	mu           sync.Mutex
	associations []*ttWayAssociation
}

func (way *ttWaySOCKS5) recorded() []*ttWayAssociation {
	way.mu.Lock()
	defer way.mu.Unlock()
	return slices.Clone(way.associations)
}

// ttWayAssociation is an association's target socket recording its destinations; closed closes on its first Close.
type ttWayAssociation struct {
	net.PacketConn
	mu           sync.Mutex
	destinations []string
	closed       chan struct{}
	once         sync.Once
}

func (association *ttWayAssociation) WriteTo(p []byte, addr net.Addr) (int, error) {
	association.mu.Lock()
	if !slices.Contains(association.destinations, addr.String()) {
		association.destinations = append(association.destinations, addr.String())
	}
	association.mu.Unlock()
	return association.PacketConn.WriteTo(p, addr)
}

func (association *ttWayAssociation) Close() error {
	association.once.Do(func() { close(association.closed) })
	return association.PacketConn.Close()
}

func (association *ttWayAssociation) sent() []string {
	association.mu.Lock()
	defer association.mu.Unlock()
	return slices.Clone(association.destinations)
}

// ttStartWaySOCKS5 serves SOCKS5 with UDP ASSOCIATE on IPv4 loopback until the test ends and returns its address.
func ttStartWaySOCKS5(test *testing.T) (string, *ttWaySOCKS5) {
	test.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { listener.Close() })
	way := &ttWaySOCKS5{}
	server := socks5.NewServer()
	server.ProxyOutgoingListenPacket = func(ctx context.Context, network, address string) (net.PacketConn, error) {
		conn, err := (&net.ListenConfig{}).ListenPacket(ctx, network, address)
		if err != nil {
			return nil, err
		}
		association := &ttWayAssociation{PacketConn: conn, closed: make(chan struct{})}
		way.mu.Lock()
		way.associations = append(way.associations, association)
		way.mu.Unlock()
		return association, nil
	}
	go server.Serve(listener)
	return listener.Addr().String(), way
}

// ttUDPPortEcho serves UDP on IPv4 loopback until the test ends, answering each datagram with its sender's port.
func ttUDPPortEcho(test *testing.T) *net.UDPAddr {
	test.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { conn.Close() })
	go func() {
		buf := make([]byte, 64)
		for {
			_, source, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			conn.WriteToUDP([]byte(strconv.Itoa(source.Port)), source)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr)
}

// ttListenerSenderPort sends a datagram from an association to a ttUDPPortEcho and returns the sender port it saw.
func ttListenerSenderPort(test *testing.T, conn net.PacketConn, echo *net.UDPAddr) string {
	test.Helper()
	if _, err := conn.WriteTo([]byte("x"), echo); err != nil {
		test.Fatalf("write to %s: %v", echo, err)
	}
	// The wait ends by closing conn, whose reads may ignore deadlines.
	guard := time.AfterFunc(ttWait, func() { conn.Close() })
	defer guard.Stop()
	buf := make([]byte, 64)
	n, source, err := conn.ReadFrom(buf)
	if err != nil || source.String() != echo.String() {
		test.Fatalf("read %q from %v, %v; want the sender port from %s", buf[:n], source, err, echo)
	}
	return string(buf[:n])
}
