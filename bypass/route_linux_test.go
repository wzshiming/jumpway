//go:build linux

package bypass

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const (
	ttAny6    = "00000000000000000000000000000000"
	ttRouter6 = "fe800000000000000000000000000001"
)

// ttRoute4 renders /proc/net/route: the header, then rows, each padded to 127 columns as the kernel does.
func ttRoute4(rows ...string) io.Reader {
	var b strings.Builder
	for _, row := range append([]string{"Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT"}, rows...) {
		fmt.Fprintf(&b, "%-127s\n", row)
	}
	return strings.NewReader(b.String())
}

// ttRow4 is one /proc/net/route row; addresses and masks are the kernel's little-endian hex.
func ttRow4(iface, dst, gateway, flags, metric, mask string) string {
	return strings.Join([]string{iface, dst, gateway, flags, "0", "0", metric, mask, "0", "0", "0"}, "\t")
}

// ttRow6 is one /proc/net/ipv6_route line as the kernel prints it: hex metric and flags, dev right-aligned.
func ttRow6(dst string, dstLen int, nexthop string, metric, flags uint32, dev string) string {
	return fmt.Sprintf("%s %02x %s 00 %s %08x 00000001 00000000 %08x %8s\n", dst, dstLen, ttAny6, nexthop, metric, flags, dev)
}

func ttRoute6(lines ...string) io.Reader { return strings.NewReader(strings.Join(lines, "")) }

func TestParseRoute4PicksLowestMetricDefault(t *testing.T) {
	got, err := parseRoute4(ttRoute4(
		ttRow4("wlan0", "00000000", "0101A8C0", "0003", "600", "00000000"),
		ttRow4("eth0", "00000000", "0100000A", "0003", "100", "00000000"),
		ttRow4("eth0", "0000000A", "00000000", "0001", "100", "000000FF"),
	))
	if err != nil || got != "eth0" {
		t.Errorf("parseRoute4 = (%q, %v), want (\"eth0\", nil): the metric-100 default beats the earlier metric-600 one", got, err)
	}
}

func TestParseRoute4WithoutDefaultReturnsErrNoDefaultRoute(t *testing.T) {
	tests := []struct {
		name string
		rows []string
	}{
		{"header only", nil},
		{"subnet and 0/1 128/1 split routes only", []string{
			ttRow4("eth0", "0000000A", "00000000", "0001", "100", "000000FF"),
			ttRow4("tun0", "00000000", "00000000", "0001", "0", "00000080"),
			ttRow4("tun0", "00000080", "00000000", "0001", "0", "00000080"),
		}},
		{"default not up", []string{ttRow4("eth0", "00000000", "0100000A", "0002", "100", "00000000")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := parseRoute4(ttRoute4(tc.rows...)); !errors.Is(err, ErrNoDefaultRoute) {
				t.Errorf("parseRoute4 = (%q, %v), want ErrNoDefaultRoute", got, err)
			}
		})
	}
}

func TestParseRoute4SkipsMalformedRows(t *testing.T) {
	got, err := parseRoute4(ttRoute4(
		"bad0\t00000000\t0101A8C0",
		ttRow4("bad1", "00000000", "0101A8C0", "0003", "QQ", "00000000"),
		ttRow4("eth0", "00000000", "0100000A", "0003", "100", "00000000"),
	))
	if err != nil || got != "eth0" {
		t.Errorf("parseRoute4 = (%q, %v), want (\"eth0\", nil) past a truncated row and a bad-metric row", got, err)
	}
}

func TestParseRoute6PicksLowestMetricDefault(t *testing.T) {
	got, err := parseRoute6(ttRoute6(
		ttRow6(ttAny6, 0, ttRouter6, 600, 0x00450003, "wlan0"),
		ttRow6(ttAny6, 0, ttRouter6, 100, 0x00450003, "eth0"),
		ttRow6("fe800000000000000000000000000000", 64, ttAny6, 256, 0x00000001, "eth0"),
		ttRow6(ttAny6, 0, ttAny6, 0xffffffff, 0x00200200, "lo"),
	))
	if err != nil || got != "eth0" {
		t.Errorf("parseRoute6 = (%q, %v), want (\"eth0\", nil): the metric-100 default beats the earlier metric-600 one", got, err)
	}
}

func TestParseRoute6WithoutDefaultReturnsErrNoDefaultRoute(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
	}{
		{"empty file", nil},
		{"link-local and ::/1 8000::/1 split routes only", []string{
			ttRow6("fe800000000000000000000000000000", 64, ttAny6, 256, 0x00000001, "eth0"),
			ttRow6(ttAny6, 1, ttAny6, 1024, 0x00000001, "tun0"),
			ttRow6("80000000000000000000000000000000", 1, ttAny6, 1024, 0x00000001, "tun0"),
		}},
		{"the kernel's unreachable default on lo", []string{ttRow6(ttAny6, 0, ttAny6, 0xffffffff, 0x00200200, "lo")}},
		{"an up default on lo", []string{ttRow6(ttAny6, 0, ttAny6, 1024, 0x00000001, "lo")}},
		{"default not up", []string{ttRow6(ttAny6, 0, ttRouter6, 1024, 0x00450002, "eth0")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := parseRoute6(ttRoute6(tc.lines...)); !errors.Is(err, ErrNoDefaultRoute) {
				t.Errorf("parseRoute6 = (%q, %v), want ErrNoDefaultRoute", got, err)
			}
		})
	}
}

func TestParseRoute6SkipsMalformedRows(t *testing.T) {
	got, err := parseRoute6(ttRoute6(
		"00000000000000000000000000000000 00\n",
		"00000000000000000000000000000000 00 00000000000000000000000000000000 00 fe800000000000000000000000000001 QQQQQQQQ 00000001 00000000 00450003     bad1\n",
		ttRow6(ttAny6, 0, ttRouter6, 100, 0x00450003, "eth0"),
	))
	if err != nil || got != "eth0" {
		t.Errorf("parseRoute6 = (%q, %v), want (\"eth0\", nil) past a truncated row and a bad-metric row", got, err)
	}
}

func TestNewDialerBindsUDPToIPv4DefaultInterface(t *testing.T) {
	iface, err := DefaultInterface(false)
	if errors.Is(err, ErrNoDefaultRoute) {
		t.Skip("no IPv4 default route on this host")
	}
	if err != nil {
		t.Fatalf("DefaultInterface(false) error = %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	// Connecting a UDP socket only records the peer; no packet leaves the host.
	conn, err := NewDialer(ttCaptureLab).DialContext(ctx, "udp4", "198.18.0.1:9")
	if errors.Is(err, syscall.EPERM) {
		t.Skipf("SO_BINDTODEVICE needs privilege on this kernel: %v", err)
	}
	if err != nil {
		t.Fatalf("DialContext(udp4, 198.18.0.1:9) error = %v", err)
	}
	defer conn.Close()
	sc, ok := conn.(syscall.Conn)
	if !ok {
		t.Fatalf("%T is not a syscall.Conn", conn)
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn error = %v", err)
	}
	var device string
	var optErr error
	if err := raw.Control(func(fd uintptr) {
		device, optErr = unix.GetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE)
	}); err != nil {
		t.Fatalf("RawConn.Control error = %v", err)
	}
	if optErr != nil {
		t.Fatalf("getsockopt(SO_BINDTODEVICE) error = %v", optErr)
	}
	if device != iface.Name {
		t.Errorf("SO_BINDTODEVICE = %q, want %q (it holds the IPv4 default route)", device, iface.Name)
	}
}
