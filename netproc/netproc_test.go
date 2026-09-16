package netproc

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestOwnersSelf(t *testing.T) {
	for _, network := range []string{"tcp4", "tcp6"} {
		t.Run(network, func(t *testing.T) {
			address := "127.0.0.1:0"
			if network == "tcp6" {
				address = "[::1]:0"
			}
			listener, err := net.Listen(network, address)
			if err != nil {
				if network == "tcp6" {
					t.Skip(err)
				}
				t.Fatal(err)
			}
			t.Cleanup(func() { listener.Close() })
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			t.Cleanup(cancel)
			dialer := net.Dialer{}
			conn, err := dialer.DialContext(ctx, network, listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { conn.Close() })
			if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			accepted, err := listener.Accept()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { accepted.Close() })
			owners, err := Owners(ctx)
			if errors.Is(err, errors.ErrUnsupported) {
				t.Skip(err)
			}
			if err != nil {
				t.Fatal(err)
			}
			local := netip.MustParseAddrPort(conn.LocalAddr().String())
			local = netip.AddrPortFrom(local.Addr().Unmap(), local.Port())
			process := owners[local]
			if process.PID != os.Getpid() || process.Name == "" {
				t.Fatalf("owner of %s = %+v, want PID %d and nonempty name", local, process, os.Getpid())
			}
			t.Logf("owner of %s = %+v", local, process)
		})
	}
}

func TestParseLsof(t *testing.T) {
	const fixture = `p20102
cCode Helper
f21
n127.0.0.1:52912->127.0.0.1:1088
f22
n127.0.0.1:59732->127.0.0.1:1088
p42
cnetproc.test
n[::1]:52913->[::1]:1088
n[::ffff:127.0.0.1]:52914->127.0.0.1:1088
n*:1088
ngarbage->garbage
`
	want := map[netip.AddrPort]Process{
		netip.MustParseAddrPort("127.0.0.1:52912"): {PID: 20102, Name: "Code Helper"},
		netip.MustParseAddrPort("127.0.0.1:59732"): {PID: 20102, Name: "Code Helper"},
		netip.MustParseAddrPort("[::1]:52913"):     {PID: 42, Name: "netproc.test"},
		netip.MustParseAddrPort("127.0.0.1:52914"): {PID: 42, Name: "netproc.test"},
	}
	if got := parseLsof(strings.NewReader(fixture)); !reflect.DeepEqual(got, want) {
		t.Fatalf("owners = %+v, want %+v", got, want)
	}
}

func TestParseProcNet(t *testing.T) {
	if binary.NativeEndian.Uint32([]byte{1, 0, 0, 0}) != 1 {
		t.Skip("fixtures use little-endian words")
	}
	const tcp = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:D431 0100007F:0440 01 00000000:00000000 00:00000000 00000000 1000 0 123456 1 0000000000000000 20 4 30 10 -1
   1: 0100007F:0440 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 123457 1 0000000000000000 100 0 0 10 0
`
	want := map[uint64]netip.AddrPort{123456: netip.MustParseAddrPort("127.0.0.1:54321")}
	if got := parseProcNet(strings.NewReader(tcp)); !reflect.DeepEqual(got, want) {
		t.Fatalf("tcp = %+v, want %+v", got, want)
	}
	const tcp6 = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000001000000:D431 00000000000000000000000001000000:0440 01 00000000:00000000 00:00000000 00000000 1000 0 123458 1
   1: 0000000000000000FFFF00000100007F:D431 0000000000000000FFFF00000100007F:0440 01 00000000:00000000 00:00000000 00000000 1000 0 123459 1
`
	want = map[uint64]netip.AddrPort{
		123458: netip.MustParseAddrPort("[::1]:54321"),
		123459: netip.MustParseAddrPort("127.0.0.1:54321"),
	}
	if got := parseProcNet(strings.NewReader(tcp6)); !reflect.DeepEqual(got, want) {
		t.Fatalf("tcp6 = %+v, want %+v", got, want)
	}
}
