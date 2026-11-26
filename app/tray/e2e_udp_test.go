//go:build e2e && (darwin || linux || windows)

package tray

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/bridge/chain"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/jumpway/app/web/services/configs"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/socks5"
	"github.com/wzshiming/tun"
	"github.com/wzshiming/tun/bypass"
)

// The UDP test targets: public, so the TUN captures them, and off port 53, so they take the rule's UDP path.
var (
	ttE2EUDPTarget4      = &net.UDPAddr{IP: net.IPv4(1, 1, 1, 1).To4(), Port: 7777}
	ttE2EUDPTarget4Other = &net.UDPAddr{IP: net.IPv4(1, 0, 0, 1).To4(), Port: 7777}
	ttE2EUDPTarget6      = &net.UDPAddr{IP: net.ParseIP("2606:4700:4700::1111"), Port: 7777}
)

func TestE2ETrayTUNUDPThroughRule(t *testing.T) {
	ttE2EGate(t)
	previousProxy := setSystemProxy
	setSystemProxy = func(string) {}
	t.Cleanup(func() { setSystemProxy = previousProxy })
	previousNoProxy, previousOnlyProxy := chain.NoProxy, chain.OnlyProxy
	chain.NoProxy, chain.OnlyProxy = nil, nil
	t.Cleanup(func() { chain.NoProxy, chain.OnlyProxy = previousNoProxy, previousOnlyProxy })

	conf := &config.Config{Rules: []config.Rule{ttTUNRule("e2e", 0)}}
	// The empty default, so the TUN captures every public destination.
	noProxy := conf.NoProxy.GetList(t.TempDir())
	capture := tun.CaptureRoutes(tunConfig(noProxy))
	// The runner's own TCP is forwarded on, bound outside the TUN; its foreign UDP is dropped.
	proxy := ttStartE2ESOCKS5(t, bypass.NewDialer(capture), ttE2EUDPTarget4, ttE2EUDPTarget4Other, ttE2EUDPTarget6)
	conf.Rules[0].Forward.Way = []bridgeconfig.Node{{LB: []string{"socks5://" + proxy.addr}}}

	// Registered after the proxy, so the app and its TUN stop before the proxy closes.
	app := newTestApp(t, conf)
	opener := &ttE2EOpener{}
	app.openTUN = opener.open
	watchdog := time.AfterFunc(3*time.Minute, func() {
		app.stop()
		opener.closeAll()
	})
	t.Cleanup(func() { watchdog.Stop() })
	ttStart(t, app)
	// Runs before the app stops, so a failure shows the state the TUN left.
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("on failure, proxy saw %s", proxy.summary())
			ttE2ELogRoutes(t)
			ttE2ELogInterfaces(t)
		}
	})

	t.Run("U1 no UDP capture before selection", func(t *testing.T) {
		ttE2EUDPSendAndDrop("udp4", ttE2EUDPTarget4, 2*time.Second)
		ttE2EUDPSendAndDrop("udp6", ttE2EUDPTarget6, 2*time.Second)
		if n4, n6 := proxy.count(ttE2EUDPTarget4), proxy.count(ttE2EUDPTarget6); n4 != 0 || n6 != 0 {
			t.Fatalf("proxy datagrams to %s and %s before selecting a TUN = %d and %d, want 0; proxy saw %s", ttE2EUDPTarget4, ttE2EUDPTarget6, n4, n6, proxy.summary())
		}
	})

	app.selectTUN("e2e")
	status := ttE2EWaitStatus(t, app, opener, "the rule to run with a TUN", func(rule configs.RuleStatus) bool {
		return rule.Running && rule.Tun != ""
	})
	devices, errs := opener.recorded()
	if len(errs) != 0 || len(devices) != 1 {
		t.Fatalf("openTUN: %d devices, errors %v; want one device", len(devices), errs)
	}
	if dev := devices[0]; len(dev.Warnings) != 0 || dev.Interface != status.Tun {
		t.Fatalf("opened %q with warnings %q; want no warnings and status Tun %q", dev.Interface, dev.Warnings, status.Tun)
	}
	t.Logf("TUN %s serves rule e2e", status.Tun)
	ttE2ELogRoutes(t)

	t.Run("U2 udp4 through one association, full cone", func(t *testing.T) {
		conn := ttE2EListenUDP(t, "udp4")
		ttE2EUDPEchoViaProxy(t, proxy, conn, ttE2EUDPTarget4, "udp-v4")
		ttE2EUDPEchoViaProxy(t, proxy, conn, ttE2EUDPTarget4Other, "udp-v4-other")
		first, other := proxy.associationsOf(ttE2EUDPTarget4), proxy.associationsOf(ttE2EUDPTarget4Other)
		if len(first) != 1 || !slices.Equal(first, other) {
			t.Fatalf("associations carrying %s = %v and %s = %v, want the same single one; proxy saw %s", ttE2EUDPTarget4, first, ttE2EUDPTarget4Other, other, proxy.summary())
		}
		t.Logf("proxy saw %s", proxy.summary())
	})
	t.Run("U2 udp6 through an association", func(t *testing.T) {
		conn := ttE2EListenUDP(t, "udp6")
		ttE2EUDPEchoViaProxy(t, proxy, conn, ttE2EUDPTarget6, "udp-v6")
		if got := proxy.associationsOf(ttE2EUDPTarget6); len(got) != 1 {
			t.Fatalf("associations carrying %s = %v, want one; proxy saw %s", ttE2EUDPTarget6, got, proxy.summary())
		}
		t.Logf("proxy saw %s", proxy.summary())
	})

	app.selectTUN("")
	ttE2EWaitStatus(t, app, opener, "the rule to run without a TUN", func(rule configs.RuleStatus) bool {
		return rule.Running && rule.Tun == ""
	})
	t.Run("U3 no UDP capture after deselection", func(t *testing.T) {
		before4, before6 := proxy.count(ttE2EUDPTarget4), proxy.count(ttE2EUDPTarget6)
		ttE2EUDPSendAndDrop("udp4", ttE2EUDPTarget4, 2*time.Second)
		ttE2EUDPSendAndDrop("udp6", ttE2EUDPTarget6, 2*time.Second)
		after4, after6 := proxy.count(ttE2EUDPTarget4), proxy.count(ttE2EUDPTarget6)
		if after4 != before4 || after6 != before6 {
			t.Fatalf("proxy datagrams to %s went %d -> %d and to %s %d -> %d after deselecting the TUN, want them unchanged; proxy saw %s", ttE2EUDPTarget4, before4, after4, ttE2EUDPTarget6, before6, after6, proxy.summary())
		}
	})
}

// ttE2EListenUDP opens a plain UDP socket of the test process, closed when the test ends.
func ttE2EListenUDP(t *testing.T, network string) net.PacketConn {
	t.Helper()
	conn, err := net.ListenPacket(network, ":0")
	if err != nil {
		t.Fatalf("listen %s: %v", network, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// ttE2EUDPEchoViaProxy resends payload from conn to target for up to 15s until its echo comes back from target, and checks
// the proxy carried at most one datagram to target per send.
func ttE2EUDPEchoViaProxy(t *testing.T, proxy *ttE2ESOCKS5, conn net.PacketConn, target *net.UDPAddr, payload string) {
	t.Helper()
	before := proxy.count(target)
	// UDP is lossy and a fresh IPv6 address may stay tentative for a moment, so one lost send is not a failure.
	deadline := time.Now().Add(15 * time.Second)
	sends := 0
	for !ttE2EUDPSendAndRead(t, conn, target, payload) {
		sends++
		if time.Now().After(deadline) {
			t.Fatalf("no echo of %q from %s after %d sends; proxy saw %s", payload, target, sends, proxy.summary())
		}
	}
	sends++
	if n := proxy.count(target) - before; n < 1 || n > sends {
		t.Fatalf("proxy datagrams to %s = %d after %d sends, want 1 to %d; proxy saw %s", target, n, sends, sends, proxy.summary())
	}
	t.Logf("echo of %q from %s after %d sends", payload, target, sends)
}

// ttE2EUDPSendAndRead sends payload to target once and reports whether its echo came back from target within a second.
func ttE2EUDPSendAndRead(t *testing.T, conn net.PacketConn, target *net.UDPAddr, payload string) bool {
	t.Helper()
	if _, err := conn.WriteTo([]byte(payload), target); err != nil {
		t.Logf("send to %s: %v", target, err)
		time.Sleep(500 * time.Millisecond)
		return false
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 256)
	for {
		n, source, err := conn.ReadFrom(buf)
		if err != nil {
			t.Logf("no echo from %s: %v", target, err)
			return false
		}
		if string(buf[:n]) != payload {
			// A late echo of an earlier step.
			continue
		}
		if !ttE2ESameUDPAddr(source, target) {
			t.Fatalf("echo of %q came from %v, want %s", payload, source, target)
		}
		return true
	}
}

func ttE2ESameUDPAddr(addr net.Addr, want *net.UDPAddr) bool {
	udp, ok := addr.(*net.UDPAddr)
	if !ok {
		return false
	}
	got, wanted := udp.AddrPort(), want.AddrPort()
	return got.Addr().Unmap() == wanted.Addr().Unmap() && got.Port() == wanted.Port()
}

// ttE2EUDPSendAndDrop sends to target from a fresh socket a few times over wait; whether anything arrives does not matter.
func ttE2EUDPSendAndDrop(network string, target *net.UDPAddr, wait time.Duration) {
	conn, err := net.ListenPacket(network, ":0")
	if err != nil {
		return
	}
	defer conn.Close()
	for range 3 {
		conn.WriteTo([]byte("not-captured"), target)
		time.Sleep(wait / 3)
	}
}

// ttE2ELogInterfaces logs the host's interfaces and their addresses.
func ttE2ELogInterfaces(t *testing.T) {
	t.Helper()
	commands := [][]string{{"ip", "addr"}, {"ip", "route"}, {"ip", "-6", "route"}}
	switch runtime.GOOS {
	case "darwin":
		commands = [][]string{{"ifconfig"}, {"netstat", "-rn"}}
	case "windows":
		commands = [][]string{{"ipconfig", "/all"}, {"netsh", "interface", "ipv6", "show", "addresses"}}
	}
	for _, args := range commands {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		out, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		cancel()
		t.Logf("%s (err %v):\n%s", strings.Join(args, " "), err, out)
	}
}

func TestE2ESOCKS5DoubleEchoesThroughRuleUDPSession(t *testing.T) {
	proxy := ttStartE2ESOCKS5(t, &net.Dialer{}, ttE2EUDPTarget4, ttE2EUDPTarget4Other, ttE2EUDPTarget6)
	rule := config.Rule{Name: "socks", Forward: config.Forward{Way: []bridgeconfig.Node{{LB: []string{"socks5://" + proxy.addr}}}}}
	listen, _ := ttRuleUDP(t, rule, &ttControlDialer{})
	conn := ttSession(t, listen, &net.UDPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 50000})
	for _, target := range []*net.UDPAddr{ttE2EUDPTarget4, ttE2EUDPTarget4Other, ttE2EUDPTarget6} {
		payload := "local-" + target.String()
		if _, err := conn.WriteTo([]byte(payload), target); err != nil {
			t.Fatalf("write to %s: %v", target, err)
		}
		guard := time.AfterFunc(ttWait, func() { conn.Close() })
		buf := make([]byte, 128)
		n, source, err := conn.ReadFrom(buf)
		guard.Stop()
		if err != nil || string(buf[:n]) != payload || source.String() != target.String() {
			t.Fatalf("read = %q from %v, %v; want %q from %s; proxy saw %s", buf[:n], source, err, payload, target, proxy.summary())
		}
		if got := proxy.associationsOf(target); len(got) != 1 || got[0] != 1 || proxy.count(target) != 1 {
			t.Fatalf("proxy saw %s in associations %v %d times, want once in association 1; proxy saw %s", target, got, proxy.count(target), proxy.summary())
		}
	}
	t.Logf("proxy saw %s", proxy.summary())
}

// ttE2ESOCKS5 is a loopback SOCKS5 proxy recording what it carries: it forwards CONNECTs through forward, echoes UDP to
// the echo targets from their own address and drops all other UDP.
type ttE2ESOCKS5 struct {
	addr    string
	echo    []string
	forward *net.Dialer

	mu           sync.Mutex
	connects     []string
	associations int
	datagrams    []ttE2EDatagram
}

// ttE2EDatagram is one datagram a UDP association sent to destination.
type ttE2EDatagram struct {
	association int
	destination string
}

func ttStartE2ESOCKS5(t *testing.T, forward *net.Dialer, echo ...*net.UDPAddr) *ttE2ESOCKS5 {
	t.Helper()
	server, err := socks5.NewSimpleServer("socks5://127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxy := &ttE2ESOCKS5{forward: forward}
	for _, target := range echo {
		proxy.echo = append(proxy.echo, target.String())
	}
	server.ProxyDial = proxy.dial
	// The client's DST names no address to bind, so every relay listens on loopback, which the reply advertises.
	server.ProxyListenPacket = func(ctx context.Context, network, address string) (net.PacketConn, error) {
		return net.ListenPacket("udp4", "127.0.0.1:0")
	}
	server.ProxyOutgoingListenPacket = proxy.associate
	if err := server.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	proxy.addr = server.Address
	return proxy
}

func (p *ttE2ESOCKS5) dial(ctx context.Context, network, address string) (net.Conn, error) {
	p.mu.Lock()
	p.connects = append(p.connects, address)
	p.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return p.forward.DialContext(ctx, network, address)
}

// associate opens the target-side socket of a new UDP association.
func (p *ttE2ESOCKS5) associate(context.Context, string, string) (net.PacketConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.associations++
	return &ttE2ETargetConn{proxy: p, association: p.associations, replies: make(chan ttDatagram, 64), closed: make(chan struct{})}, nil
}

func (p *ttE2ESOCKS5) count(target *net.UDPAddr) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, datagram := range p.datagrams {
		if datagram.destination == target.String() {
			n++
		}
	}
	return n
}

// associationsOf lists the associations that carried a datagram to target, each once.
func (p *ttE2ESOCKS5) associationsOf(target *net.UDPAddr) []int {
	p.mu.Lock()
	defer p.mu.Unlock()
	var associations []int
	for _, datagram := range p.datagrams {
		if datagram.destination == target.String() && !slices.Contains(associations, datagram.association) {
			associations = append(associations, datagram.association)
		}
	}
	return associations
}

// summary tells the associations, the datagrams per association and destination, and the CONNECT targets.
func (p *ttE2ESOCKS5) summary() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	type key struct {
		association int
		destination string
	}
	var order []key
	counts := map[key]int{}
	for _, datagram := range p.datagrams {
		k := key(datagram)
		if counts[k] == 0 {
			order = append(order, k)
		}
		counts[k]++
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d UDP associations;", p.associations)
	for _, k := range order {
		fmt.Fprintf(&b, " #%d->%s x%d", k.association, k.destination, counts[k])
	}
	fmt.Fprintf(&b, "; %d CONNECTs %q", len(p.connects), p.connects)
	return b.String()
}

// ttE2ETargetConn is a UDP association's target-side socket: datagrams to an echo target come back from it, others are dropped.
type ttE2ETargetConn struct {
	net.PacketConn
	proxy       *ttE2ESOCKS5
	association int
	replies     chan ttDatagram
	closed      chan struct{}
	once        sync.Once
}

func (conn *ttE2ETargetConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	destination := addr.String()
	conn.proxy.mu.Lock()
	conn.proxy.datagrams = append(conn.proxy.datagrams, ttE2EDatagram{association: conn.association, destination: destination})
	echo := slices.Contains(conn.proxy.echo, destination)
	conn.proxy.mu.Unlock()
	if echo {
		select {
		case conn.replies <- ttDatagram{payload: slices.Clone(p), source: addr}:
		default:
		}
	}
	return len(p), nil
}

func (conn *ttE2ETargetConn) ReadFrom(p []byte) (int, net.Addr, error) {
	select {
	case datagram := <-conn.replies:
		return copy(p, datagram.payload), datagram.source, nil
	case <-conn.closed:
		return 0, nil, net.ErrClosed
	}
}

func (conn *ttE2ETargetConn) Close() error {
	conn.once.Do(func() { close(conn.closed) })
	return nil
}

func (conn *ttE2ETargetConn) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4zero}
}
