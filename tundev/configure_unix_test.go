//go:build darwin || linux

package tundev

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
)

// ttTools point at paths that do not exist, so even a stray exec cannot change the host.
var ttTools = tools{ifconfig: "/tt/sbin/ifconfig", route: "/tt/sbin/route", ip: "/tt/sbin/ip"}

var ttIfNames = map[string]string{"darwin": "utun7", "linux": "jumpway0"}

// ttErrFileExists is the failure a recorded command returns; "File exists" gets no special case.
var ttErrFileExists = errors.New("exit status 2: RTNETLINK answers: File exists")

// ttRecorder is a runFunc that records every argv and fails the one equal to fail.
type ttRecorder struct {
	fail  []string
	mu    sync.Mutex
	calls [][]string
}

func (r *ttRecorder) run(_ context.Context, argv []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, slices.Clone(argv))
	if r.fail != nil && slices.Equal(argv, r.fail) {
		return ttErrFileExists
	}
	return nil
}

func (r *ttRecorder) recorded() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

// ttWantSteps renders the contract's command table for goos with ttTools, routing every CaptureRoutes prefix.
func ttWantSteps(t *testing.T, goos string, cfg Config) []step {
	t.Helper()
	name := ttIfNames[goos]
	var a4, a6 netip.Prefix
	for _, a := range cfg.Addresses {
		if a.Addr().Is4() {
			a4 = a
		} else {
			a6 = a
		}
	}
	v4, v6 := ttSplit(CaptureRoutes(cfg))
	if (a4.IsValid() && len(v4) == 0) || (a6.IsValid() && len(v6) == 0) {
		t.Fatalf("CaptureRoutes(%v) left a configured family without routes", cfg.Addresses)
	}
	withLen := func(p netip.Prefix) string { return fmt.Sprintf("%s/%d", p.Addr(), p.Bits()) }
	var want []step
	add := func(isV6 bool, argv ...string) { want = append(want, step{argv: argv, v6: isV6}) }
	switch goos {
	case "darwin":
		if a4.IsValid() {
			add(false, ttTools.ifconfig, name, "inet", withLen(a4), a4.Addr().String(), "alias")
		}
		if a6.IsValid() {
			add(true, ttTools.ifconfig, name, "inet6", withLen(a6), "alias")
		}
		add(false, ttTools.ifconfig, name, "up")
		for _, p := range v4 {
			add(false, ttTools.route, "-n", "add", "-inet", p.String(), "-interface", name)
		}
		for _, p := range v6 {
			add(true, ttTools.route, "-n", "add", "-inet6", p.String(), "-interface", name)
		}
	case "linux":
		if a4.IsValid() {
			add(false, ttTools.ip, "-4", "addr", "add", withLen(a4), "dev", name)
		}
		if a6.IsValid() {
			add(true, ttTools.ip, "-6", "addr", "add", withLen(a6), "dev", name)
		}
		add(false, ttTools.ip, "link", "set", "dev", name, "up")
		for _, p := range v4 {
			add(false, ttTools.ip, "-4", "route", "add", p.String(), "dev", name)
		}
		for _, p := range v6 {
			add(true, ttTools.ip, "-6", "route", "add", p.String(), "dev", name)
		}
	default:
		t.Fatalf("no command table for %s", goos)
	}
	return want
}

func ttArgvs(ss []step) [][]string {
	argvs := make([][]string, len(ss))
	for i, s := range ss {
		argvs[i] = s.argv
	}
	return argvs
}

func ttLines(argvs [][]string) string {
	lines := make([]string, len(argvs))
	for i, argv := range argvs {
		lines[i] = fmt.Sprintf("  %3d %s", i, strings.Join(argv, " "))
	}
	return strings.Join(lines, "\n")
}

func ttCheckArgvs(t *testing.T, got, want [][]string) {
	t.Helper()
	if !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Errorf("argv sequence (%d):\n%s\nwant (%d):\n%s", len(got), ttLines(got), len(want), ttLines(want))
	}
}

func TestStepsFollowCommandTable(t *testing.T) {
	cfg := ttDefaultConfig()
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			want := ttWantSteps(t, goos, cfg)
			got, err := steps(goos, ttTools, ttIfNames[goos], cfg)
			if err != nil {
				t.Fatalf("steps: %v", err)
			}
			ttCheckArgvs(t, ttArgvs(got), ttArgvs(want))
			for i := range min(len(got), len(want)) {
				if got[i].v6 != want[i].v6 {
					t.Errorf("step %d %q: v6 = %t, want %t", i, got[i].argv, got[i].v6, want[i].v6)
				}
			}
		})
	}
}

func TestConfigureRunsEveryStepInOrder(t *testing.T) {
	ttGuardSystem(t)
	cfg := ttDefaultConfig()
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			rec := &ttRecorder{}
			warnings, err := configure(t.Context(), goos, ttTools, ttIfNames[goos], cfg, rec.run)
			if err != nil || len(warnings) != 0 {
				t.Errorf("configure = %q, %v; want no warnings and no error", warnings, err)
			}
			ttCheckArgvs(t, rec.recorded(), ttArgvs(ttWantSteps(t, goos, cfg)))
		})
	}
}

func TestConfigureStopsAtFailingIPv4Step(t *testing.T) {
	ttGuardSystem(t)
	cfg := ttDefaultConfig()
	for _, goos := range []string{"darwin", "linux"} {
		want := ttWantSteps(t, goos, cfg)
		// The default table runs the IPv4 address, the IPv6 address, link up, then the IPv4 routes.
		for _, tc := range []struct {
			name string
			fail int
		}{
			{"address", 0},
			{"route", 3},
		} {
			t.Run(goos+" IPv4 "+tc.name, func(t *testing.T) {
				rec := &ttRecorder{fail: want[tc.fail].argv}
				_, err := configure(t.Context(), goos, ttTools, ttIfNames[goos], cfg, rec.run)
				if err == nil || !strings.Contains(err.Error(), ttErrFileExists.Error()) {
					t.Errorf("configure = %v, want the failing step's error %q", err, ttErrFileExists)
				}
				ttCheckArgvs(t, rec.recorded(), ttArgvs(want[:tc.fail+1]))
			})
		}
	}
}

func TestConfigureWarnsAndSkipsIPv6AfterIPv6AddressFailure(t *testing.T) {
	ttGuardSystem(t)
	cfg := ttDefaultConfig()
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			want := ttWantSteps(t, goos, cfg)
			failing := want[1] // the IPv6 address
			rec := &ttRecorder{fail: failing.argv}
			warnings, err := configure(t.Context(), goos, ttTools, ttIfNames[goos], cfg, rec.run)
			if err != nil {
				t.Errorf("configure = %v, want nil: an IPv6 failure only warns while IPv4 is configured", err)
			}
			wantWarning := "ipv6: " + strings.Join(failing.argv, " ") + ": " + ttErrFileExists.Error()
			if !slices.Equal(warnings, []string{wantWarning}) {
				t.Errorf("warnings = %q, want [%q]", warnings, wantWarning)
			}
			var wantArgvs [][]string
			for i, s := range want {
				if i <= 1 || !s.v6 {
					wantArgvs = append(wantArgvs, s.argv)
				}
			}
			ttCheckArgvs(t, rec.recorded(), wantArgvs)
		})
	}
}

func TestConfigureFailsOnIPv6FailureWithoutIPv4(t *testing.T) {
	ttGuardSystem(t)
	cfg := ttDefaultConfig()
	cfg.Addresses = ttPrefixes("fd00:6a77:7761::1/64")
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			want := ttWantSteps(t, goos, cfg)
			rec := &ttRecorder{fail: want[0].argv}
			_, err := configure(t.Context(), goos, ttTools, ttIfNames[goos], cfg, rec.run)
			if err == nil || !strings.Contains(err.Error(), ttErrFileExists.Error()) {
				t.Errorf("configure = %v, want the IPv6 step's error %q with no IPv4 address configured", err, ttErrFileExists)
			}
			ttCheckArgvs(t, rec.recorded(), ttArgvs(want[:1]))
		})
	}
}

func TestStepsAndConfigureRejectUnknownGOOS(t *testing.T) {
	ttGuardSystem(t)
	cfg := ttDefaultConfig()
	for _, goos := range []string{"windows", "freebsd"} {
		t.Run(goos, func(t *testing.T) {
			if got, err := steps(goos, ttTools, "tun0", cfg); err == nil {
				t.Errorf("steps(%s) = nil error with\n%s\nwant an error", goos, ttLines(ttArgvs(got)))
			}
			rec := &ttRecorder{}
			if _, err := configure(t.Context(), goos, ttTools, "tun0", cfg, rec.run); err == nil {
				t.Errorf("configure(%s) = nil, want an error", goos)
			}
			if argvs := rec.recorded(); len(argvs) != 0 {
				t.Errorf("configure(%s) ran\n%s", goos, ttLines(argvs))
			}
		})
	}
}
