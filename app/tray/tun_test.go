package tray

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/wzshiming/jumpway/app/web/services/configs"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/tundev"
	"golang.zx2c4.com/wireguard/tun"
)

// ttWait bounds every wait for something that must happen.
const ttWait = 5 * time.Second

func TestReloadTUNRuleStatus(test *testing.T) {
	app, _ := ttNewTUNApp(test, &config.Config{Rules: []config.Rule{
		ttTUNRule(0, true),
		{Name: "plain", Listen: config.Listen{Host: "127.0.0.1"}},
	}}, &ttTUNOpener{})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	status := ttWaitStatus(test, app, func(status configs.Status) bool {
		return status.Rules[0].Running && status.Rules[0].Tun == "utun9" && status.Rules[1].Running
	})
	if _, _, err := net.SplitHostPort(status.Rules[0].Address); err != nil || status.Rules[0].Error != "" || status.Rules[1].Tun != "" {
		test.Fatalf("status = %+v, want the http listener's address beside tun utun9, and no tun on plain", status.Rules)
	}
	for index, want := range []string{"utun9", ""} {
		data, err := json.Marshal(status.Rules[index])
		if err != nil {
			test.Fatal(err)
		}
		var object map[string]any
		if err := json.Unmarshal(data, &object); err != nil {
			test.Fatal(err)
		}
		if value, ok := object["tun"]; ok != (want != "") || ok && value != want {
			test.Fatalf("rule %q JSON = %s, want tun %q", status.Rules[index].Name, data, want)
		}
	}
}

func TestReloadTUNOnlyRule(test *testing.T) {
	port, address := ttFreePort(test)
	app, _ := ttNewTUNApp(test, &config.Config{Rules: []config.Rule{ttTUNRule(port, false)}}, &ttTUNOpener{})
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	rule := ttWaitStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Running }).Rules[0]
	if rule.Address != "utun9" || rule.Tun != "utun9" || rule.Error != "" {
		test.Fatalf("tun-only status = %+v, want running on utun9", rule)
	}
	ttAssertRefused(test, address)
}

func TestReloadTUNOpenFailureIsOneShot(test *testing.T) {
	opener := &ttTUNOpener{err: errors.New("tt open failed")}
	app, _ := ttNewTUNApp(test, &config.Config{Rules: []config.Rule{ttTUNRule(0, false)}}, opener)
	for reload := int32(1); reload <= 2; reload++ {
		if err := app.Reload(); err != nil && !strings.Contains(err.Error(), i18n.TUNUnavailable()) {
			test.Fatalf("reload %d error = %v, want none or %q", reload, err, i18n.TUNUnavailable())
		}
		rule := ttWaitStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Error != "" }).Rules[0]
		if rule.Running || rule.Attempt != 1 || rule.Tun != "" ||
			!strings.Contains(rule.Error, i18n.TUNUnavailable()) || !strings.Contains(rule.Error, "tt open failed") {
			test.Fatalf("status after reload %d = %+v, want stopped at attempt 1 with the TUN error and its cause", reload, rule)
		}
		// A retry on the 100 ms listen backoff would land inside this window.
		time.Sleep(300 * time.Millisecond)
		if calls := opener.calls.Load(); calls != reload {
			test.Fatalf("open calls after reload %d = %d, want %d", reload, calls, reload)
		}
	}
}

func TestReloadTUNOpenErrorText(test *testing.T) {
	for _, scenario := range []struct {
		name           string
		err            error
		want, unwanted string
	}{
		{name: "denied", err: fmt.Errorf("tt helper: %w", tundev.ErrDenied), want: i18n.TUNDenied(), unwanted: i18n.TUNUnavailable()},
		{name: "unsupported", err: errors.ErrUnsupported, want: i18n.TUNUnavailable(), unwanted: i18n.TUNDenied()},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			app, _ := ttNewTUNApp(test, &config.Config{Rules: []config.Rule{ttTUNRule(0, false)}}, &ttTUNOpener{err: scenario.err})
			if err := app.Reload(); err != nil && !strings.Contains(err.Error(), scenario.want) {
				test.Fatalf("reload error = %v, want none or %q", err, scenario.want)
			}
			rule := ttWaitStatus(test, app, func(status configs.Status) bool { return status.Rules[0].Error != "" }).Rules[0]
			if rule.Running || !strings.Contains(rule.Error, scenario.want) || strings.Contains(rule.Error, scenario.unwanted) ||
				!strings.Contains(rule.Error, scenario.err.Error()) {
				test.Fatalf("status = %+v, want %q with the cause %q", rule, scenario.want, scenario.err)
			}
		})
	}
}

func TestReloadTUNDeviceKeptAcrossReload(test *testing.T) {
	conf := &config.Config{Rules: []config.Rule{ttTUNRule(0, false)}}
	opener := &ttTUNOpener{}
	app, _ := ttNewTUNApp(test, conf, opener)
	running := func(status configs.Status) bool { return status.Rules[0].Running && status.Rules[0].Tun == "utun9" }
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	ttWaitStatus(test, app, running)
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	if calls, closed := opener.calls.Load(), opener.device(0).isClosed(); calls != 1 || closed {
		test.Fatalf("unchanged reload: open calls = %d, first device closed = %v; want the device kept", calls, closed)
	}
	conf.NoProxy.List = []string{"10.0.0.0/8"}
	if err := app.store.Save(conf); err != nil {
		test.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		test.Fatal(err)
	}
	ttWaitUntil(test, "a second open after the no_proxy change", func() bool { return opener.calls.Load() >= 2 })
	ttWaitUntil(test, "the first device to close", opener.device(0).isClosed)
	ttWaitStatus(test, app, running)
	if calls := opener.calls.Load(); calls != 2 {
		test.Fatalf("open calls after the no_proxy change = %d, want 2", calls)
	}
}

func TestReloadTUNBypassDialer(test *testing.T) {
	bypassed := runtime.GOOS == "darwin" || runtime.GOOS == "linux"
	addresses := []string{"198.19.0.1/24", "fd00:1::1/64"}
	want := tundev.CaptureRoutes(tundev.Config{
		Addresses: []netip.Prefix{netip.MustParsePrefix(addresses[0]), netip.MustParsePrefix(addresses[1])},
		MTU:       config.DefaultTUNMTU,
		// The no_proxy list below without its hostname and its malformed CIDR.
		Bypass: []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/8"),
			netip.MustParsePrefix("192.0.2.7/32"),
			netip.MustParsePrefix("2001:db8::/32"),
			netip.MustParsePrefix("3fff::9/128"),
		},
	})
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		io.WriteString(writer, "target")
	}))
	test.Cleanup(target.Close)
	for _, scenario := range []struct {
		name string
		tun  bool
	}{
		{name: "tun", tun: true},
		{name: "no-tun"},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			rule := ttTUNRule(0, true)
			if scenario.tun {
				rule.Listen.Protocols[1].Address = addresses
			} else {
				rule.Listen.Protocols = rule.Listen.Protocols[:1]
			}
			app, recorder := ttNewTUNApp(test, &config.Config{
				NoProxy: config.NoProxy{List: []string{"10.0.0.0/8", "192.0.2.7", "2001:db8::/32", "3fff::9", "example.invalid", "10.1.2.3/33"}},
				Rules:   []config.Rule{rule},
			}, &ttTUNOpener{})
			if err := app.Reload(); err != nil {
				test.Fatal(err)
			}
			status := ttWaitStatus(test, app, func(status configs.Status) bool {
				return status.Rules[0].Running && (status.Rules[0].Tun == "utun9") == scenario.tun
			})
			proxy := &url.URL{Scheme: "http", Host: status.Rules[0].Address}
			if code, err := requestThrough(test, proxy, target.URL); err != nil || code != http.StatusOK {
				test.Fatalf("request through the rule = %d, %v; want 200", code, err)
			}
			captures, dialed := recorder.recorded()
			if !scenario.tun || !bypassed {
				if len(captures) != 0 {
					test.Fatalf("newBypassDialer captures = %v, want no call on %s", captures, runtime.GOOS)
				}
				return
			}
			if len(captures) != 1 || !slices.Equal(captures[0], want) {
				test.Fatalf("newBypassDialer captures = %v, want one call with %v", captures, want)
			}
			if !slices.Contains(dialed, target.Listener.Addr().String()) {
				test.Fatalf("bypass dialer connected to %q, want the proxied target %s", dialed, target.Listener.Addr())
			}
		})
	}
}

func TestReloadTUNOpenFailureStopsListener(test *testing.T) {
	port, address := ttFreePort(test)
	app, _ := ttNewTUNApp(test, &config.Config{Rules: []config.Rule{ttTUNRule(port, true)}}, &ttTUNOpener{err: errors.New("tt open failed")})
	if err := app.Reload(); err != nil && !strings.Contains(err.Error(), i18n.TUNUnavailable()) {
		test.Fatalf("reload error = %v, want none or %q", err, i18n.TUNUnavailable())
	}
	var done <-chan struct{}
	app.doSync(func() error {
		done = app.runtimes["tun"].done
		return nil
	})
	select {
	case <-done:
	case <-time.After(ttWait):
		test.Fatal("rule kept running after its TUN failed to open")
	}
	if rule := app.Status().Rules[0]; rule.Running || !strings.Contains(rule.Error, i18n.TUNUnavailable()) {
		test.Fatalf("status = %+v, want stopped with the TUN error", rule)
	}
	ttAssertRefused(test, address)
}

// ttTUNDevice is a TUN without traffic: Read blocks until Close, Write drops every packet.
type ttTUNDevice struct {
	name   string
	events chan tun.Event
	done   chan struct{}
	once   sync.Once
}

func (device *ttTUNDevice) File() *os.File                               { return nil }
func (device *ttTUNDevice) Write(bufs [][]byte, offset int) (int, error) { return len(bufs), nil }
func (device *ttTUNDevice) MTU() (int, error)                            { return 1500, nil }
func (device *ttTUNDevice) Name() (string, error)                        { return device.name, nil }
func (device *ttTUNDevice) Events() <-chan tun.Event                     { return device.events }
func (device *ttTUNDevice) BatchSize() int                               { return 1 }

func (device *ttTUNDevice) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	<-device.done
	return 0, os.ErrClosed
}

func (device *ttTUNDevice) Close() error {
	device.once.Do(func() {
		close(device.done)
		close(device.events)
	})
	return nil
}

func (device *ttTUNDevice) isClosed() bool {
	select {
	case <-device.done:
		return true
	default:
		return false
	}
}

// ttTUNOpener is a fake App.openTUN counting its calls: it fails with err, or opens a ttTUNDevice named utun9.
type ttTUNOpener struct {
	err     error
	calls   atomic.Int32
	mu      sync.Mutex
	devices []*ttTUNDevice
}

func (opener *ttTUNOpener) open(ctx context.Context, cfg tundev.Config) (*tundev.Device, error) {
	opener.calls.Add(1)
	if opener.err != nil {
		return nil, opener.err
	}
	device := &ttTUNDevice{name: "utun9", events: make(chan tun.Event), done: make(chan struct{})}
	opener.mu.Lock()
	opener.devices = append(opener.devices, device)
	opener.mu.Unlock()
	return &tundev.Device{Device: device, Interface: "utun9"}, nil
}

func (opener *ttTUNOpener) device(index int) *ttTUNDevice {
	opener.mu.Lock()
	defer opener.mu.Unlock()
	return opener.devices[index]
}

// ttBypassRecorder stands in for bypass.NewDialer, recording each capture set and every address its dialers connect to.
type ttBypassRecorder struct {
	mu       sync.Mutex
	captures [][]netip.Prefix
	dialed   []string
}

func (recorder *ttBypassRecorder) newDialer(capture []netip.Prefix) *net.Dialer {
	recorder.mu.Lock()
	recorder.captures = append(recorder.captures, slices.Clone(capture))
	recorder.mu.Unlock()
	return &net.Dialer{Control: func(network, address string, _ syscall.RawConn) error {
		recorder.mu.Lock()
		recorder.dialed = append(recorder.dialed, address)
		recorder.mu.Unlock()
		return nil
	}}
}

func (recorder *ttBypassRecorder) recorded() ([][]netip.Prefix, []string) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return slices.Clone(recorder.captures), slices.Clone(recorder.dialed)
}

// ttNewTUNApp is newTestApp with opener as its TUN and a ttBypassRecorder as newBypassDialer, so no test opens a real TUN or binds to an interface.
func ttNewTUNApp(test *testing.T, conf *config.Config, opener *ttTUNOpener) (*App, *ttBypassRecorder) {
	test.Helper()
	recorder := &ttBypassRecorder{}
	previous := newBypassDialer
	newBypassDialer = recorder.newDialer
	// Registered before newTestApp's cleanup, so it runs once the app has stopped.
	test.Cleanup(func() { newBypassDialer = previous })
	app := newTestApp(test, conf)
	app.openTUN = opener.open
	return app, recorder
}

// ttTUNRule is the proxy rule "tun" on 127.0.0.1:port with a default tun row, after an http row when withHTTP is set.
func ttTUNRule(port uint32, withHTTP bool) config.Rule {
	protocols := []config.Protocol{{Type: config.ProtocolTUN}}
	if withHTTP {
		protocols = []config.Protocol{{Type: config.ProtocolHTTP}, {Type: config.ProtocolTUN}}
	}
	return config.Rule{Name: "tun", Listen: config.Listen{Host: "127.0.0.1", Port: port, Protocols: protocols}}
}

// ttFreePort returns a loopback port nothing listens on, and its address.
func ttFreePort(test *testing.T) (uint32, string) {
	test.Helper()
	listener := occupyPort(test)
	if err := listener.Close(); err != nil {
		test.Fatal(err)
	}
	return uint32(listener.Addr().(*net.TCPAddr).Port), listener.Addr().String()
}

// ttAssertRefused fails when something accepts TCP connections at address.
func ttAssertRefused(test *testing.T, address string) {
	test.Helper()
	connection, err := net.DialTimeout("tcp", address, ttWait)
	if err == nil {
		connection.Close()
		test.Fatalf("%s accepted a connection, want it refused", address)
	}
}

func ttWaitStatus(test *testing.T, app *App, ready func(configs.Status) bool) configs.Status {
	test.Helper()
	deadline := time.Now().Add(ttWait)
	for {
		status := app.Status()
		if ready(status) {
			return status
		}
		if time.Now().After(deadline) {
			test.Fatalf("status did not converge: %+v", status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func ttWaitUntil(test *testing.T, what string, ready func() bool) {
	test.Helper()
	deadline := time.Now().Add(ttWait)
	for !ready() {
		if time.Now().After(deadline) {
			test.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
