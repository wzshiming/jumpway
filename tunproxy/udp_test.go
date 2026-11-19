package tunproxy

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/wzshiming/tun/stack"
)

func TestListenConfigsRefuseStreamListen(test *testing.T) {
	for name, listen := range map[string]stack.ListenConfig{"direct": Direct(nil), "proxied": Proxied(&ttDialer{}, ProxiedOptions{})} {
		if _, err := listen.Listen(context.Background(), "tcp", ":0"); !errors.Is(err, errors.ErrUnsupported) {
			test.Errorf("%s: Listen error = %v, want errors.ErrUnsupported", name, err)
		}
	}
}

func TestDirectToLoopbackIsPlainFullCone(test *testing.T) {
	captures, _ := ttSeamBoundListenConfig(test)
	capture := []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")}
	conn, err := Direct(capture).ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		test.Fatal(err)
	}
	defer conn.Close()
	ttAssertFullCone(test, conn)
	if len(*captures) != 0 {
		test.Fatalf("bound listen configs for captures %v, want none for loopback", *captures)
	}
}

func TestDirectBindsSocketForCapturedFirstDestination(test *testing.T) {
	for _, scenario := range []struct {
		network, destination, want string
	}{
		{network: "udp", destination: "192.0.2.1:9", want: "udp4"},
		{network: "udp4", destination: "192.0.2.1:9", want: "udp4"},
		{network: "udp", destination: "[2001:db8::1]:9", want: "udp6"},
		{network: "udp6", destination: "[2001:db8::1]:9", want: "udp6"},
	} {
		test.Run(scenario.network+" to "+scenario.destination, func(test *testing.T) {
			captures, opened := ttSeamBoundListenConfig(test)
			capture := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("2001:db8::/32")}
			conn, err := Direct(capture).ListenPacket(context.Background(), scenario.network, ":0")
			if err != nil {
				test.Fatal(err)
			}
			defer conn.Close()
			if len(*captures) != 0 {
				test.Fatalf("bound listen configs %v before the first datagram, want none", *captures)
			}
			// The destination may be unroutable here; only the socket it opens matters.
			conn.WriteTo([]byte("x"), net.UDPAddrFromAddrPort(netip.MustParseAddrPort(scenario.destination)))
			conn.WriteTo([]byte("x"), ttUDPEcho(test))
			if want := [][]netip.Prefix{capture}; !reflect.DeepEqual(*captures, want) {
				test.Fatalf("bound listen configs for captures %v, want %v", *captures, want)
			}
			if want := []string{scenario.want}; !slices.Equal(*opened, want) {
				test.Fatalf("bound sockets %v, want %v", *opened, want)
			}
		})
	}
}

func TestDirectLeavesOtherFirstDestinationsPlain(test *testing.T) {
	capture := []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0")}
	for name, scenario := range map[string]struct {
		capture     []netip.Prefix
		destination string
	}{
		"outside capture": {capture: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}, destination: "198.51.100.1:9"},
		"loopback":        {capture: capture, destination: "127.0.0.1:9"},
		"IPv6 loopback":   {capture: capture, destination: "[::1]:9"},
		"link-local":      {capture: capture, destination: "169.254.1.1:9"},
		"multicast":       {capture: capture, destination: "224.0.0.251:5353"},
		"unspecified":     {capture: capture, destination: "0.0.0.0:9"},
	} {
		test.Run(name, func(test *testing.T) {
			captures, _ := ttSeamBoundListenConfig(test)
			conn, err := Direct(scenario.capture).ListenPacket(context.Background(), "udp", ":0")
			if err != nil {
				test.Fatal(err)
			}
			defer conn.Close()
			conn.WriteTo([]byte("x"), net.UDPAddrFromAddrPort(netip.MustParseAddrPort(scenario.destination)))
			if len(*captures) != 0 {
				test.Fatalf("bound listen configs for captures %v, want a plain socket", *captures)
			}
			if port := conn.LocalAddr().(*net.UDPAddr).Port; port == 0 {
				test.Fatalf("local address %v after the first datagram, want an open socket", conn.LocalAddr())
			}
		})
	}
}

func TestDirectFamilyFollowsFirstDestination(test *testing.T) {
	conn, err := Direct(nil).ListenPacket(context.Background(), "udp", ":0")
	if err != nil {
		test.Fatal(err)
	}
	defer conn.Close()
	ttAssertFullCone(test, conn)
	if ip := conn.LocalAddr().(*net.UDPAddr).IP; ip.To4() == nil {
		test.Fatalf("local address %v after an IPv4 destination, want an IPv4 socket", conn.LocalAddr())
	}
}

func TestDirectBeforeFirstDatagram(test *testing.T) {
	conn, err := Direct(nil).ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		test.Fatal(err)
	}
	if addr := conn.LocalAddr(); addr == nil {
		test.Fatal("LocalAddr = nil before the first datagram, want an address")
	}
	conn.SetDeadline(time.Time{})
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})
	read := make(chan error, 1)
	go func() {
		_, _, err := conn.ReadFrom(make([]byte, 8))
		read <- err
	}()
	if err := conn.Close(); err != nil {
		test.Fatalf("Close = %v, want nil", err)
	}
	select {
	case err := <-read:
		if !errors.Is(err, net.ErrClosed) {
			test.Fatalf("ReadFrom after Close = %v, want net.ErrClosed", err)
		}
	case <-time.After(ttWait):
		test.Fatalf("ReadFrom still blocked %v after Close", ttWait)
	}
	if err := conn.Close(); err != nil {
		test.Fatalf("second Close = %v, want nil", err)
	}
	if _, err := conn.WriteTo([]byte("x"), ttUDPEcho(test)); !errors.Is(err, net.ErrClosed) {
		test.Fatalf("WriteTo after Close = %v, want net.ErrClosed", err)
	}
}

func TestDirectReadWaitsForFirstDatagram(test *testing.T) {
	conn, err := Direct(nil).ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		test.Fatal(err)
	}
	defer conn.Close()
	read := make(chan string, 1)
	go func() {
		buf := make([]byte, 8)
		n, _, _ := conn.ReadFrom(buf)
		read <- string(buf[:n])
	}()
	if _, err := conn.WriteTo([]byte("echo"), ttUDPEcho(test)); err != nil {
		test.Fatal(err)
	}
	select {
	case payload := <-read:
		if payload != "echo" {
			test.Fatalf("read %q, want the echo of the first datagram", payload)
		}
	case <-time.After(ttWait):
		test.Fatalf("no echo read within %v", ttWait)
	}
}

// ttSeamBoundListenConfig swaps newBoundListenConfig for one recording its captures and the network of each socket it opens.
func ttSeamBoundListenConfig(test *testing.T) (captures *[][]netip.Prefix, opened *[]string) {
	captures, opened = &[][]netip.Prefix{}, &[]string{}
	previous := newBoundListenConfig
	test.Cleanup(func() { newBoundListenConfig = previous })
	newBoundListenConfig = func(capture []netip.Prefix) *net.ListenConfig {
		*captures = append(*captures, capture)
		return &net.ListenConfig{Control: func(network, address string, _ syscall.RawConn) error {
			*opened = append(*opened, network)
			return nil
		}}
	}
	return captures, opened
}

// ttAssertFullCone writes from conn to two echo servers and wants each echo back from its source.
func ttAssertFullCone(test *testing.T, conn net.PacketConn) {
	test.Helper()
	want := map[string]string{}
	for _, payload := range []string{"first", "second"} {
		echo := ttUDPEcho(test)
		want[echo.String()] = payload
		if _, err := conn.WriteTo([]byte(payload), echo); err != nil {
			test.Fatalf("write to %s: %v", echo, err)
		}
	}
	got := map[string]string{}
	conn.SetReadDeadline(time.Now().Add(ttWait))
	buf := make([]byte, 64)
	for len(got) < len(want) {
		n, source, err := conn.ReadFrom(buf)
		if err != nil {
			test.Fatalf("read echoes %v: %v, want %v", got, err, want)
		}
		udp, ok := source.(*net.UDPAddr)
		if !ok {
			test.Fatalf("echo source %v is a %T, want a *net.UDPAddr", source, source)
		}
		got[udp.String()] = string(buf[:n])
	}
	if !reflect.DeepEqual(got, want) {
		test.Fatalf("echoes by source %v, want %v", got, want)
	}
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
