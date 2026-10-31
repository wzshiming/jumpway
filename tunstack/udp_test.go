package tunstack

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
)

// testIdleBound is how long a 200ms DNSIdle may take to end a flow; the 10s default exceeds it.
const testIdleBound = 5 * time.Second

// testDNSQuery is a well-formed DNS query for the A record of example.com.
const testDNSQuery = "\x12\x34\x01\x00\x00\x01\x00\x00\x00\x00\x00\x00\x07example\x03com\x00\x00\x01\x00\x01"

func TestUDPDNSDialsTCPWithClientAddr(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	conn := env.dialUDP(t, "10.0.0.53:53")
	testSend(t, conn, "query")
	env.nextTarget(t)
	env.dialer.expectOnly(t, "10.0.0.53:53", conn.LocalAddr())
}

func TestUDPDNSRelaysLengthFramedMessages(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	conn := env.dialUDP(t, "10.0.0.53:53")
	testSend(t, conn, "query")
	target := env.nextTarget(t)
	if got := testReadFrame(t, target); got != "query" {
		t.Fatalf("target read the framed message %q, want the datagram %q", got, "query")
	}
	testWriteFrame(t, target, "reply")
	reply := make([]byte, 512)
	n, err := conn.Read(reply)
	if err != nil || string(reply[:n]) != "reply" {
		t.Fatalf("client Read() = %q, %v, want the unframed reply %q", reply[:n], err, "reply")
	}
}

func TestUDPDNSFlowReusesTCPConn(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	conn := env.dialUDP(t, "10.0.0.53:53")
	testSend(t, conn, "first")
	target := env.nextTarget(t)
	testReadFrame(t, target)
	testSend(t, conn, "second")
	if got := testReadFrame(t, target); got != "second" {
		t.Fatalf("the flow's TCP conn read %q, want the second datagram %q", got, "second")
	}
	if dials := env.dialer.recorded(); len(dials) != 1 {
		t.Fatalf("dialer.DialContext called %d times for one UDP flow, want 1", len(dials))
	}
}

func TestUDPDNSFlowEndsAfterIdle(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses, DNSIdle: 200 * time.Millisecond})
	conn := env.dialUDP(t, "10.0.0.53:53")
	testSend(t, conn, "query")
	target := env.nextTarget(t)
	testReadFrame(t, target)
	if err := target.SetReadDeadline(time.Now().Add(testIdleBound)); err != nil {
		t.Fatal(err)
	}
	testReadEOF(t, target, "target of a flow idle past Options.DNSIdle 200ms")
}

func TestUDPDNSFlowWithStalledRemoteEndsAfterIdle(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses, DNSIdle: 200 * time.Millisecond})
	remote := testStalledDNS(t, env)
	select {
	case <-remote.closed:
	case <-time.After(testIdleBound):
		t.Fatalf("a DNS relay blocked writing to its stalled TCP/53 conn still held it open %v later, want the Options.DNSIdle 200ms idle timer to close it", testIdleBound)
	}
}

func TestUDPOtherPortGetsPortUnreachable(t *testing.T) {
	env := testServe(t, Options{Addresses: testAddresses})
	conn := env.dialUDP(t, "10.0.0.9:1234")
	testSend(t, conn, "not dns")
	// A blocked gonet Read does not wake for the ICMP error, so poll with short deadlines.
	reply := make([]byte, 512)
	var err error
	for start := time.Now(); ; {
		if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		var n int
		n, err = conn.Read(reply)
		if err == nil {
			t.Fatalf("client read %q from 10.0.0.9:1234, want ICMP port unreachable", reply[:n])
		}
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() || time.Since(start) > testWait {
			break
		}
		if returned := env.returned(); returned != "" {
			t.Fatalf("no ICMP port unreachable reached the client%s", returned)
		}
	}
	if want := (&tcpip.ErrConnectionRefused{}).String(); !strings.Contains(err.Error(), want) {
		t.Fatalf("client Read() error = %v, want %q from ICMP port unreachable", err, want)
	}
	if dials := env.dialer.recorded(); len(dials) != 0 {
		t.Fatalf("dialer.DialContext called for UDP to port 1234: %+v, want no dial", dials)
	}
}

func testSend(t *testing.T, conn net.Conn, datagram string) {
	t.Helper()
	if _, err := io.WriteString(conn, datagram); err != nil {
		t.Fatalf("client send %q: %v", datagram, err)
	}
}

// testStalledDNS opens a DNS flow whose TCP/53 conn stalls and returns that conn once the relay is blocked writing a query to it.
func testStalledDNS(t *testing.T, env *testEnv) *stalledConn {
	t.Helper()
	env.dialer.stall()
	testSend(t, env.dialUDP(t, "10.0.0.53:53"), testDNSQuery)
	remote := env.nextTarget(t).(*stalledConn)
	select {
	case <-remote.writing:
	case <-time.After(testWait):
		t.Fatalf("the DNS relay did not write the query to its TCP/53 conn within %v%s", testWait, env.returned())
	}
	return remote
}

// testReadFrame reads one message prefixed with its 2-byte big-endian length.
func testReadFrame(t *testing.T, conn net.Conn) string {
	t.Helper()
	var size [2]byte
	if _, err := io.ReadFull(conn, size[:]); err != nil {
		t.Fatalf("read the 2-byte length of a relayed DNS message: %v", err)
	}
	message := make([]byte, binary.BigEndian.Uint16(size[:]))
	if _, err := io.ReadFull(conn, message); err != nil {
		t.Fatalf("read a %d-byte relayed DNS message: %v", len(message), err)
	}
	return string(message)
}

func testWriteFrame(t *testing.T, conn net.Conn, message string) {
	t.Helper()
	frame := binary.BigEndian.AppendUint16(nil, uint16(len(message)))
	if _, err := conn.Write(append(frame, message...)); err != nil {
		t.Fatalf("write the framed reply %q: %v", message, err)
	}
}
