package tunstack

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/tun"
)

// testStopBound is how long Serve may take to return once its context ends.
const testStopBound = 3 * time.Second

func TestServeReturnsContextErrorAndClosesDevice(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	select {
	case <-env.done:
		t.Fatalf("Serve() = %v before its context ended, want it to block until then", env.err)
	case <-time.After(100 * time.Millisecond):
	}
	env.cancel()
	if err := env.wait(t); err != context.Canceled {
		t.Fatalf("Serve() = %v after cancel, want ctx.Err() = context.Canceled", err)
	}
	if !env.dev.closedByServe() {
		t.Fatal("Serve returned without closing the device")
	}
}

func TestServeCancelClosesTargetConns(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	env.dialTCP(t, "10.0.0.9:443")
	tcpTarget := env.nextTarget(t)
	testSend(t, env.dialUDP(t, "10.0.0.53:53"), "query")
	dnsTarget := env.nextTarget(t)
	testReadFrame(t, dnsTarget)
	env.cancel()
	testReadEOF(t, tcpTarget, "TCP flow's target after cancel")
	testReadEOF(t, dnsTarget, "DNS flow's target after cancel")
}

func TestServeDNSShutdownWithStalledRemote(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	testStalledDNS(t, env)
	env.cancel()
	select {
	case <-env.done:
	case <-time.After(testStopBound):
		t.Fatalf("Serve did not return within %v of its context ending while a DNS relay was blocked writing to a stalled TCP/53 conn, want shutdown to close that conn", testStopBound)
	}
}

func TestServeReturnsDeviceReadError(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	injected := errors.New("test read failure")
	env.dev.failRead(injected)
	if err := env.wait(t); !errors.Is(err, injected) {
		t.Fatalf("Serve() = %v after the device Read failed, want an error wrapping %q", err, injected)
	}
	if !env.dev.closedByServe() {
		t.Fatal("Serve returned without closing the device after its Read failed")
	}
}

func TestServeKeepsReadingAfterTooManySegments(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	env.failOneRead(t, tun.ErrTooManySegments)
	conn := env.dialTCP(t, "10.0.0.9:443")
	target := env.nextTarget(t)
	testRelay(t, conn, target, "from the client")
	testRelay(t, target, conn, "from the target")
}

func TestServeDeliversPacketsReadWithTooManySegments(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	env.dev.segmentSYN()
	ctx, cancel := context.WithTimeout(env.alive, testWait)
	defer cancel()
	conn, err := env.client.DialContextTCPAddrPort(ctx, netip.MustParseAddrPort("10.0.0.9:443"))
	if err != nil {
		t.Fatalf("client dial whose only SYN came from a device Read returning (1, tun.ErrTooManySegments): %v%s, want Serve to inject the packet read with that error", err, env.returned())
	}
	if !env.dev.synSegmented.Load() {
		t.Fatal("the client's SYN bypassed the device Read returning tun.ErrTooManySegments")
	}
	conn.SetDeadline(time.Now().Add(testWait))
	target := env.nextTarget(t)
	testRelay(t, conn, target, "from the client")
	testRelay(t, target, conn, "from the target")
}

func TestServeDrainsDeviceEvents(t *testing.T) {
	const sends, wait = 20, 2 * time.Second
	env := testServe(t, Options{Addresses: testAddresses})
	received, all := env.dev.feedEvents(sends)
	select {
	case <-all:
	case <-env.done:
		t.Fatalf("Serve returned %v while the device sent events, want it to ignore them", env.err)
	case <-time.After(wait):
		t.Fatalf("Serve received %d of %d events sent on the device's unbuffered Events channel within %v, want it to drain them all", received.Load(), sends, wait)
	}
}
