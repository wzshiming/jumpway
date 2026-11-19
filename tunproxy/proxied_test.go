package tunproxy

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/tun/stack"
)

func TestProxiedFullConeDialsOnceForEveryDestination(test *testing.T) {
	cone := ttNewConeConn()
	dialer := &ttDialer{dial: func(string) (net.Conn, error) { return cone, nil }}
	conn := ttProxiedSession(test, dialer, nil)
	first, second := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:443")
	ttWrite(test, conn, "first", first)
	ttWrite(test, conn, "second", second)
	if want := []ttDial{{network: "udp", address: first.String()}}; !slices.Equal(dialer.recorded(), want) {
		test.Fatalf("dials %+v, want one %+v for both destinations", dialer.recorded(), want)
	}
	got := map[string]string{}
	for range 2 {
		payload, source := ttRead(test, conn)
		if _, ok := source.(*net.UDPAddr); !ok {
			test.Fatalf("reply source %v is a %T, want a *net.UDPAddr", source, source)
		}
		got[source.String()] = payload
	}
	if want := map[string]string{first.String(): "first", second.String(): "second"}; !reflect.DeepEqual(got, want) {
		test.Fatalf("replies by source %v, want %v", got, want)
	}
}

func TestProxiedFullConeReturnsReadError(test *testing.T) {
	cone := ttNewConeConn()
	conn := ttProxiedSession(test, &ttDialer{dial: func(string) (net.Conn, error) { return cone, nil }}, nil)
	ttWrite(test, conn, "x", ttAddrPort("192.0.2.1:53"))
	ttRead(test, conn)
	cone.Close()
	if _, _, err := conn.ReadFrom(make([]byte, 8)); !errors.Is(err, net.ErrClosed) {
		test.Fatalf("read after the conn failed: %v, want its net.ErrClosed", err)
	}
}

func TestProxiedPlainConnsDialEachDestination(test *testing.T) {
	var mu sync.Mutex
	conns := map[string]*ttPlainConn{}
	dialer := &ttDialer{dial: func(address string) (net.Conn, error) {
		mu.Lock()
		defer mu.Unlock()
		conns[address] = ttNewPlainConn(nil)
		return conns[address], nil
	}}
	conn := ttProxiedSession(test, dialer, nil)
	first, second := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:443")
	ttWrite(test, conn, "first", first)
	ttWrite(test, conn, "second", second)
	ttWrite(test, conn, "again", first)
	got := map[string][]string{}
	for range 3 {
		payload, source := ttRead(test, conn)
		got[source.String()] = append(got[source.String()], payload)
	}
	if want := map[string][]string{first.String(): {"first", "again"}, second.String(): {"second"}}; !reflect.DeepEqual(got, want) {
		test.Fatalf("replies by source %v, want %v", got, want)
	}
	dials := dialer.recorded()
	slices.SortFunc(dials, func(a, b ttDial) int { return strings.Compare(a.address, b.address) })
	if want := []ttDial{{network: "udp", address: first.String()}, {network: "udp", address: second.String()}}; !slices.Equal(dials, want) {
		test.Fatalf("dials %+v, want one per destination %+v", dials, want)
	}
}

func TestProxiedConnectedUDPConnsDialEachDestination(test *testing.T) {
	dialer := &ttDialer{dial: func(address string) (net.Conn, error) { return net.Dial("udp", address) }}
	conn := ttProxiedSession(test, dialer, nil)
	first, second := ttUDPEcho(test), ttUDPEcho(test)
	ttWrite(test, conn, "first", first)
	ttWrite(test, conn, "second", second)
	got := map[string]string{}
	for range 2 {
		payload, source := ttRead(test, conn)
		got[source.String()] = payload
	}
	if want := map[string]string{first.String(): "first", second.String(): "second"}; !reflect.DeepEqual(got, want) {
		test.Fatalf("replies by source %v, want %v", got, want)
	}
	byAddress := func(a, b ttDial) int { return strings.Compare(a.address, b.address) }
	want := []ttDial{{network: "udp", address: first.String()}, {network: "udp", address: second.String()}}
	slices.SortFunc(want, byAddress)
	dials := dialer.recorded()
	slices.SortFunc(dials, byAddress)
	if !slices.Equal(dials, want) {
		test.Fatalf("dials %+v, want one per destination %+v", dials, want)
	}
}

func TestProxiedRedialsAfterReadFailure(test *testing.T) {
	failing := ttNewPlainConn(errors.New("tt conn gone"))
	dialer := &ttDialer{dial: func(string) (net.Conn, error) {
		select {
		case <-failing.closed:
			return ttNewPlainConn(nil), nil
		default:
			return failing, nil
		}
	}}
	conn := ttProxiedSession(test, dialer, nil)
	destination := ttAddrPort("192.0.2.1:53")
	ttWrite(test, conn, "first", destination)
	ttClosed(test, failing)
	ttWrite(test, conn, "second", destination)
	if payload, source := ttRead(test, conn); payload != "second" || source.String() != destination.String() {
		test.Fatalf("read %q from %v, want second from %v", payload, source, destination)
	}
	if got := len(dialer.recorded()); got != 2 {
		test.Fatalf("%d dials, want a second dial after the read failed", got)
	}
}

func TestProxiedDialErrorThenRedialsAfterRetryDelay(test *testing.T) {
	ttSeamDialRetryAfter(test, time.Millisecond)
	refused := errors.New("tt refused")
	dialer := &ttDialer{dial: func(string) (net.Conn, error) { return nil, refused }}
	conn := ttProxiedSession(test, dialer, nil)
	destination := ttAddrPort("192.0.2.1:53")
	if _, err := conn.WriteTo([]byte("first"), destination); !errors.Is(err, refused) {
		test.Fatalf("write with a failing dial: %v, want %v", err, refused)
	}
	dialer.setDial(func(string) (net.Conn, error) { return ttNewConeConn(), nil })
	deadline := time.Now().Add(ttWait)
	for {
		_, err := conn.WriteTo([]byte("second"), destination)
		if err == nil {
			break
		}
		if !errors.Is(err, refused) || time.Now().After(deadline) {
			test.Fatalf("write during the retry delay: %v, want %v until it passes", err, refused)
		}
	}
	if got := len(dialer.recorded()); got != 2 {
		test.Fatalf("%d dials, want the first datagram after the retry delay to dial again", got)
	}
	if payload, _ := ttRead(test, conn); payload != "second" {
		test.Fatalf("read %q, want second", payload)
	}
}

func TestProxiedFailedFirstDialHoldsOffTheWholeSession(test *testing.T) {
	ttSeamDialRetryAfter(test, time.Hour)
	refused := errors.New("tt refused")
	dialer := &ttDialer{dial: func(string) (net.Conn, error) { return nil, refused }}
	conn := ttProxiedSession(test, dialer, nil)
	first, second := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:53")
	for _, to := range []net.Addr{first, first, second} {
		if _, err := conn.WriteTo([]byte("x"), to); !errors.Is(err, refused) {
			test.Fatalf("write to %v: %v, want the remembered %v", to, err, refused)
		}
	}
	if want := []ttDial{{network: "udp", address: first.String()}}; !slices.Equal(dialer.recorded(), want) {
		test.Fatalf("dials %+v, want only %+v within the retry delay", dialer.recorded(), want)
	}
}

func TestProxiedFailedDialHoldsOffOnlyItsDestination(test *testing.T) {
	ttSeamDialRetryAfter(test, time.Hour)
	refused := errors.New("tt refused")
	first, failing, third := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:53"), ttAddrPort("192.0.2.3:53")
	dialer := &ttDialer{dial: func(address string) (net.Conn, error) {
		if address == failing.String() {
			return nil, refused
		}
		return ttNewPlainConn(nil), nil
	}}
	conn := ttProxiedSession(test, dialer, nil)
	ttWrite(test, conn, "first", first)
	ttRefused(test, conn, failing, refused)
	if _, err := conn.WriteTo([]byte("x"), failing); !errors.Is(err, refused) {
		test.Fatalf("write to %v: %v, want the remembered %v", failing, err, refused)
	}
	ttWrite(test, conn, "again", first)
	ttWrite(test, conn, "third", third)
	for range 3 {
		ttRead(test, conn)
	}
	want := []ttDial{{network: "udp", address: first.String()}, {network: "udp", address: failing.String()}, {network: "udp", address: third.String()}}
	dials := dialer.recorded()
	slices.SortFunc(dials, func(a, b ttDial) int { return strings.Compare(a.address, b.address) })
	if !slices.Equal(dials, want) {
		test.Fatalf("dials %+v, want %+v", dials, want)
	}
}

func TestProxiedForgetsDialFailuresPastTheirRetry(test *testing.T) {
	ttSeamDialRetryAfter(test, time.Millisecond)
	refused := errors.New("tt refused")
	reached := ttAddrPort("192.0.2.1:53")
	dialer := &ttDialer{dial: func(address string) (net.Conn, error) {
		if address == reached.String() {
			return ttNewPlainConn(nil), nil
		}
		return nil, refused
	}}
	conn := ttProxiedSession(test, dialer, nil)
	ttWrite(test, conn, "first", reached)
	ttRead(test, conn)
	for _, failing := range []string{"192.0.2.2:53", "192.0.2.3:53"} {
		ttWrite(test, conn, "x", ttAddrPort(failing))
	}
	session := conn.(*proxiedConn)
	deadline := time.Now().Add(ttWait)
	for {
		session.mu.Lock()
		remembered := len(session.failures)
		session.mu.Unlock()
		if remembered == 2 {
			break
		}
		if time.Now().After(deadline) {
			test.Fatalf("%d dial failures remembered after %v, want both", remembered, ttWait)
		}
		time.Sleep(time.Millisecond)
	}
	for {
		ttWrite(test, conn, "again", reached)
		ttRead(test, conn)
		session.mu.Lock()
		remembered := len(session.failures)
		session.mu.Unlock()
		if remembered == 0 {
			return
		}
		if time.Now().After(deadline) {
			test.Fatalf("%d dial failures still remembered %v past their retry", remembered, ttWait)
		}
	}
}

func TestProxiedIdleDestinationConnIsClosedAndRedialed(test *testing.T) {
	idle := 10 * time.Millisecond
	ttSeamRemoteIdleTimeout(test, idle)
	var mu sync.Mutex
	conns := map[string][]*ttPlainConn{}
	dialer := &ttDialer{dial: func(address string) (net.Conn, error) {
		mu.Lock()
		defer mu.Unlock()
		conns[address] = append(conns[address], ttNewPlainConn(nil))
		return conns[address][len(conns[address])-1], nil
	}}
	conn := ttProxiedSession(test, dialer, nil)
	idled, other := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:53")
	ttWrite(test, conn, "first", idled)
	ttRead(test, conn)
	mu.Lock()
	first := conns[idled.String()][0]
	mu.Unlock()
	deadline := time.After(ttWait)
	for expired := false; !expired; {
		select {
		case <-first.closed:
			expired = true
		case <-time.After(idle):
			ttWrite(test, conn, "tick", other)
			ttRead(test, conn)
		case <-deadline:
			test.Fatal("idle destination conn not closed")
		}
	}
	ttWrite(test, conn, "again", idled)
	if payload, source := ttRead(test, conn); payload != "again" || source.String() != idled.String() {
		test.Fatalf("read %q from %v, want again from %v", payload, source, idled)
	}
	mu.Lock()
	defer mu.Unlock()
	if got := len(conns[idled.String()]); got != 2 {
		test.Fatalf("%d dials to %v, want a new dial after its conn idled", got, idled)
	}
}

func TestProxiedCloseClosesConnsAndUnblocksRead(test *testing.T) {
	for name, cone := range map[string]bool{"full cone": true, "per destination": false} {
		test.Run(name, func(test *testing.T) {
			var mu sync.Mutex
			var closers []ttCloser
			dialer := &ttDialer{dial: func(string) (net.Conn, error) {
				mu.Lock()
				defer mu.Unlock()
				if cone {
					closers = append(closers, ttNewConeConn())
				} else {
					closers = append(closers, ttNewPlainConn(nil))
				}
				return closers[len(closers)-1].(net.Conn), nil
			}}
			conn := ttProxiedSession(test, dialer, nil)
			ttWrite(test, conn, "first", ttAddrPort("192.0.2.1:53"))
			ttWrite(test, conn, "second", ttAddrPort("192.0.2.2:53"))
			for range 2 {
				ttRead(test, conn)
			}
			read := make(chan error, 1)
			go func() {
				_, _, err := conn.ReadFrom(make([]byte, 8))
				read <- err
			}()
			conn.Close()
			select {
			case err := <-read:
				if err == nil {
					test.Fatal("read after Close succeeded, want an error")
				}
			case <-time.After(ttWait):
				test.Fatal("read still blocked after Close")
			}
			mu.Lock()
			defer mu.Unlock()
			for _, closer := range closers {
				ttClosed(test, closer)
			}
		})
	}
}

func TestProxiedReadWaitsForFirstWrite(test *testing.T) {
	type reply struct {
		payload string
		err     error
	}
	var got reply
	reading, returned := make(chan struct{}), make(chan struct{})
	returnedBeforeWrite := make(chan bool, 1)
	dialer := &ttDialer{dial: func(string) (net.Conn, error) {
		select {
		case <-returned:
			returnedBeforeWrite <- true
		default:
			returnedBeforeWrite <- false
		}
		return ttNewConeConn(), nil
	}}
	conn := ttProxiedSession(test, dialer, nil)
	go func() {
		buf := make([]byte, 8)
		close(reading)
		n, _, err := conn.ReadFrom(buf)
		got = reply{payload: string(buf[:n]), err: err}
		close(returned)
	}()
	<-reading
	ttWrite(test, conn, "first", ttAddrPort("192.0.2.1:53"))
	if <-returnedBeforeWrite {
		test.Fatalf("read returned %+v before the first write, want it blocked", got)
	}
	select {
	case <-returned:
		if got.err != nil || got.payload != "first" {
			test.Fatalf("read %+v, want the reply first that only the write makes", got)
		}
	case <-time.After(ttWait):
		test.Fatal("read still blocked after the first write")
	}
}

func TestProxiedCloseDuringDialEndsItAndClosesItsConn(test *testing.T) {
	dialing := make(chan struct{})
	late := ttNewPlainConn(nil)
	dialer := ttDialerFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		close(dialing)
		<-ctx.Done()
		return late, nil
	})
	conn := ttProxiedSession(test, dialer, nil)
	written := make(chan error, 1)
	go func() {
		_, err := conn.WriteTo([]byte("x"), ttAddrPort("192.0.2.1:53"))
		written <- err
	}()
	select {
	case <-dialing:
	case <-time.After(ttWait):
		test.Fatal("no dial")
	}
	conn.Close()
	select {
	case err := <-written:
		if !errors.Is(err, net.ErrClosed) {
			test.Fatalf("write whose dial outlived Close: %v, want net.ErrClosed", err)
		}
	case <-time.After(ttWait):
		test.Fatal("dial's ctx did not end on Close")
	}
	ttClosed(test, late)
}

func TestProxiedWriteAfterCloseIsErrClosed(test *testing.T) {
	for name, dial := range map[string]func(string) (net.Conn, error){
		"before any dial": nil,
		"full cone":       func(string) (net.Conn, error) { return ttNewConeConn(), nil },
		"per destination": func(string) (net.Conn, error) { return ttNewPlainConn(nil), nil },
	} {
		test.Run(name, func(test *testing.T) {
			conn := ttProxiedSession(test, &ttDialer{dial: dial}, nil)
			destination := ttAddrPort("192.0.2.1:53")
			if dial != nil {
				ttWrite(test, conn, "x", destination)
			}
			conn.Close()
			conn.Close()
			if _, err := conn.WriteTo([]byte("x"), destination); !errors.Is(err, net.ErrClosed) {
				test.Fatalf("write after Close: %v, want net.ErrClosed", err)
			}
		})
	}
}

func TestProxiedConcurrentWriteAndRead(test *testing.T) {
	for name, dial := range map[string]func(string) (net.Conn, error){
		"full cone":       func(string) (net.Conn, error) { return ttNewConeConn(), nil },
		"per destination": func(string) (net.Conn, error) { return ttNewPlainConn(nil), nil },
	} {
		test.Run(name, func(test *testing.T) {
			conn := ttProxiedSession(test, &ttDialer{dial: dial}, nil)
			destinations := []net.Addr{ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:53")}
			const count = 64
			written := make(chan error, 1)
			go func() {
				for i := range count {
					if _, err := conn.WriteTo([]byte("x"), destinations[i%len(destinations)]); err != nil {
						written <- err
						return
					}
				}
				written <- nil
			}()
			for range count {
				ttRead(test, conn)
			}
			if err := <-written; err != nil {
				test.Fatalf("write: %v", err)
			}
		})
	}
}

func TestProxiedPerDestinationReplyIsFromTheDestination(test *testing.T) {
	dialer := &ttDialer{dial: func(string) (net.Conn, error) { return ttRemoteAddrConn{ttNewPlainConn(nil)}, nil }}
	conn := ttProxiedSession(test, dialer, nil)
	destination := ttAddrPort("192.0.2.1:53")
	ttWrite(test, conn, "x", destination)
	_, source := ttRead(test, conn)
	if udp, ok := source.(*net.UDPAddr); !ok || udp.String() != destination.String() {
		test.Fatalf("reply source %v (%T), want the *net.UDPAddr %v", source, source, destination)
	}
}

func TestProxiedDialsWithSessionClient(test *testing.T) {
	for name, dial := range map[string]func(string) (net.Conn, error){
		"full cone":       func(string) (net.Conn, error) { return ttNewConeConn(), nil },
		"per destination": func(string) (net.Conn, error) { return ttNewPlainConn(nil), nil },
	} {
		test.Run(name, func(test *testing.T) {
			client := &net.UDPAddr{IP: net.IPv4(198, 18, 0, 2), Port: 50000}
			dialer := &ttDialer{dial: dial}
			conn := ttProxiedSession(test, dialer, client)
			ttWrite(test, conn, "x", ttAddrPort("192.0.2.1:53"))
			if want := []ttDial{{network: "udp", address: "192.0.2.1:53", client: client.String()}}; !slices.Equal(dialer.recorded(), want) {
				test.Fatalf("dials %+v, want %+v", dialer.recorded(), want)
			}
		})
	}
}

func TestProxiedSymmetricDialsEachDestinationThoughConnCanAddressAny(test *testing.T) {
	var mu sync.Mutex
	conns := map[string]ttPacketPlainConn{}
	dialer := &ttDialer{dial: func(address string) (net.Conn, error) {
		mu.Lock()
		defer mu.Unlock()
		conns[address] = ttPacketPlainConn{ttNewPlainConn(nil)}
		return conns[address], nil
	}}
	conn := ttProxiedOptionsSession(test, dialer, ProxiedOptions{Symmetric: true})
	first, second := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:443")
	ttWrite(test, conn, "first", first)
	ttWrite(test, conn, "second", second)
	ttWrite(test, conn, "again", first)
	got := map[string][]string{}
	for range 3 {
		payload, source := ttRead(test, conn)
		got[source.String()] = append(got[source.String()], payload)
	}
	if want := map[string][]string{first.String(): {"first", "again"}, second.String(): {"second"}}; !reflect.DeepEqual(got, want) {
		test.Fatalf("replies by source %v, want %v", got, want)
	}
	dials := dialer.recorded()
	slices.SortFunc(dials, func(a, b ttDial) int { return strings.Compare(a.address, b.address) })
	if want := []ttDial{{network: "udp", address: first.String()}, {network: "udp", address: second.String()}}; !slices.Equal(dials, want) {
		test.Fatalf("dials %+v, want one per destination %+v", dials, want)
	}
}

func TestProxiedSymmetricFailedFirstDialHoldsOffOnlyItsDestination(test *testing.T) {
	ttSeamDialRetryAfter(test, time.Hour)
	refused := errors.New("tt refused")
	failing, reached := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:53")
	dialer := &ttDialer{dial: func(address string) (net.Conn, error) {
		if address == failing.String() {
			return nil, refused
		}
		return ttNewPlainConn(nil), nil
	}}
	conn := ttProxiedOptionsSession(test, dialer, ProxiedOptions{Symmetric: true})
	ttRefused(test, conn, failing, refused)
	ttWrite(test, conn, "reached", reached)
	if payload, source := ttRead(test, conn); payload != "reached" || source.String() != reached.String() {
		test.Fatalf("read %q from %v, want reached from %v", payload, source, reached)
	}
}

func TestProxiedIdleOptionClosesIdleDestinationConn(test *testing.T) {
	ttSeamRemoteIdleTimeout(test, time.Hour)
	idle := 10 * time.Millisecond
	var mu sync.Mutex
	conns := map[string]*ttPlainConn{}
	dialer := &ttDialer{dial: func(address string) (net.Conn, error) {
		mu.Lock()
		defer mu.Unlock()
		conns[address] = ttNewPlainConn(nil)
		return conns[address], nil
	}}
	conn := ttProxiedOptionsSession(test, dialer, ProxiedOptions{Idle: idle})
	idled, other := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:53")
	ttWrite(test, conn, "first", idled)
	ttRead(test, conn)
	mu.Lock()
	first := conns[idled.String()]
	mu.Unlock()
	deadline := time.After(ttWait)
	for {
		select {
		case <-first.closed:
			return
		case <-time.After(idle):
			ttWrite(test, conn, "tick", other)
			ttRead(test, conn)
		case <-deadline:
			test.Fatalf("destination conn still open %v after it idled for the %v option", ttWait, idle)
		}
	}
}

// ttPerDestination are the sessions that dial each destination: by option, and by a first conn tied to its destination.
var ttPerDestination = map[string]ProxiedOptions{"symmetric": {Symmetric: true}, "first conn tied to its destination": {}}

func TestProxiedDialInFlightDoesNotDelayOtherDestinations(test *testing.T) {
	for name, opts := range ttPerDestination {
		test.Run(name, func(test *testing.T) {
			first, slow, other := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:53"), ttAddrPort("192.0.2.3:53")
			release := make(chan struct{})
			dialer := &ttDialer{dial: func(address string) (net.Conn, error) {
				if address == slow.String() {
					<-release
				}
				return ttNewPlainConn(nil), nil
			}}
			conn := ttProxiedOptionsSession(test, dialer, opts)
			test.Cleanup(func() { close(release) })
			ttWrite(test, conn, "first", first)
			ttRead(test, conn)
			ttWriteWithin(test, conn, "slow", slow)
			ttWriteWithin(test, conn, "other", other)
			if payload, source := ttRead(test, conn); payload != "other" || source.String() != other.String() {
				test.Fatalf("read %q from %v while %v dials, want other from %v", payload, source, slow, other)
			}
		})
	}
}

func TestProxiedDatagramsDuringDialAreSentInOrderOnceItSucceeds(test *testing.T) {
	for name, opts := range ttPerDestination {
		test.Run(name, func(test *testing.T) {
			first, slow := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:53")
			release := make(chan struct{})
			dialer := &ttDialer{dial: func(address string) (net.Conn, error) {
				if address == slow.String() {
					<-release
				}
				return ttNewPlainConn(nil), nil
			}}
			conn := ttProxiedOptionsSession(test, dialer, opts)
			ttWrite(test, conn, "first", first)
			ttRead(test, conn)
			var want []string
			for i := range pendingLimit + 6 {
				payload := strconv.Itoa(i)
				ttWriteWithin(test, conn, payload, slow)
				if i < pendingLimit {
					want = append(want, payload)
				}
			}
			close(release)
			var got []string
			for range want {
				payload, _ := ttRead(test, conn)
				got = append(got, payload)
			}
			if !slices.Equal(got, want) {
				test.Fatalf("read %v after the dial, want the first %d datagrams in order %v", got, pendingLimit, want)
			}
			ttWrite(test, conn, "after", slow)
			if payload, _ := ttRead(test, conn); payload != "after" {
				test.Fatalf("read %q after the queued datagrams, want after: those past %d are dropped", payload, pendingLimit)
			}
		})
	}
}

func TestProxiedFailedDialDropsTheDatagramsQueuedForIt(test *testing.T) {
	for name, opts := range ttPerDestination {
		test.Run(name, func(test *testing.T) {
			ttSeamDialRetryAfter(test, time.Millisecond)
			first, slow := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:53")
			refused := errors.New("tt refused")
			failing, redialed, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var mu sync.Mutex
			dials := 0
			dialer := &ttDialer{dial: func(address string) (net.Conn, error) {
				if address != slow.String() {
					return ttNewPlainConn(nil), nil
				}
				mu.Lock()
				dials++
				again := dials > 1
				mu.Unlock()
				if !again {
					<-failing
					return nil, refused
				}
				close(redialed)
				<-release
				return ttNewPlainConn(nil), nil
			}}
			conn := ttProxiedOptionsSession(test, dialer, opts)
			ttWrite(test, conn, "first", first)
			ttRead(test, conn)
			ttWriteWithin(test, conn, "dropped", slow)
			ttWriteWithin(test, conn, "dropped", slow)
			close(failing)
			deadline := time.After(ttWait)
			for again := false; !again; {
				select {
				case <-redialed:
					again = true
				case <-deadline:
					close(release)
					test.Fatal("no dial again after the failed one")
				case <-time.After(time.Millisecond):
					conn.WriteTo([]byte("after"), slow)
				}
			}
			close(release)
			if payload, _ := ttRead(test, conn); payload != "after" {
				test.Fatalf("read %q first after the failed dial, want after: the datagrams queued for it are dropped", payload)
			}
		})
	}
}

func TestProxiedCloseDuringDestinationDialEndsItAndClosesItsConn(test *testing.T) {
	for name, opts := range ttPerDestination {
		test.Run(name, func(test *testing.T) {
			first, slow := ttAddrPort("192.0.2.1:53"), ttAddrPort("192.0.2.2:53")
			dialing := make(chan struct{})
			late := ttNewPlainConn(nil)
			dialer := ttDialerFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
				if address != slow.String() {
					return ttNewPlainConn(nil), nil
				}
				close(dialing)
				<-ctx.Done()
				return late, nil
			})
			conn := ttProxiedOptionsSession(test, dialer, opts)
			ttWrite(test, conn, "first", first)
			ttRead(test, conn)
			ttWriteWithin(test, conn, "slow", slow)
			select {
			case <-dialing:
			case <-time.After(ttWait):
				test.Fatal("no dial")
			}
			conn.Close()
			ttClosed(test, late)
			if _, err := conn.WriteTo([]byte("x"), slow); !errors.Is(err, net.ErrClosed) {
				test.Fatalf("write after Close: %v, want net.ErrClosed", err)
			}
		})
	}
}

// ttRefused writes to to until the write answers want, the error of its failed dial, failing after ttWait.
func ttRefused(test *testing.T, conn net.PacketConn, to net.Addr, want error) {
	test.Helper()
	deadline := time.Now().Add(ttWait)
	for {
		_, err := conn.WriteTo([]byte("x"), to)
		if errors.Is(err, want) {
			return
		}
		if err != nil || time.Now().After(deadline) {
			test.Fatalf("write to %v: %v, want %v once its dial failed", to, err, want)
		}
		time.Sleep(time.Millisecond)
	}
}

// ttWriteWithin writes payload to to, failing when the write does not return within ttWait.
func ttWriteWithin(test *testing.T, conn net.PacketConn, payload string, to net.Addr) {
	test.Helper()
	written := make(chan error, 1)
	go func() {
		_, err := conn.WriteTo([]byte(payload), to)
		written <- err
	}()
	select {
	case err := <-written:
		if err != nil {
			test.Fatalf("write to %v: %v", to, err)
		}
	case <-time.After(ttWait):
		test.Fatalf("write to %v still blocked after %v", to, ttWait)
	}
}

// ttProxiedSession is a session socket of Proxied over dialer for client, closed when the test ends.
func ttProxiedSession(test *testing.T, dialer stack.Dialer, client net.Addr) net.PacketConn {
	test.Helper()
	ctx := context.Background()
	if client != nil {
		ctx = stack.WithClientAddr(ctx, client)
	}
	conn, err := Proxied(dialer, ProxiedOptions{}).ListenPacket(ctx, "udp4", ":0")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { conn.Close() })
	return conn
}

// ttProxiedOptionsSession is a session socket of Proxied over dialer with opts, closed when the test ends.
func ttProxiedOptionsSession(test *testing.T, dialer stack.Dialer, opts ProxiedOptions) net.PacketConn {
	test.Helper()
	conn, err := Proxied(dialer, opts).ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { conn.Close() })
	return conn
}

// ttPacketPlainConn is a ttPlainConn that is also a net.PacketConn, whose WriteTo and ReadFrom fail.
type ttPacketPlainConn struct{ *ttPlainConn }

func (conn ttPacketPlainConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	return 0, errors.New("tt: WriteTo on a conn of one destination")
}

func (conn ttPacketPlainConn) ReadFrom(p []byte) (int, net.Addr, error) {
	return 0, nil, errors.New("tt: ReadFrom on a conn of one destination")
}

// ttSeamDialRetryAfter sets dialRetryAfter for the sessions the test opens.
func ttSeamDialRetryAfter(test *testing.T, delay time.Duration) {
	previous := dialRetryAfter
	test.Cleanup(func() { dialRetryAfter = previous })
	dialRetryAfter = delay
}

// ttSeamRemoteIdleTimeout sets remoteIdleTimeout for the sessions the test opens.
func ttSeamRemoteIdleTimeout(test *testing.T, timeout time.Duration) {
	previous := remoteIdleTimeout
	test.Cleanup(func() { remoteIdleTimeout = previous })
	remoteIdleTimeout = timeout
}

func ttAddrPort(address string) *net.UDPAddr {
	return net.UDPAddrFromAddrPort(netip.MustParseAddrPort(address))
}

func ttWrite(test *testing.T, conn net.PacketConn, payload string, to net.Addr) {
	test.Helper()
	if _, err := conn.WriteTo([]byte(payload), to); err != nil {
		test.Fatalf("write to %v: %v", to, err)
	}
}

// ttRead reads one datagram from conn, failing after ttWait by closing it.
func ttRead(test *testing.T, conn net.PacketConn) (string, net.Addr) {
	test.Helper()
	type reply struct {
		payload string
		source  net.Addr
		err     error
	}
	read := make(chan reply, 1)
	go func() {
		buf := make([]byte, 64)
		n, source, err := conn.ReadFrom(buf)
		read <- reply{payload: string(buf[:n]), source: source, err: err}
	}()
	select {
	case got := <-read:
		if got.err != nil {
			test.Fatalf("read: %v", got.err)
		}
		return got.payload, got.source
	case <-time.After(ttWait):
		conn.Close()
		test.Fatal("no datagram to read")
		return "", nil
	}
}

// ttDialer dials each destination with its dial func and records the dials with the session's client.
type ttDialer struct {
	mu    sync.Mutex
	dials []ttDial
	dial  func(address string) (net.Conn, error)
}

func (dialer *ttDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer.mu.Lock()
	dialer.dials = append(dialer.dials, ttDial{network: network, address: address, client: ttClient(ctx)})
	dial := dialer.dial
	dialer.mu.Unlock()
	return dial(address)
}

func (dialer *ttDialer) setDial(dial func(address string) (net.Conn, error)) {
	dialer.mu.Lock()
	defer dialer.mu.Unlock()
	dialer.dial = dial
}

func (dialer *ttDialer) recorded() []ttDial {
	dialer.mu.Lock()
	defer dialer.mu.Unlock()
	return slices.Clone(dialer.dials)
}

// ttCloser is a fake conn whose closed channel is closed by Close.
type ttCloser interface{ isClosed() <-chan struct{} }

// ttClosed waits for closer to be closed.
func ttClosed(test *testing.T, closer ttCloser) {
	test.Helper()
	select {
	case <-closer.isClosed():
	case <-time.After(ttWait):
		test.Fatal("conn not closed")
	}
}

// ttAddr is an address of no standard type.
type ttAddr string

func (addr ttAddr) Network() string { return "tt" }
func (addr ttAddr) String() string  { return string(addr) }

// ttConeConn is a dialed conn that can address any destination, echoing each datagram from a ttAddr of it.
type ttConeConn struct {
	net.Conn
	replies chan ttDatagram
	closed  chan struct{}
	once    sync.Once
}

type ttDatagram struct {
	payload []byte
	source  net.Addr
}

func ttNewConeConn() *ttConeConn {
	return &ttConeConn{replies: make(chan ttDatagram, 8), closed: make(chan struct{})}
}

func (conn *ttConeConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	conn.replies <- ttDatagram{payload: slices.Clone(p), source: ttAddr(addr.String())}
	return len(p), nil
}

func (conn *ttConeConn) ReadFrom(p []byte) (int, net.Addr, error) {
	select {
	case datagram := <-conn.replies:
		return copy(p, datagram.payload), datagram.source, nil
	case <-conn.closed:
		return 0, nil, net.ErrClosed
	}
}

func (conn *ttConeConn) Close() error {
	conn.once.Do(func() { close(conn.closed) })
	return nil
}

func (conn *ttConeConn) LocalAddr() net.Addr       { return &net.UDPAddr{} }
func (conn *ttConeConn) isClosed() <-chan struct{} { return conn.closed }

// ttPlainConn is a dialed conn echoing each write, or failing every read with readErr.
type ttPlainConn struct {
	net.Conn
	readErr error
	replies chan []byte
	closed  chan struct{}
	once    sync.Once
}

func ttNewPlainConn(readErr error) *ttPlainConn {
	return &ttPlainConn{readErr: readErr, replies: make(chan []byte, 8), closed: make(chan struct{})}
}

func (conn *ttPlainConn) Write(p []byte) (int, error) {
	conn.replies <- slices.Clone(p)
	return len(p), nil
}

func (conn *ttPlainConn) Read(p []byte) (int, error) {
	if conn.readErr != nil {
		return 0, conn.readErr
	}
	select {
	case payload := <-conn.replies:
		return copy(p, payload), nil
	case <-conn.closed:
		return 0, net.ErrClosed
	}
}

func (conn *ttPlainConn) Close() error {
	conn.once.Do(func() { close(conn.closed) })
	return nil
}

func (conn *ttPlainConn) isClosed() <-chan struct{} { return conn.closed }

// ttRemoteAddrConn is a ttPlainConn reporting a remote address of no standard type.
type ttRemoteAddrConn struct{ *ttPlainConn }

func (conn ttRemoteAddrConn) RemoteAddr() net.Addr { return ttAddr("tt-remote") }

// ttDialerFunc is a stack.Dialer calling itself.
type ttDialerFunc func(ctx context.Context, network, address string) (net.Conn, error)

func (dial ttDialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dial(ctx, network, address)
}
