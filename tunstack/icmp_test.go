package tunstack

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// testSilence is how long an echo request must stay unanswered.
const testSilence = time.Second

func TestICMPEchoAnsweredOnlyForGateway(t *testing.T) {
	for _, family := range []struct {
		name                   string
		client, gateway, other netip.Addr
	}{
		{"ipv4", testClient4, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.9")},
		{"ipv6", testClient6, netip.MustParseAddr("fd00::1"), netip.MustParseAddr("fd00::9")},
	} {
		t.Run(family.name, func(t *testing.T) {
			env := testServe(t, Options{Addresses: testAddresses})
			if !testPing(t, env, family.client, family.gateway, testWait) {
				t.Fatalf("echo to the gateway address %v got no reply within %v, want the stack to answer it", family.gateway, testWait)
			}
			if testPing(t, env, family.client, family.other, testSilence) {
				t.Fatalf("echo to %v, not in Options.Addresses, was answered; want it consumed without reply", family.other)
			}
		})
	}
}

// testPing reports whether echo requests from src to dst get a reply within wait.
func testPing(t *testing.T, env *testEnv, src, dst netip.Addr, wait time.Duration) bool {
	t.Helper()
	conn, err := env.client.DialPingAddr(src, dst)
	if err != nil {
		t.Fatalf("client DialPingAddr(%v, %v): %v", src, dst, err)
	}
	request, reply := byte(header.ICMPv4Echo), byte(header.ICMPv4EchoReply)
	if dst.Is6() {
		request, reply = byte(header.ICMPv6EchoRequest), byte(header.ICMPv6EchoReply)
	}
	if err := conn.SetReadDeadline(time.Now().Add(wait)); err != nil {
		t.Fatal(err)
	}
	replied := make(chan struct{})
	reading := make(chan struct{})
	go func() {
		defer close(reading)
		message := make([]byte, testMTU)
		for {
			n, _, err := conn.ReadFrom(message)
			if err != nil {
				return
			}
			if n > 0 && message[0] == reply {
				close(replied)
				return
			}
		}
	}()
	defer func() {
		conn.Close()
		<-reading
	}()
	// PingConn.ReadFrom misses a reply that arrives before it waits, so keep sending until one is read.
	echo := []byte{request, 0, 0, 0, 0, 0, 0, 0, 't', 'e', 's', 't'}
	timeout := time.NewTimer(wait)
	defer timeout.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for seq := uint16(1); ; seq++ {
		binary.BigEndian.PutUint16(echo[6:], seq)
		if _, err := conn.Write(echo); err != nil {
			t.Fatalf("client echo request to %v: %v", dst, err)
		}
		select {
		case <-replied:
			return true
		case <-timeout.C:
			return false
		case <-env.done:
			t.Fatalf("Serve returned %v while %v was being pinged", env.err, dst)
		case <-tick.C:
		}
	}
}
