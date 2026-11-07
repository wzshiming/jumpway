//go:build e2e && (darwin || linux || windows)

package tray

import (
	"bufio"
	"context"
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/bridge/chain"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/jumpway/app/web/services/configs"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/tun"
	"github.com/wzshiming/tun/bypass"
)

const (
	ttE2ETarget4 = "1.1.1.1:443"
	ttE2ETarget6 = "[2606:4700:4700::1111]:443"
)

// ttE2EGate skips unless JUMPWAY_TUN_E2E=1, the test runs privileged and -test.run names TestE2E, so no run ever prompts to elevate.
func ttE2EGate(t *testing.T) {
	t.Helper()
	if os.Getenv("JUMPWAY_TUN_E2E") != "1" {
		t.Skip("changes the host's interfaces and routes; set JUMPWAY_TUN_E2E=1 to run")
	}
	if !ttE2EPrivileged() {
		t.Skip(ttE2ENotPrivileged)
	}
	if run := flag.Lookup("test.run"); run == nil || !strings.Contains(run.Value.String(), "TestE2E") {
		t.Skip("runs only when -test.run names the TestE2E tests")
	}
	// CreateTUN would write the interface name to the file this names.
	os.Unsetenv("WG_TUN_NAME_FILE")
}

func TestE2ETrayTUNCaptureThroughRule(t *testing.T) {
	ttE2EGate(t)
	previousProxy := setSystemProxy
	setSystemProxy = func(string) {}
	t.Cleanup(func() { setSystemProxy = previousProxy })
	previousNoProxy, previousOnlyProxy := chain.NoProxy, chain.OnlyProxy
	chain.NoProxy, chain.OnlyProxy = nil, nil
	t.Cleanup(func() { chain.NoProxy, chain.OnlyProxy = previousNoProxy, previousOnlyProxy })

	conf := &config.Config{Rules: []config.Rule{ttTUNRule("e2e", 0)}}
	// The empty default, so the TUN captures every public destination.
	noProxy := conf.NoProxy.GetList(t.TempDir())
	capture := tun.CaptureRoutes(tunConfig(noProxy))
	// Other captured traffic of the runner is forwarded on, bound outside the TUN, so only the test targets echo.
	hop := ttStartE2EHop(t, bypass.NewDialer(capture), ttE2ETarget4, ttE2ETarget6)
	conf.Rules[0].Forward.Way = []bridgeconfig.Node{{LB: []string{"http://" + hop.addr}}}

	// Registered after the hop, so the app and its TUN stop before the hop closes.
	app := newTestApp(t, conf)
	opener := &ttE2EOpener{}
	app.openTUN = opener.open
	watchdog := time.AfterFunc(3*time.Minute, func() {
		app.stop()
		opener.closeAll()
	})
	t.Cleanup(func() { watchdog.Stop() })
	ttStart(t, app)

	t.Run("E1 no capture before selection", func(t *testing.T) {
		ttE2EDialAndDrop("tcp", ttE2ETarget4, 2*time.Second)
		if n := hop.count(ttE2ETarget4); n != 0 {
			t.Fatalf("hop CONNECTs to %s before selecting a TUN = %d, want 0", ttE2ETarget4, n)
		}
	})

	app.selectTUN("e2e")
	status := ttE2EWaitStatus(t, app, opener, "the rule to run with a TUN", func(rule configs.RuleStatus) bool {
		return rule.Running && rule.Tun != ""
	})
	devices, errs := opener.recorded()
	if len(errs) != 0 || len(devices) != 1 {
		t.Fatalf("openTUN: %d devices, errors %v; want one device", len(devices), errs)
	}
	if dev := devices[0]; len(dev.Warnings) != 0 || dev.Interface != status.Tun {
		t.Fatalf("opened %q with warnings %q; want no warnings and status Tun %q", dev.Interface, dev.Warnings, status.Tun)
	}
	t.Logf("TUN %s serves rule e2e", status.Tun)
	ttE2ELogRoutes(t)

	t.Run("E2 tcp4 through the rule", func(t *testing.T) {
		ttE2EEchoViaHop(t, hop, ttE2ETarget4, "ping-v4")
	})
	t.Run("E2 tcp6 through the rule", func(t *testing.T) {
		ttE2EEchoViaHop(t, hop, ttE2ETarget6, "ping-v6")
	})

	t.Run("E3 bypass dialer escapes the TUN", func(t *testing.T) {
		before := hop.count(ttE2ETarget4)
		d := bypass.NewDialer(tun.CaptureRoutes(tunConfig(noProxy)))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err := d.DialContext(ctx, "tcp", ttE2ETarget4)
		if err == nil {
			conn.Close()
		}
		t.Logf("bypass dial %s: %v", ttE2ETarget4, err)
		if after := hop.count(ttE2ETarget4); after != before {
			t.Fatalf("hop CONNECTs to %s went %d -> %d on a bypass dial, want it unchanged", ttE2ETarget4, before, after)
		}
	})

	app.selectTUN("")
	ttE2EWaitStatus(t, app, opener, "the rule to run without a TUN", func(rule configs.RuleStatus) bool {
		return rule.Running && rule.Tun == ""
	})
	t.Run("E4 no capture after deselection", func(t *testing.T) {
		before := hop.count(ttE2ETarget4)
		ttE2EDialAndDrop("tcp", ttE2ETarget4, 2*time.Second)
		if after := hop.count(ttE2ETarget4); after != before {
			t.Fatalf("hop CONNECTs to %s went %d -> %d after deselecting the TUN, want it unchanged", ttE2ETarget4, before, after)
		}
	})
}

// ttE2EOpener is the real tun.Open, recording every device and error for the test goroutine to check.
type ttE2EOpener struct {
	mu      sync.Mutex
	devices []*tun.Device
	errs    []error
}

func (o *ttE2EOpener) open(ctx context.Context, cfg tun.Config) (*tun.Device, error) {
	dev, err := tun.Open(ctx, cfg)
	o.mu.Lock()
	defer o.mu.Unlock()
	if err != nil {
		o.errs = append(o.errs, err)
		return nil, err
	}
	o.devices = append(o.devices, dev)
	return dev, nil
}

func (o *ttE2EOpener) recorded() ([]*tun.Device, []error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.devices), slices.Clone(o.errs)
}

func (o *ttE2EOpener) closeAll() {
	devices, _ := o.recorded()
	for _, dev := range devices {
		dev.Close()
	}
}

// ttE2EWaitStatus polls for up to 60s until the e2e rule's status satisfies ready, failing early on an openTUN error.
func ttE2EWaitStatus(t *testing.T, app *App, opener *ttE2EOpener, what string, ready func(configs.RuleStatus) bool) configs.RuleStatus {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		status := app.Status()
		if len(status.Rules) == 1 && ready(status.Rules[0]) {
			return status.Rules[0]
		}
		if _, errs := opener.recorded(); len(errs) != 0 {
			t.Fatalf("openTUN failed: %v; status %+v", errs, status)
		}
		if time.Now().After(deadline) {
			t.Fatalf("waited 60s for %s; status %+v", what, status)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// ttE2EEchoViaHop dials address plainly, echoes payload, and checks the hop got exactly one CONNECT per successful dial to it.
func ttE2EEchoViaHop(t *testing.T, hop *ttE2EHop, address, payload string) {
	t.Helper()
	before := hop.count(address)
	var conn net.Conn
	attempts := 0
	// A fresh IPv6 address may stay tentative for a moment, which fails a dial at once.
	deadline := time.Now().Add(10 * time.Second)
	for {
		attempts++
		c, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err == nil {
			conn = c
			break
		}
		t.Logf("dial %s: %v", address, err)
		if time.Now().After(deadline) {
			t.Fatalf("dial %s through the TUN: %v", address, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.WriteString(conn, payload); err != nil {
		t.Fatalf("write to %s: %v", address, err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read the echo from %s: %v; hop targets %q", address, err, hop.recorded())
	}
	if string(got) != payload {
		t.Fatalf("echo from %s = %q, want %q", address, got, payload)
	}
	n := hop.count(address) - before
	if n < 1 || (attempts == 1 && n != 1) || n > attempts {
		t.Fatalf("hop CONNECTs to %s = %d after %d dial attempts, want exactly one per reaching dial; hop targets %q", address, n, attempts, hop.recorded())
	}
}

// ttE2EDialAndDrop dials address and closes any connection; its outcome does not matter.
func ttE2EDialAndDrop(network, address string, timeout time.Duration) {
	if conn, err := net.DialTimeout(network, address, timeout); err == nil {
		conn.Close()
	}
}

// ttE2ELogRoutes logs the kernel's routes to the test targets.
func ttE2ELogRoutes(t *testing.T) {
	t.Helper()
	commands := [][]string{{"ip", "route", "get", "1.1.1.1"}, {"ip", "-6", "route", "get", "2606:4700:4700::1111"}}
	switch runtime.GOOS {
	case "darwin":
		commands = [][]string{{"route", "-n", "get", "1.1.1.1"}, {"route", "-n", "get", "-inet6", "2606:4700:4700::1111"}}
	case "windows":
		commands = [][]string{{"route", "print"}}
	}
	for _, args := range commands {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		t.Logf("%s (err %v):\n%s", strings.Join(args, " "), err, out)
	}
}

// ttE2EHop is a loopback HTTP CONNECT proxy recording every target; it echoes the echo targets and forwards the rest through forward.
type ttE2EHop struct {
	addr    string
	echo    []string
	forward *net.Dialer

	mu      sync.Mutex
	targets []string
}

func ttStartE2EHop(t *testing.T, forward *net.Dialer, echo ...string) *ttE2EHop {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on 127.0.0.1: %v", err)
	}
	hop := &ttE2EHop{addr: ln.Addr().String(), echo: echo, forward: forward}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				hop.handle(conn)
			}()
		}
	}()
	return hop
}

func (h *ttE2EHop) handle(conn net.Conn) {
	reader := bufio.NewReader(conn)
	request, err := http.ReadRequest(reader)
	if err != nil || request.Method != http.MethodConnect {
		return
	}
	target := request.Host
	h.mu.Lock()
	h.targets = append(h.targets, target)
	h.mu.Unlock()
	if slices.Contains(h.echo, target) {
		io.WriteString(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
		io.Copy(conn, reader)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	upstream, err := h.forward.DialContext(ctx, "tcp", target)
	cancel()
	if err != nil {
		io.WriteString(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer upstream.Close()
	io.WriteString(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
	go func() {
		io.Copy(upstream, reader)
		upstream.Close()
	}()
	io.Copy(conn, upstream)
}

func (h *ttE2EHop) count(target string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, got := range h.targets {
		if got == target {
			n++
		}
	}
	return n
}

func (h *ttE2EHop) recorded() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.targets)
}
