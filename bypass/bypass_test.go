package bypass

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"syscall"
	"testing"
	"time"
)

var (
	ttT0         = time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)
	ttErrLookup  = errors.New("tt: lookup failed")
	ttErrBind    = errors.New("tt: bind failed")
	ttCaptureAll = []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0")}
	ttCaptureLab = []netip.Prefix{netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("2001:db8::/32")}
)

const (
	ttWithinTTL = 999 * time.Millisecond
	ttPastTTL   = time.Second + time.Millisecond
)

func ttNow() time.Time { return ttT0 }

func ttIface(n int) boundIface { return boundIface{Index: n, Name: fmt.Sprintf("tt%d", n)} }

// ttLookup is a counting lookupFunc: it records each call's family, and call n returns ttIface(n) unless err is set.
type ttLookup struct {
	err      error
	families []bool
}

func (l *ttLookup) lookup(ipv6 bool) (boundIface, error) {
	l.families = append(l.families, ipv6)
	if l.err != nil {
		return boundIface{}, l.err
	}
	return ttIface(len(l.families)), nil
}

// ttRawConn stands in for a dialed socket; fd keeps it non-zero-size, so distinct fakes never share a pointer.
type ttRawConn struct{ fd uintptr }

func (c *ttRawConn) Control(f func(fd uintptr)) error {
	f(c.fd)
	return nil
}

func (c *ttRawConn) Read(func(fd uintptr) bool) error { return errors.ErrUnsupported }

func (c *ttRawConn) Write(func(fd uintptr) bool) error { return errors.ErrUnsupported }

type ttBindCall struct {
	conn  syscall.RawConn
	iface boundIface
	ipv6  bool
}

// ttBinder is a bindFunc recording each call, including the RawConn it was handed.
type ttBinder struct {
	err   error
	calls []ttBindCall
}

func (b *ttBinder) bind(c syscall.RawConn, iface boundIface, ipv6 bool) error {
	b.calls = append(b.calls, ttBindCall{conn: c, iface: iface, ipv6: ipv6})
	return b.err
}

// ttControl wires newDialer to the fakes, capturing every destination, at a clock frozen at ttT0.
func ttControl(t *testing.T, l *ttLookup, b *ttBinder) func(network, address string, c syscall.RawConn) error {
	t.Helper()
	d := newDialer(ttCaptureAll, l.lookup, b.bind, ttNow)
	if d == nil || d.Control == nil {
		t.Fatal("newDialer returned no Control hook")
	}
	return d.Control
}

func TestShouldBind(t *testing.T) {
	tests := []struct {
		name     string
		network  string
		address  string
		capture  []netip.Prefix
		wantIPv6 bool
		wantOK   bool
	}{
		{"IPv4 inside the capture over tcp4", "tcp4", "198.18.5.5:80", ttCaptureLab, false, true},
		{"IPv6 inside the capture over tcp6", "tcp6", "[2001:db8::1]:443", ttCaptureLab, true, true},
		{"IPv4 inside the capture over udp4", "udp4", "198.18.5.5:53", ttCaptureLab, false, true},
		{"IPv6 inside the capture over udp6", "udp6", "[2001:db8::1]:53", ttCaptureLab, true, true},
		{"IPv4 over tcp without a family suffix", "tcp", "198.18.5.5:80", ttCaptureLab, false, true},
		{"IPv6 over tcp without a family suffix", "tcp", "[2001:db8::1]:443", ttCaptureLab, true, true},
		{"IPv4 over udp without a family suffix", "udp", "198.18.5.5:53", ttCaptureLab, false, true},
		{"IPv4 over ip without a family suffix", "ip", "198.18.5.5:0", ttCaptureLab, false, true},
		{"IPv4-mapped IPv6 inside the capture over tcp6", "tcp6", "[::ffff:198.18.5.5]:80", ttCaptureLab, true, true},
		{"IPv4-mapped IPv6 inside the capture over udp6", "udp6", "[::ffff:198.18.5.5]:53", ttCaptureLab, true, true},
		{"IPv4-mapped IPv6 inside the capture over tcp4", "tcp4", "[::ffff:198.18.5.5]:80", ttCaptureLab, false, true},
		{"IPv4-mapped IPv6 over tcp without a family suffix", "tcp", "[::ffff:198.18.5.5]:80", ttCaptureLab, false, true},
		{"IPv6 literal over tcp4", "tcp4", "[2001:db8::1]:443", ttCaptureAll, false, false},
		{"unmapped IPv6 inside the capture over tcp4", "tcp4", "[2001:db8::1]:80", ttCaptureLab, false, false},
		{"plain IPv4 inside the capture over tcp6", "tcp6", "198.18.5.5:80", ttCaptureLab, false, false},
		{"IPv4 loopback", "tcp4", "127.0.0.1:80", ttCaptureAll, false, false},
		{"IPv6 loopback", "tcp6", "[::1]:80", ttCaptureAll, false, false},
		{"IPv4 link-local unicast", "tcp4", "169.254.10.10:80", ttCaptureAll, false, false},
		{"IPv6 link-local unicast", "tcp6", "[fe80::1]:80", ttCaptureAll, false, false},
		{"IPv4 link-local multicast", "udp4", "224.0.0.251:5353", ttCaptureAll, false, false},
		{"IPv6 link-local multicast", "udp6", "[ff02::fb]:5353", ttCaptureAll, false, false},
		{"IPv4 multicast", "udp4", "239.255.255.250:1900", ttCaptureAll, false, false},
		{"IPv6 multicast", "udp6", "[ff05::1:3]:547", ttCaptureAll, false, false},
		{"IPv4 unspecified", "tcp4", "0.0.0.0:80", ttCaptureAll, false, false},
		{"IPv6 unspecified", "tcp6", "[::]:80", ttCaptureAll, false, false},
		{"host name", "tcp", "example.com:80", ttCaptureAll, false, false},
		{"IPv4 outside the capture", "tcp4", "203.0.113.7:80", ttCaptureLab, false, false},
		{"IPv6 outside the capture", "tcp6", "[2001:db9::1]:443", ttCaptureLab, false, false},
		{"IPv4-mapped IPv6 outside the capture over tcp6", "tcp6", "[::ffff:203.0.113.7]:80", ttCaptureLab, false, false},
		// Containment is tested on the unmapped IPv4, which no IPv6 prefix contains.
		{"IPv4-mapped IPv6 over tcp6 with only IPv6 captured", "tcp6", "[::ffff:198.18.5.5]:80", []netip.Prefix{netip.MustParsePrefix("::/0")}, false, false},
		{"empty capture", "tcp4", "198.18.5.5:80", nil, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ipv6, ok := shouldBind(tc.network, tc.address, tc.capture)
			if ok != tc.wantOK || ok && ipv6 != tc.wantIPv6 {
				t.Errorf("shouldBind(%q, %q, %v) = (%t, %t), want (%t, %t)", tc.network, tc.address, tc.capture, ipv6, ok, tc.wantIPv6, tc.wantOK)
			}
		})
	}
}

func TestRouteCacheReusesLookupWithinTTL(t *testing.T) {
	var cache routeCache
	l := &ttLookup{}
	for _, at := range []time.Duration{0, ttWithinTTL} {
		if got, err := cache.get(false, ttT0.Add(at), l.lookup); err != nil || got != ttIface(1) {
			t.Fatalf("get at +%v = (%+v, %v), want the first lookup's (%+v, nil)", at, got, err, ttIface(1))
		}
	}
	if len(l.families) != 1 {
		t.Errorf("lookups = %d for two gets %v apart, want 1", len(l.families), ttWithinTTL)
	}
}

func TestRouteCacheLooksUpAgainAfterTTL(t *testing.T) {
	var cache routeCache
	l := &ttLookup{}
	if _, err := cache.get(false, ttT0, l.lookup); err != nil {
		t.Fatalf("first get error = %v", err)
	}
	if got, err := cache.get(false, ttT0.Add(ttPastTTL), l.lookup); err != nil || got != ttIface(2) {
		t.Errorf("get %v later = (%+v, %v), want a fresh lookup's (%+v, nil)", ttPastTTL, got, err, ttIface(2))
	}
}

func TestRouteCacheCachesLookupErrorForTTL(t *testing.T) {
	var cache routeCache
	l := &ttLookup{err: ttErrLookup}
	for _, at := range []time.Duration{0, ttWithinTTL} {
		if _, err := cache.get(false, ttT0.Add(at), l.lookup); !errors.Is(err, ttErrLookup) {
			t.Fatalf("get at +%v error = %v, want %v", at, err, ttErrLookup)
		}
	}
	if len(l.families) != 1 {
		t.Fatalf("lookups = %d for two failing gets %v apart, want 1: the error was not cached", len(l.families), ttWithinTTL)
	}
	l.err = nil
	if _, err := cache.get(false, ttT0.Add(ttPastTTL), l.lookup); err != nil {
		t.Errorf("get %v after the failure error = %v, want nil: the cached error outlived the TTL", ttPastTTL, err)
	}
}

func TestRouteCacheInvalidateForcesLookup(t *testing.T) {
	var cache routeCache
	l := &ttLookup{}
	if _, err := cache.get(false, ttT0, l.lookup); err != nil {
		t.Fatalf("first get error = %v", err)
	}
	cache.invalidate(false)
	if got, err := cache.get(false, ttT0, l.lookup); err != nil || got != ttIface(2) {
		t.Errorf("get after invalidate(false) = (%+v, %v), want a fresh lookup's (%+v, nil)", got, err, ttIface(2))
	}
}

func TestRouteCacheKeepsFamiliesApart(t *testing.T) {
	var cache routeCache
	l := &ttLookup{}
	steps := []struct {
		ipv6 bool
		want boundIface
	}{{false, ttIface(1)}, {true, ttIface(2)}, {false, ttIface(1)}, {true, ttIface(2)}}
	for i, step := range steps {
		if got, err := cache.get(step.ipv6, ttT0, l.lookup); err != nil || got != step.want {
			t.Fatalf("get #%d (ipv6=%t) = (%+v, %v), want (%+v, nil)", i+1, step.ipv6, got, err, step.want)
		}
	}
	if want := []bool{false, true}; !slices.Equal(l.families, want) {
		t.Errorf("lookup families = %v, want %v", l.families, want)
	}
}

func TestRouteCacheInvalidateDropsOnlyThatFamily(t *testing.T) {
	var cache routeCache
	l := &ttLookup{}
	for _, ipv6 := range []bool{false, true} {
		if _, err := cache.get(ipv6, ttT0, l.lookup); err != nil {
			t.Fatalf("get(ipv6=%t) error = %v", ipv6, err)
		}
	}
	cache.invalidate(true)
	for _, ipv6 := range []bool{false, true} {
		if _, err := cache.get(ipv6, ttT0, l.lookup); err != nil {
			t.Fatalf("get(ipv6=%t) after invalidate(true) error = %v", ipv6, err)
		}
	}
	if want := []bool{false, true, true}; !slices.Equal(l.families, want) {
		t.Errorf("lookup families = %v, want %v: invalidate(true) must drop only the IPv6 entry", l.families, want)
	}
}

func TestNewDialerControlBindsCapturedDestination(t *testing.T) {
	l, b, raw := &ttLookup{}, &ttBinder{}, &ttRawConn{}
	if err := ttControl(t, l, b)("tcp4", "198.18.5.5:80", raw); err != nil {
		t.Fatalf("Control(tcp4, 198.18.5.5:80) error = %v, want nil", err)
	}
	if want := []ttBindCall{{conn: raw, iface: ttIface(1), ipv6: false}}; !slices.Equal(b.calls, want) {
		t.Errorf("bind calls = %+v, want %+v: once, on the RawConn handed to Control, with the looked-up IPv4 interface", b.calls, want)
	}
	if want := []bool{false}; !slices.Equal(l.families, want) {
		t.Errorf("lookup families = %v, want %v", l.families, want)
	}
}

func TestNewDialerControlBindsIPv6DestinationAsIPv6(t *testing.T) {
	l, b, raw := &ttLookup{}, &ttBinder{}, &ttRawConn{}
	if err := ttControl(t, l, b)("tcp6", "[2001:db8::1]:443", raw); err != nil {
		t.Fatalf("Control(tcp6, [2001:db8::1]:443) error = %v, want nil", err)
	}
	if want := []ttBindCall{{conn: raw, iface: ttIface(1), ipv6: true}}; !slices.Equal(b.calls, want) {
		t.Errorf("bind calls = %+v, want %+v: once, with the looked-up IPv6 interface", b.calls, want)
	}
	if want := []bool{true}; !slices.Equal(l.families, want) {
		t.Errorf("lookup families = %v, want %v", l.families, want)
	}
}

func TestNewDialerControlLeavesLoopbackUnbound(t *testing.T) {
	l, b := &ttLookup{}, &ttBinder{}
	if err := ttControl(t, l, b)("tcp4", "127.0.0.1:80", &ttRawConn{}); err != nil {
		t.Fatalf("Control(tcp4, 127.0.0.1:80) error = %v, want nil", err)
	}
	if len(b.calls) != 0 {
		t.Errorf("loopback dial: bind calls = %+v, want none", b.calls)
	}
	if len(l.families) != 0 {
		t.Errorf("loopback dial looked up the default route %d times, want 0", len(l.families))
	}
}

func TestNewDialerControlLeavesUncapturedDestinationUnbound(t *testing.T) {
	l, b := &ttLookup{}, &ttBinder{}
	d := newDialer(ttCaptureLab, l.lookup, b.bind, ttNow)
	if d == nil || d.Control == nil {
		t.Fatal("newDialer returned no Control hook")
	}
	if err := d.Control("tcp4", "203.0.113.9:80", &ttRawConn{}); err != nil {
		t.Fatalf("Control(tcp4, 203.0.113.9:80) error = %v, want nil", err)
	}
	if len(l.families) != 0 || len(b.calls) != 0 {
		t.Errorf("dial outside the capture %v: lookup families %v, bind calls %+v, want neither", ttCaptureLab, l.families, b.calls)
	}
}

func TestNewDialerControlReturnsLookupError(t *testing.T) {
	l, b := &ttLookup{err: ttErrLookup}, &ttBinder{}
	if err := ttControl(t, l, b)("tcp4", "198.18.5.5:80", &ttRawConn{}); !errors.Is(err, ttErrLookup) {
		t.Errorf("Control error = %v, want %v: a captured dial without a default route must fail", err, ttErrLookup)
	}
	if len(b.calls) != 0 {
		t.Errorf("bind calls = %d after a failed lookup, want 0", len(b.calls))
	}
}

func TestNewDialerControlReturnsBindError(t *testing.T) {
	l, b := &ttLookup{}, &ttBinder{err: ttErrBind}
	if err := ttControl(t, l, b)("tcp4", "198.18.5.5:80", &ttRawConn{}); !errors.Is(err, ttErrBind) {
		t.Errorf("Control error = %v, want %v", err, ttErrBind)
	}
}

func TestNewDialerControlReusesCachedLookup(t *testing.T) {
	l, b := &ttLookup{}, &ttBinder{}
	control := ttControl(t, l, b)
	for i := range 2 {
		if err := control("tcp4", "198.18.5.5:80", &ttRawConn{}); err != nil {
			t.Fatalf("Control #%d error = %v, want nil", i+1, err)
		}
	}
	if want := []bool{false}; !slices.Equal(l.families, want) {
		t.Errorf("lookup families = %v for two dials at one instant, want %v", l.families, want)
	}
}

func TestNewDialerControlInvalidatesCacheAfterBindError(t *testing.T) {
	l, b := &ttLookup{}, &ttBinder{err: ttErrBind}
	control := ttControl(t, l, b)
	failed, retried := &ttRawConn{}, &ttRawConn{}
	_ = control("tcp4", "198.18.5.5:80", failed)
	b.err = nil
	if err := control("tcp4", "198.18.5.5:80", retried); err != nil {
		t.Fatalf("Control after the failed bind error = %v, want nil", err)
	}
	if want := []bool{false, false}; !slices.Equal(l.families, want) {
		t.Errorf("lookup families = %v for a failed bind then a dial at the same instant, want %v: the failed bind must invalidate the cache", l.families, want)
	}
	if want := []ttBindCall{{conn: failed, iface: ttIface(1)}, {conn: retried, iface: ttIface(2)}}; !slices.Equal(b.calls, want) {
		t.Errorf("bind calls = %+v, want %+v: the retry binds the fresh lookup", b.calls, want)
	}
}
