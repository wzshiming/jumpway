package tunstack

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"strings"
	"testing"

	"gvisor.dev/gvisor/pkg/tcpip"
)

func TestTCPDialsDestinationWithClientAddr(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	conn := env.dialTCP(t, "10.0.0.9:443")
	env.dialer.expectOnly(t, "10.0.0.9:443", conn.LocalAddr())
}

func TestTCPRelaysBytesBothWays(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	conn := env.dialTCP(t, "10.0.0.9:443")
	target := env.nextTarget(t)
	testRelay(t, conn, target, "from the client")
	testRelay(t, target, conn, "from the target")
}

func TestTCPDialFailureRefusesClient(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	env.dialer.fail(errors.New("test dial failure"))
	ctx, cancel := context.WithTimeout(env.alive, testWait)
	defer cancel()
	conn, err := env.client.DialContextTCPAddrPort(ctx, netip.MustParseAddrPort("10.0.0.9:443"))
	if err == nil {
		conn.Close()
	}
	if want := (&tcpip.ErrConnectionRefused{}).String(); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("client dial error = %v%s, want it to contain %q", err, env.returned(), want)
	}
}

func TestTCPHalfCloseReachesBothEnds(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	conn := env.dialTCP(t, "10.0.0.9:443")
	target := env.nextTarget(t)
	if _, err := io.WriteString(conn, "request"); err != nil {
		t.Fatal(err)
	}
	if err := conn.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	if request, err := io.ReadAll(target); err != nil || string(request) != "request" {
		t.Fatalf(`target ReadAll() = %q, %v after the client's CloseWrite, want "request" ending in EOF`, request, err)
	}
	testRelay(t, target, conn, "late reply")
	if err := target.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	testReadEOF(t, conn, "client after the target's CloseWrite")
}

func TestTCPIPv6DialsBracketedDestination(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	conn := env.dialTCP(t, "[fd00::9]:443")
	env.dialer.expectOnly(t, "[fd00::9]:443", conn.LocalAddr())
}
