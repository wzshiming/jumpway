package tray

import (
	"cmp"
	"context"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/hostmatcher"
	"github.com/wzshiming/httpproxy"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/metrics"
	"github.com/wzshiming/jumpway/tunproxy"
	"github.com/wzshiming/socks5"
	"github.com/wzshiming/tun"
	"github.com/wzshiming/tun/stack"
)

func TestUDPListenConfigPassesClientAddr(test *testing.T) {
	client := &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000}
	var got ttDial
	listen := udpListenConfig{ListenConfig: ttListenPacket(func(ctx context.Context, network, address string) (net.PacketConn, error) {
		got = ttDial{network: network, address: address, client: jumpway.ClientAddr(ctx)}
		return nil, errors.New("refused")
	})}
	listen.ListenPacket(stack.WithClientAddr(context.Background(), client), "udp4", ":0")
	if want := (ttDial{network: "udp4", address: ":0", client: client.String()}); got != want {
		test.Fatalf("rule listen saw %+v, want %+v", got, want)
	}
}

type ttDial struct{ network, address, client string }

func TestUDPListenConfigKeepsClientAddrWithoutTUNClient(test *testing.T) {
	var got string
	listen := udpListenConfig{ListenConfig: ttListenPacket(func(ctx context.Context, network, address string) (net.PacketConn, error) {
		got = jumpway.ClientAddr(ctx)
		return nil, errors.New("refused")
	})}
	client := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 50000}
	listen.ListenPacket(jumpway.WithClientAddr(context.Background(), client), "udp4", ":0")
	if got != client.String() {
		test.Fatalf("rule listen saw client %q, want the consumer's %q", got, client)
	}
}

func TestUDPListenConfigRefusesStreamListen(test *testing.T) {
	for name, listen := range map[string]stack.ListenConfig{
		"full cone":   udpListenConfig{ListenConfig: tunproxy.Direct(nil)},
		"unsupported": udpUnsupported("alpha"),
	} {
		if _, err := listen.Listen(context.Background(), "tcp", ":0"); !errors.Is(err, errors.ErrUnsupported) {
			test.Errorf("%s: Listen error = %v, want errors.ErrUnsupported", name, err)
		}
	}
}

func TestUDPListenConfigDirectRuleUsesDirect(test *testing.T) {
	inner := ttNewEchoConn()
	calls := ttSeamTUNUDP(test, ttListenPacket(func(context.Context, string, string) (net.PacketConn, error) { return inner, nil }))
	capture := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	listen := ttUDPListenConfig(test, config.Rule{Name: "direct"}, exit{base: local.LOCAL, capture: capture}, nil)
	if len(calls.direct) != 1 || !slices.Equal(calls.direct[0], capture) {
		test.Fatalf("tunproxy.Direct calls %v, want one with %v", calls.direct, capture)
	}
	if conn, err := listen.ListenPacket(context.Background(), "udp4", ":0"); err != nil || conn != inner {
		test.Fatalf("ListenPacket = %v, %v; want the socket of tunproxy.Direct's listen config", conn, err)
	}
}

func TestUDPListenConfigAccountsDirectSession(test *testing.T) {
	ttSeamTUNUDP(test, ttListenPacket(func(context.Context, string, string) (net.PacketConn, error) { return ttNewEchoConn(), nil }))
	envShuntSetenv(test, "", "")
	registry := metrics.NewRegistry()
	registry.Sync([]config.Rule{{Name: "direct"}}, "", nil)
	listen := newUDPListenConfig(config.Rule{Name: "direct"}, exit{base: local.LOCAL}, nil, registry.Rule("direct").WrapPacketConn)
	client := &net.UDPAddr{IP: net.IPv4(198, 18, 0, 2), Port: 50000}
	conn, err := listen.ListenPacket(stack.WithClientAddr(context.Background(), client), "udp4", ":0")
	if err != nil {
		test.Fatal(err)
	}
	defer conn.Close()
	ttExchange(test, conn)
	connections := registry.Snapshot().Rules[0].Connections
	if len(connections) != 1 || connections[0].Client != client.String() || connections[0].Target != "192.0.2.1:53" || connections[0].Stats.Up != connections[0].Stats.Down || connections[0].Stats.Up != int64(len("first")+len("second")) {
		test.Fatalf("connections = %+v, want one from %s to the first peer with the echoed bytes of both", connections, client)
	}
}

func TestUDPListenConfigUnsupportedExits(test *testing.T) {
	for _, scenario := range []struct {
		name    string
		forward config.Forward
	}{
		{name: "fixed target", forward: config.Forward{Host: "127.0.0.1", Port: 1}},
		{name: "virtual forward", forward: config.Forward{Virtual: "channel"}},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			calls := ttSeamTUNUDP(test, nil)
			listen := ttUDPListenConfig(test, config.Rule{Name: "unsupported", Forward: scenario.forward}, exit{base: local.LOCAL}, nil)
			if len(calls.direct) != 0 {
				test.Fatalf("tunproxy.Direct calls %v, want none", calls.direct)
			}
			conn, err := listen.ListenPacket(context.Background(), "udp4", ":0")
			if err == nil {
				conn.Close()
			}
			if !errors.Is(err, errors.ErrUnsupported) {
				test.Fatalf("ListenPacket error = %v, want errors.ErrUnsupported", err)
			}
		})
	}
}

func TestUDPListenConfigProxiesThroughRuleDialer(test *testing.T) {
	for _, scenario := range []struct {
		name string
		way  []bridgeconfig.Node
	}{
		{name: "socks5 exit", way: []bridgeconfig.Node{{LB: []string{"socks5://127.0.0.1:1"}}}},
		{name: "http exit", way: []bridgeconfig.Node{{LB: []string{"http://127.0.0.1:1"}}}},
		{name: "ssh exit", way: []bridgeconfig.Node{{LB: []string{"ssh://user@127.0.0.1:1"}}}},
		{name: "http exit behind socks5 hop", way: []bridgeconfig.Node{{LB: []string{"http://127.0.0.1:1"}}, {LB: []string{"socks5://127.0.0.1:2"}}}},
		{name: "load-balanced exit", way: []bridgeconfig.Node{{LB: []string{"socks5://127.0.0.1:1", "http://127.0.0.1:2"}}}},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			calls := ttSeamTUNUDP(test, nil)
			dialer := &ttControlDialer{refuse: errors.New("tt refused")}
			rule := config.Rule{Name: "proxied", Forward: config.Forward{Way: scenario.way}}
			listen := ttUDPListenConfig(test, rule, exit{base: local.LOCAL}, dialer)
			client := &net.UDPAddr{IP: net.IPv4(198, 18, 0, 2), Port: 50000}
			conn := ttSession(test, listen, client)
			if _, err := conn.WriteTo([]byte("x"), &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 53}); !errors.Is(err, dialer.refuse) {
				test.Fatalf("write = %v, want the rule dialer's %v", err, dialer.refuse)
			}
			if want := []ttDial{{network: "udp", address: "192.0.2.1:53", client: client.String()}}; !slices.Equal(dialer.recorded(), want) {
				test.Fatalf("rule dials %+v, want %+v", dialer.recorded(), want)
			}
			if len(calls.direct) != 0 {
				test.Fatalf("tunproxy.Direct calls %v, want none", calls.direct)
			}
		})
	}
}

func TestUDPListenConfigShuntingRuleStaysSymmetric(test *testing.T) {
	socksWay := []bridgeconfig.Node{{LB: []string{"socks5://127.0.0.1:1"}}}
	for _, scenario := range []struct {
		name           string
		way            []bridgeconfig.Node
		envNo, envOnly string
		noProxy        []string
		symmetric      bool
	}{
		{name: "NO_PROXY list", way: socksWay, envNo: "192.0.2.1", symmetric: true},
		{name: "ONLY_PROXY list", way: socksWay, envOnly: "192.0.2.1", symmetric: true},
		{name: "no_proxy wildcard", way: socksWay, noProxy: []string{"*"}, symmetric: true},
		{name: "no_proxy IP with port", way: socksWay, noProxy: []string{"192.0.2.1:53"}, symmetric: true},
		{name: "no_proxy IPs, CIDRs and domains", way: socksWay, noProxy: []string{"10.0.0.0/8", "192.0.2.1", "example.com", "*.example.net", "example.org:53", "192.0.2.2/path"}},
		{name: "direct rule with NO_PROXY list", envNo: "192.0.2.1"},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			calls := ttSeamTUNUDP(test, ttListenPacket(func(context.Context, string, string) (net.PacketConn, error) { return ttNewEchoConn(), nil }))
			envShuntSetenv(test, scenario.envNo, scenario.envOnly)
			rule := config.Rule{Name: "shunt", Forward: config.Forward{Way: scenario.way}}
			policy := exit{noProxy: scenario.noProxy, base: local.LOCAL, capture: tun.CaptureRoutes(tunConfig(scenario.noProxy))}
			dialer := &ttEchoDialer{}
			dials := ttSessionDials(test, newUDPListenConfig(rule, policy, dialer, nil), dialer)
			if got := len(dials) == 2; got != scenario.symmetric {
				test.Fatalf("UDP symmetric = %v with dials %v, want %v", got, dials, scenario.symmetric)
			}
			if !scenario.symmetric {
				ttAssertNoProxySkipsCaptured(test, scenario.noProxy, policy.capture)
			}
			if direct := len(scenario.way) == 0; direct != (len(calls.direct) == 1 && len(dials) == 0) {
				test.Fatalf("tunproxy.Direct calls %v with dials %v, want the direct rule alone on tunproxy.Direct", calls.direct, dials)
			}
		})
	}
}

// ttAssertNoProxySkipsCaptured fails when the no_proxy matcher sends an address the TUN captures direct.
func ttAssertNoProxySkipsCaptured(test *testing.T, noProxy []string, capture []netip.Prefix) {
	test.Helper()
	matcher := hostmatcher.NewMatcher(noProxy)
	for _, address := range []string{"10.1.2.3:53", "192.0.2.1:53", "192.0.2.2:53", "198.51.100.1:53", "[2001:db8::1]:53"} {
		ip := netip.MustParseAddrPort(address).Addr()
		captured := slices.ContainsFunc(capture, func(prefix netip.Prefix) bool { return prefix.Contains(ip) })
		if captured && matcher.Match(address) {
			test.Fatalf("no_proxy %q matches captured %s", noProxy, address)
		}
	}
}

// ttUDPListenConfig builds rule's UDP listen config over dialer without proxy lists and fails unless there is one.
func ttUDPListenConfig(test *testing.T, rule config.Rule, policy exit, dialer bridge.Dialer) stack.ListenConfig {
	test.Helper()
	envShuntSetenv(test, "", "")
	listen := newUDPListenConfig(rule, policy, dialer, nil)
	if listen == nil {
		test.Fatal("no TUN listen config, want one")
	}
	return listen
}

func TestUDPListenConfigSOCKS5WayIsOneAssociation(test *testing.T) {
	for _, scheme := range []string{"socks5", "socks5h"} {
		test.Run(scheme, func(test *testing.T) {
			proxy := ttSOCKS5Server(test, "user", "secret")
			base := &ttControlDialer{}
			rule := config.Rule{Name: "socks", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{scheme + "://user:secret@" + proxy}}}}}
			listen, registry := ttRuleUDP(test, rule, base)
			client := &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000}
			echoes := ttAssertFullCone(test, ttSession(test, listen, client))
			if want := []ttDial{{network: "tcp", address: proxy, client: client.String()}}; !slices.Equal(base.recorded(), want) {
				test.Fatalf("base dials %+v, want one control connection %+v", base.recorded(), want)
			}
			if want := []ttDial{{network: "udp4", address: ":0", client: client.String()}}; !slices.Equal(base.listened(), want) {
				test.Fatalf("base packet listens %+v, want one relay socket %+v", base.listened(), want)
			}
			payload := int64(len("first") + len("second"))
			snapshot := registry.Snapshot().Rules[0]
			if len(snapshot.Targets) != 1 || snapshot.Targets[0].Address != echoes[0].String() || snapshot.Targets[0].Stats.Up != payload || snapshot.Targets[0].Stats.Down != payload {
				test.Fatalf("targets = %+v, want only %s with %d bytes each way", snapshot.Targets, echoes[0], payload)
			}
			if len(snapshot.Connections) != 1 || snapshot.Connections[0].Client != client.String() || snapshot.Connections[0].Target != echoes[0].String() {
				test.Fatalf("connections = %+v, want one from %s to %s", snapshot.Connections, client, echoes[0])
			}
			if stats := snapshot.Forward[0].URLs[0].Stats; stats.Dials != 1 || stats.DialFailures != 0 || stats.Up != payload || stats.Down != payload {
				test.Fatalf("forward hop 0 = %+v, want one dial with %d bytes each way", stats, payload)
			}
		})
	}
}

func TestUDPListenConfigSOCKS5WayBehindHTTPHopAssociates(test *testing.T) {
	proxy := ttSOCKS5Server(test, "", "")
	hop, err := httpproxy.NewSimpleServer("http://127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}
	if err := hop.Start(context.Background()); err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { hop.Close() })
	base := &ttControlDialer{}
	rule := config.Rule{Name: "socks", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"socks5://" + proxy}}, {LB: []string{hop.ProxyURL()}}}}}
	listen, _ := ttRuleUDP(test, rule, base)
	client := &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000}
	ttAssertFullCone(test, ttSession(test, listen, client))
	if want := []ttDial{{network: "tcp", address: hop.Address, client: client.String()}}; !slices.Equal(base.recorded(), want) {
		test.Fatalf("base dials %+v, want only the hop %+v", base.recorded(), want)
	}
	if want := []ttDial{{network: "udp4", address: ":0", client: client.String()}}; !slices.Equal(base.listened(), want) {
		test.Fatalf("base packet listens %+v, want one relay socket %+v", base.listened(), want)
	}
}

func TestUDPListenConfigHTTPExitDropsDatagrams(test *testing.T) {
	// The proxy host's UDP port is bound and silent, so the dial gets neither a reply nor a refusal on any OS.
	proxy, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { proxy.Close() })
	target, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { target.Close() })
	rule := config.Rule{Name: "http", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"http://" + proxy.LocalAddr().String()}}}}}
	listen, _ := ttRuleUDP(test, rule, &ttControlDialer{})
	conn := ttSession(test, listen, &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000})
	written := make(chan error, 1)
	go func() {
		_, err := conn.WriteTo([]byte("x"), target.LocalAddr())
		written <- err
	}()
	if err := proxy.SetReadDeadline(time.Now().Add(ttWait)); err != nil {
		test.Fatal(err)
	}
	if _, _, err := proxy.ReadFrom(make([]byte, 1024)); err != nil {
		test.Fatalf("the proxy host got no datagram of the dial within %v: %v", ttWait, err)
	}
	select {
	case err := <-written:
		test.Fatalf("write = %v before the session closed, want it pending on the silent proxy", err)
	default:
	}
	closed := make(chan error, 1)
	go func() { closed <- conn.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			test.Fatalf("Close = %v, want nil", err)
		}
	case <-time.After(ttWait):
		test.Fatalf("Close still blocked %v while the dial was in flight", ttWait)
	}
	select {
	case err := <-written:
		if err == nil {
			test.Fatal("write through an http exit succeeded, want an error")
		}
	case <-time.After(ttWait):
		test.Fatalf("write still pending %v after the session closed", ttWait)
	}
	// A sentinel sent after the session closed is queued behind anything that reached the target.
	sentinel, err := net.Dial("udp4", target.LocalAddr().String())
	if err != nil {
		test.Fatal(err)
	}
	defer sentinel.Close()
	if _, err := sentinel.Write([]byte("sentinel")); err != nil {
		test.Fatal(err)
	}
	if err := target.SetReadDeadline(time.Now().Add(ttWait)); err != nil {
		test.Fatal(err)
	}
	buf := make([]byte, 64)
	if n, source, err := target.ReadFrom(buf); err != nil || source.String() != sentinel.LocalAddr().String() {
		test.Fatalf("target first read %q from %v, %v; want only the sentinel from %v", buf[:n], source, err, sentinel.LocalAddr())
	}
}

func TestUDPListenConfigClosedSOCKS5SessionEndsControlConnection(test *testing.T) {
	proxy, ended := ttObservedSOCKS5Server(test)
	rule := config.Rule{Name: "socks", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"socks5://" + proxy}}}}}
	listen, _ := ttRuleUDP(test, rule, &ttControlDialer{})
	conn := ttSession(test, listen, &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000})
	ttAssertFullCone(test, conn)
	select {
	case <-ended:
		test.Fatal("the proxy's control connection ended while the session was open")
	default:
	}
	if err := conn.Close(); err != nil {
		test.Fatalf("Close = %v, want nil", err)
	}
	select {
	case <-ended:
	case <-time.After(ttWait):
		test.Fatalf("the proxy's control connection still open %v after the session closed", ttWait)
	}
}

func TestUDPListenConfigSOCKS5SessionSkipsForeignDatagram(test *testing.T) {
	proxy := ttSOCKS5Server(test, "", "")
	base := &ttControlDialer{}
	rule := config.Rule{Name: "socks", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"socks5://" + proxy}}}}}
	listen, _ := ttRuleUDP(test, rule, base)
	conn := ttSession(test, listen, &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000})
	echoes := ttAssertFullCone(test, conn)
	if listens := base.listened(); len(listens) != 1 {
		test.Fatalf("base packet listens %+v, want one relay socket", listens)
	}
	foreign, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}
	defer foreign.Close()
	relay := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: base.socket(0).LocalAddr().(*net.UDPAddr).Port}
	if _, err := foreign.WriteTo([]byte("foreign"), relay); err != nil {
		test.Fatal(err)
	}
	if _, err := conn.WriteTo([]byte("third"), echoes[0]); err != nil {
		test.Fatalf("write after a foreign datagram: %v", err)
	}
	guard := time.AfterFunc(ttWait, func() { conn.Close() })
	defer guard.Stop()
	buf := make([]byte, 64)
	n, source, err := conn.ReadFrom(buf)
	if err != nil || string(buf[:n]) != "third" || source.String() != echoes[0].String() {
		test.Fatalf("read after a foreign datagram = %q from %v, %v; want %q from %s", buf[:n], source, err, "third", echoes[0])
	}
}

func TestUDPListenConfigSOCKS5RelaySocketComesFromBypassBase(test *testing.T) {
	ttForceTUNSupported(test)
	recorder := &ttBypassRecorder{}
	previousDialer, previousDirect := newBypassDialer, newDirectUDP
	test.Cleanup(func() { newBypassDialer, newDirectUDP = previousDialer, previousDirect })
	newBypassDialer = recorder.newDialer
	var mu sync.Mutex
	var networks []string
	newDirectUDP = func(capture []netip.Prefix) stack.ListenConfig {
		direct := previousDirect(capture)
		return ttListenPacket(func(ctx context.Context, network, address string) (net.PacketConn, error) {
			mu.Lock()
			networks = append(networks, network)
			mu.Unlock()
			return direct.ListenPacket(ctx, network, address)
		})
	}
	envShuntSetenv(test, "", "")
	proxy := ttSOCKS5Server(test, "", "")
	rule := config.Rule{Name: "socks", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"socks5://" + proxy}}}}}
	registry := metrics.NewRegistry()
	registry.Sync([]config.Rule{rule}, "", nil)
	rs := registry.Rule(rule.Name)
	capture := tun.CaptureRoutes(tunConfig(nil))
	policy := exit{base: bypassBase(capture), capture: capture}
	dialer, err := (&App{}).buildDialer(context.Background(), rule, rs, policy)
	if err != nil {
		test.Fatal(err)
	}
	conn := ttSession(test, newUDPListenConfig(rule, policy, dialer, rs.WrapPacketConn), &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000})
	ttAssertFullCone(test, conn)
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"udp4"}; !slices.Equal(networks, want) {
		test.Fatalf("base packet listens %v, want one relay socket %v", networks, want)
	}
	if _, dialed := recorder.recorded(); !slices.Equal(dialed, []string{proxy}) {
		test.Fatalf("bypass dials %v, want only the control connection to %s", dialed, proxy)
	}
}

// ttObservedSOCKS5Server serves SOCKS5 on IPv4 loopback until the test ends; ended closes once a client connection ends.
func ttObservedSOCKS5Server(test *testing.T) (string, <-chan struct{}) {
	test.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { listener.Close() })
	observed := &ttEndListener{Listener: listener, ended: make(chan struct{})}
	go socks5.NewServer().Serve(observed)
	return listener.Addr().String(), observed.ended
}

// ttEndListener closes ended when a Read of any accepted connection fails.
type ttEndListener struct {
	net.Listener
	ended chan struct{}
	once  sync.Once
}

func (listener *ttEndListener) Accept() (net.Conn, error) {
	conn, err := listener.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &ttEndConn{Conn: conn, listener: listener}, nil
}

type ttEndConn struct {
	net.Conn
	listener *ttEndListener
}

func (conn *ttEndConn) Read(p []byte) (int, error) {
	n, err := conn.Conn.Read(p)
	if err != nil {
		conn.listener.once.Do(func() { close(conn.listener.ended) })
	}
	return n, err
}

// ttRuleUDP builds rule's dialer over base and its TUN listen config as startRule does, with the rule's metrics.
func ttRuleUDP(test *testing.T, rule config.Rule, base *ttControlDialer) (stack.ListenConfig, *metrics.Registry) {
	test.Helper()
	envShuntSetenv(test, "", "")
	registry := metrics.NewRegistry()
	registry.Sync([]config.Rule{rule}, "", nil)
	rs := registry.Rule(rule.Name)
	policy := exit{base: ttLocal(base)}
	dialer, err := (&App{}).buildDialer(context.Background(), rule, rs, policy)
	if err != nil {
		test.Fatal(err)
	}
	listen := newUDPListenConfig(rule, policy, dialer, rs.WrapPacketConn)
	if listen == nil {
		test.Fatal("no TUN listen config, want one")
	}
	return listen, registry
}

// ttSession opens a UDP session's socket of listen for client, closed when the test ends.
func ttSession(test *testing.T, listen stack.ListenConfig, client net.Addr) net.PacketConn {
	test.Helper()
	conn, err := listen.ListenPacket(stack.WithClientAddr(context.Background(), client), "udp4", ":0")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { conn.Close() })
	return conn
}

// ttLocal is a copy of local.LOCAL dialing and listening through dialer.
func ttLocal(dialer *ttControlDialer) *local.Local {
	base := *local.LOCAL
	base.Dialer = dialer
	base.ListenConfig = dialer
	return &base
}

// ttControlDialer dials and listens for real, or fails dials with refuse unless nil, and records each dial and packet
// listen with its jumpway client address.
type ttControlDialer struct {
	refuse  error
	mu      sync.Mutex
	dials   []ttDial
	listens []ttDial
	sockets []net.PacketConn
}

func (dialer *ttControlDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer.mu.Lock()
	dialer.dials = append(dialer.dials, ttDial{network: network, address: address, client: jumpway.ClientAddr(ctx)})
	dialer.mu.Unlock()
	if dialer.refuse != nil {
		return nil, dialer.refuse
	}
	return (&net.Dialer{}).DialContext(ctx, network, address)
}

func (dialer *ttControlDialer) recorded() []ttDial {
	dialer.mu.Lock()
	defer dialer.mu.Unlock()
	return slices.Clone(dialer.dials)
}

func (dialer *ttControlDialer) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return (&net.ListenConfig{}).Listen(ctx, network, address)
}

func (dialer *ttControlDialer) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	conn, err := (&net.ListenConfig{}).ListenPacket(ctx, network, address)
	if err != nil {
		return nil, err
	}
	dialer.mu.Lock()
	dialer.listens = append(dialer.listens, ttDial{network: network, address: address, client: jumpway.ClientAddr(ctx)})
	dialer.sockets = append(dialer.sockets, conn)
	dialer.mu.Unlock()
	return conn, nil
}

func (dialer *ttControlDialer) listened() []ttDial {
	dialer.mu.Lock()
	defer dialer.mu.Unlock()
	return slices.Clone(dialer.listens)
}

// socket is the packet socket the base opened at index.
func (dialer *ttControlDialer) socket(index int) net.PacketConn {
	dialer.mu.Lock()
	defer dialer.mu.Unlock()
	return dialer.sockets[index]
}

// ttSOCKS5Server serves SOCKS5 with UDP ASSOCIATE on IPv4 loopback until the test ends and returns its address.
func ttSOCKS5Server(test *testing.T, user, password string) string {
	test.Helper()
	proxy := "socks5://127.0.0.1:0"
	if user != "" {
		proxy = "socks5://" + user + ":" + password + "@127.0.0.1:0"
	}
	server, err := socks5.NewSimpleServer(proxy)
	if err != nil {
		test.Fatal(err)
	}
	if err := server.Start(context.Background()); err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { server.Close() })
	return server.Address
}

// ttAssertFullCone writes from conn to two echo servers, wants each echo back from its source, and returns them in order.
func ttAssertFullCone(test *testing.T, conn net.PacketConn) []*net.UDPAddr {
	test.Helper()
	var targets []*net.UDPAddr
	want := map[string]string{}
	for _, payload := range []string{"first", "second"} {
		echo := ttUDPEcho(test)
		targets = append(targets, echo)
		want[echo.String()] = payload
		if _, err := conn.WriteTo([]byte(payload), echo); err != nil {
			test.Fatalf("write to %s: %v", echo, err)
		}
	}
	type echo struct {
		source  net.Addr
		payload string
		err     error
	}
	// A proxy's packet conn may ignore read deadlines, so the wait ends by closing conn.
	echoes := make(chan echo, len(want))
	go func() {
		buf := make([]byte, 64)
		for range len(want) {
			n, source, err := conn.ReadFrom(buf)
			echoes <- echo{source: source, payload: string(buf[:n]), err: err}
			if err != nil {
				return
			}
		}
	}()
	got := map[string]string{}
	timeout := time.After(ttWait)
	for range len(want) {
		select {
		case echo := <-echoes:
			if echo.err != nil {
				test.Fatalf("read echoes %v: %v, want %v", got, echo.err, want)
			}
			udp, ok := echo.source.(*net.UDPAddr)
			if !ok {
				test.Fatalf("echo source %v is a %T, want a *net.UDPAddr", echo.source, echo.source)
			}
			got[udp.String()] = echo.payload
		case <-timeout:
			conn.Close()
			test.Fatalf("read echoes %v: none more within %v, want %v", got, ttWait, want)
		}
	}
	if !reflect.DeepEqual(got, want) {
		test.Fatalf("echoes by source %v, want %v", got, want)
	}
	return targets
}

// ttUDPEcho serves a UDP echo on IPv4 loopback until the test ends and returns its address.
func ttUDPEcho(test *testing.T) *net.UDPAddr {
	test.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { conn.Close() })
	go func() {
		buf := make([]byte, 64)
		for {
			n, source, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			conn.WriteTo(buf[:n], source)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr)
}

// ttUDPCalls records the calls of newDirectUDP.
type ttUDPCalls struct {
	direct [][]netip.Prefix
}

// ttSeamTUNUDP swaps newDirectUDP for a fake recording its calls and returning listen.
func ttSeamTUNUDP(test *testing.T, listen stack.ListenConfig) *ttUDPCalls {
	calls := &ttUDPCalls{}
	previous := newDirectUDP
	test.Cleanup(func() { newDirectUDP = previous })
	newDirectUDP = func(capture []netip.Prefix) stack.ListenConfig {
		calls.direct = append(calls.direct, capture)
		return listen
	}
	return calls
}

// ttListenPacket is a UDP listen config opening each session's socket with its func.
type ttListenPacket func(ctx context.Context, network, address string) (net.PacketConn, error)

func (listen ttListenPacket) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return nil, errors.ErrUnsupported
}

func (listen ttListenPacket) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	return listen(ctx, network, address)
}

// ttEchoConn is an in-memory UDP socket on which each datagram comes back from the address it was sent to.
type ttEchoConn struct {
	net.PacketConn
	replies chan ttDatagram
	closed  chan struct{}
	once    sync.Once
}

type ttDatagram struct {
	payload []byte
	source  net.Addr
}

func ttNewEchoConn() *ttEchoConn {
	return &ttEchoConn{replies: make(chan ttDatagram, 8), closed: make(chan struct{})}
}

func (conn *ttEchoConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	conn.replies <- ttDatagram{payload: slices.Clone(p), source: addr}
	return len(p), nil
}

func (conn *ttEchoConn) ReadFrom(p []byte) (int, net.Addr, error) {
	select {
	case datagram := <-conn.replies:
		return copy(p, datagram.payload), datagram.source, nil
	case <-conn.closed:
		return 0, nil, net.ErrClosed
	}
}

func (conn *ttEchoConn) Close() error {
	conn.once.Do(func() { close(conn.closed) })
	return nil
}

func (conn *ttEchoConn) LocalAddr() net.Addr { return &net.UDPAddr{} }

// ttExchange sends "first" and "second" from conn to two peers and reads both echoes.
func ttExchange(test *testing.T, conn net.PacketConn) {
	test.Helper()
	for index, payload := range []string{"first", "second"} {
		if _, err := conn.WriteTo([]byte(payload), &net.UDPAddr{IP: net.IPv4(192, 0, 2, byte(index+1)), Port: 53}); err != nil {
			test.Fatal(err)
		}
	}
	buf := make([]byte, 64)
	for range 2 {
		if _, _, err := conn.ReadFrom(buf); err != nil {
			test.Fatal(err)
		}
	}
}

// ttRuleShapes are a rule's forward shapes, each with a way to show that one goes unused.
var ttRuleShapes = []struct {
	name    string
	forward config.Forward
}{
	{name: "way", forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"socks5://127.0.0.1:1"}}}}},
	{name: "no way", forward: config.Forward{}},
	{name: "fixed target", forward: config.Forward{Host: "127.0.0.1", Port: 1, Way: []bridgeconfig.Node{{LB: []string{"socks5://127.0.0.1:1"}}}}},
	{name: "virtual target", forward: config.Forward{Virtual: "channel"}},
}

func TestUDPListenConfigProxyModeIsTheDefault(test *testing.T) {
	for _, shape := range ttRuleShapes {
		test.Run(shape.name, func(test *testing.T) {
			decide := func(mode string) (direct int, dials []string, err error) {
				calls := ttSeamTUNUDP(test, ttListenPacket(func(context.Context, string, string) (net.PacketConn, error) { return ttNewEchoConn(), nil }))
				forward := shape.forward
				forward.UDP.Mode = mode
				dialer := &ttEchoDialer{}
				conn, err := ttUDPListenConfig(test, config.Rule{Name: "rule", Forward: forward}, exit{base: local.LOCAL}, dialer).ListenPacket(context.Background(), "udp4", ":0")
				if err == nil {
					guard := time.AfterFunc(ttWait, func() { conn.Close() })
					ttExchange(test, conn)
					guard.Stop()
					conn.Close()
				}
				return len(calls.direct), dialer.dialed(), err
			}
			absentDirect, absentDials, absentErr := decide("")
			direct, dials, err := decide(config.UDPModeProxy)
			if direct != absentDirect || !slices.Equal(dials, absentDials) || (err == nil) != (absentErr == nil) {
				test.Fatalf("mode proxy: Direct calls %d, dials %v, error %v; absent: %d, %v, %v", direct, dials, err, absentDirect, absentDials, absentErr)
			}
		})
	}
}

func TestUDPListenConfigDirectModeLeavesFromThisMachine(test *testing.T) {
	for _, shape := range ttRuleShapes {
		test.Run(shape.name, func(test *testing.T) {
			calls := ttSeamTUNUDP(test, ttListenPacket(func(context.Context, string, string) (net.PacketConn, error) { return ttNewEchoConn(), nil }))
			envShuntSetenv(test, "", "")
			rule := config.Rule{Name: "rule", Forward: shape.forward}
			rule.Forward.UDP.Mode = config.UDPModeDirect
			registry := metrics.NewRegistry()
			registry.Sync([]config.Rule{rule}, "", nil)
			capture := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
			dialer := &ttEchoDialer{}
			listen := newUDPListenConfig(rule, exit{base: local.LOCAL, capture: capture}, dialer, registry.Rule(rule.Name).WrapPacketConn)
			client := &net.UDPAddr{IP: net.IPv4(198, 18, 0, 2), Port: 50000}
			conn := ttSession(test, listen, client)
			guard := time.AfterFunc(ttWait, func() { conn.Close() })
			defer guard.Stop()
			ttExchange(test, conn)
			if dials := dialer.dialed(); len(dials) != 0 {
				test.Fatalf("rule dials %v, want none", dials)
			}
			if len(calls.direct) != 1 || !slices.Equal(calls.direct[0], capture) {
				test.Fatalf("tunproxy.Direct calls %v, want one with %v", calls.direct, capture)
			}
			if connections := registry.Snapshot().Rules[0].Connections; len(connections) != 1 || connections[0].Client != client.String() {
				test.Fatalf("connections = %+v, want the session from %s", connections, client)
			}
		})
	}
}

func TestUDPListenConfigBlockModeOpensNothing(test *testing.T) {
	for _, shape := range ttRuleShapes {
		test.Run(shape.name, func(test *testing.T) {
			calls := ttSeamTUNUDP(test, ttListenPacket(func(context.Context, string, string) (net.PacketConn, error) { return ttNewEchoConn(), nil }))
			rule := config.Rule{Name: "rule", Forward: shape.forward}
			rule.Forward.UDP = config.UDP{Mode: config.UDPModeBlock, NAT: config.NATSymmetric}
			base := &ttControlDialer{}
			dialer := &ttEchoDialer{}
			conn, err := ttUDPListenConfig(test, rule, exit{base: ttLocal(base)}, dialer).ListenPacket(context.Background(), "udp4", ":0")
			if err == nil {
				conn.Close()
				test.Fatal("ListenPacket opened a session, want it refused")
			}
			if len(calls.direct) != 0 || len(dialer.dialed()) != 0 || len(base.recorded()) != 0 || len(base.listened()) != 0 {
				test.Fatalf("Direct calls %v, rule dials %v, base dials %v and listens %v; want none", calls.direct, dialer.dialed(), base.recorded(), base.listened())
			}
		})
	}
}

func TestUDPListenConfigNAT(test *testing.T) {
	for _, scenario := range []struct {
		name      string
		nat       string
		envNo     string
		symmetric bool
	}{
		{name: "full cone", nat: config.NATFullCone},
		{name: "full cone over a NO_PROXY list", nat: config.NATFullCone, envNo: "192.0.2.1"},
		{name: "symmetric", nat: config.NATSymmetric, symmetric: true},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			envShuntSetenv(test, scenario.envNo, "")
			rule := config.Rule{Name: "rule", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"socks5://127.0.0.1:1"}}}, UDP: config.UDP{NAT: scenario.nat}}}
			dialer := &ttEchoDialer{}
			dials := ttSessionDials(test, newUDPListenConfig(rule, exit{base: local.LOCAL}, dialer, nil), dialer)
			if got := len(dials) == 2; got != scenario.symmetric {
				test.Fatalf("UDP symmetric = %v with dials %v, want %v", got, dials, scenario.symmetric)
			}
		})
	}
}

func TestUDPListenConfigSymmetricSOCKS5WayIsOneAssociationPerDestination(test *testing.T) {
	proxy := ttSOCKS5Server(test, "", "")
	base := &ttControlDialer{}
	rule := config.Rule{Name: "socks", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"socks5://" + proxy}}}, UDP: config.UDP{NAT: config.NATSymmetric}}}
	listen, _ := ttRuleUDP(test, rule, base)
	client := &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000}
	ttAssertFullCone(test, ttSession(test, listen, client))
	control := ttDial{network: "tcp", address: proxy, client: client.String()}
	if want := []ttDial{control, control}; !slices.Equal(base.recorded(), want) {
		test.Fatalf("base dials %+v, want a control connection per destination %+v", base.recorded(), want)
	}
	if listens := base.listened(); len(listens) != 2 {
		test.Fatalf("base packet listens %+v, want a relay socket per destination", listens)
	}
}

func TestUDPListenConfigSymmetricDirectDialsEachDestinationFromBase(test *testing.T) {
	for _, shape := range ttRuleShapes[:2] {
		test.Run(shape.name, func(test *testing.T) {
			calls := ttSeamTUNUDP(test, nil)
			envShuntSetenv(test, "", "")
			rule := config.Rule{Name: "rule", Forward: shape.forward}
			rule.Forward.UDP = config.UDP{Mode: config.UDPModeDirect, NAT: config.NATSymmetric}
			if len(shape.forward.Way) == 0 {
				rule.Forward.UDP.Mode = ""
			}
			registry := metrics.NewRegistry()
			registry.Sync([]config.Rule{rule}, "", nil)
			base := &ttControlDialer{}
			dialer := &ttEchoDialer{}
			listen := newUDPListenConfig(rule, exit{base: ttLocal(base)}, dialer, registry.Rule(rule.Name).WrapPacketConn)
			client := &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000}
			echoes := ttAssertFullCone(test, ttSession(test, listen, client))
			// Each destination is dialed on its own goroutine, so the dials come in any order.
			byAddress := func(a, b ttDial) int { return cmp.Compare(a.address, b.address) }
			want := []ttDial{{network: "udp", address: echoes[0].String(), client: client.String()}, {network: "udp", address: echoes[1].String(), client: client.String()}}
			slices.SortFunc(want, byAddress)
			dials := base.recorded()
			slices.SortFunc(dials, byAddress)
			if !slices.Equal(dials, want) {
				test.Fatalf("base dials %+v, want one per destination %+v", dials, want)
			}
			if len(dialer.dialed()) != 0 || len(calls.direct) != 0 {
				test.Fatalf("rule dials %v and Direct calls %v, want none", dialer.dialed(), calls.direct)
			}
			if connections := registry.Snapshot().Rules[0].Connections; len(connections) != 1 || connections[0].Client != client.String() {
				test.Fatalf("connections = %+v, want the session from %s", connections, client)
			}
		})
	}
}

func TestUDPListenConfigIdleClosesIdleDestinationConn(test *testing.T) {
	rule := config.Rule{Name: "rule", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"socks5://127.0.0.1:1"}}}, UDP: config.UDP{NAT: config.NATSymmetric, Idle: 1}}}
	dialer := &ttEchoDialer{}
	conn := ttSession(test, ttUDPListenConfig(test, rule, exit{base: local.LOCAL}, dialer), &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000})
	guard := time.AfterFunc(ttWait+2*time.Second, func() { conn.Close() })
	defer guard.Stop()
	idled, other := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 53}, &net.UDPAddr{IP: net.IPv4(192, 0, 2, 2), Port: 53}
	exchange := func(to net.Addr) {
		if _, err := conn.WriteTo([]byte("x"), to); err != nil {
			test.Fatal(err)
		}
		if _, _, err := conn.ReadFrom(make([]byte, 8)); err != nil {
			test.Fatal(err)
		}
	}
	exchange(idled)
	first := dialer.conn(0)
	deadline := time.After(ttWait)
	for {
		select {
		case <-first.closed:
			return
		case <-time.After(100 * time.Millisecond):
			exchange(other)
		case <-deadline:
			test.Fatalf("destination conn still open %v after it idled past udp.idle of 1s", ttWait)
		}
	}
}

// ttSessionDials opens a session of listen, exchanges a datagram with each of two destinations, and returns dialer's dials.
func ttSessionDials(test *testing.T, listen stack.ListenConfig, dialer *ttEchoDialer) []string {
	test.Helper()
	conn := ttSession(test, listen, &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000})
	guard := time.AfterFunc(ttWait, func() { conn.Close() })
	defer guard.Stop()
	ttExchange(test, conn)
	return dialer.dialed()
}

// ttEchoDialer dials each address as a ttDialedEcho and records the addresses.
type ttEchoDialer struct {
	mu        sync.Mutex
	addresses []string
	conns     []ttDialedEcho
}

func (dialer *ttEchoDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	remote, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	conn := ttDialedEcho{ttEchoConn: ttNewEchoConn(), remote: remote}
	dialer.mu.Lock()
	defer dialer.mu.Unlock()
	dialer.addresses = append(dialer.addresses, address)
	dialer.conns = append(dialer.conns, conn)
	return conn, nil
}

func (dialer *ttEchoDialer) dialed() []string {
	dialer.mu.Lock()
	defer dialer.mu.Unlock()
	return slices.Clone(dialer.addresses)
}

// conn is the conn dialed at index.
func (dialer *ttEchoDialer) conn(index int) ttDialedEcho {
	dialer.mu.Lock()
	defer dialer.mu.Unlock()
	return dialer.conns[index]
}

// ttDialedEcho is a ttEchoConn dialed to remote: it can address any destination, and Write and Read are with remote.
type ttDialedEcho struct {
	*ttEchoConn
	remote net.Addr
}

func (conn ttDialedEcho) Write(p []byte) (int, error) { return conn.WriteTo(p, conn.remote) }

func (conn ttDialedEcho) Read(p []byte) (int, error) {
	n, _, err := conn.ReadFrom(p)
	return n, err
}

func (conn ttDialedEcho) RemoteAddr() net.Addr { return conn.remote }
