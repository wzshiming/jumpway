//go:build darwin || linux

package tundev

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"
	"testing"

	"golang.zx2c4.com/wireguard/tun"
)

// ttSystemGuard stands in for the seams that create devices or run commands, so no test can reach the host network.
type ttSystemGuard struct {
	mu    sync.Mutex
	calls []string
}

func ttGuardSystem(t *testing.T) *ttSystemGuard {
	t.Helper()
	g := &ttSystemGuard{}
	prevTUN, prevRun := createTUN, runCommand
	t.Cleanup(func() { createTUN, runCommand = prevTUN, prevRun })
	createTUN = func(name string, mtu int) (tun.Device, error) {
		g.record(fmt.Sprintf("createTUN(%q, %d)", name, mtu))
		return nil, errors.New("tt: creating a TUN is disabled in tests")
	}
	runCommand = func(_ context.Context, argv []string) error {
		g.record(fmt.Sprintf("runCommand(%q)", argv))
		return errors.New("tt: running commands is disabled in tests")
	}
	return g
}

func (g *ttSystemGuard) record(call string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, call)
}

func (g *ttSystemGuard) touched() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.calls)
}

const ttNameFile = "/nonexistent/tt-wg-tun-name"

// ttHelperRefuses checks Helper returns 2 without creating a device, running a command or unsetting WG_TUN_NAME_FILE.
func ttHelperRefuses(t *testing.T, args []string) {
	t.Helper()
	guard := ttGuardSystem(t)
	t.Setenv("WG_TUN_NAME_FILE", ttNameFile)
	if got := Helper(args); got != 2 {
		t.Errorf("Helper(%q) = %d, want 2", args, got)
	}
	if calls := guard.touched(); len(calls) != 0 {
		t.Errorf("Helper(%q) reached the system: %q", args, calls)
	}
	if v, ok := os.LookupEnv("WG_TUN_NAME_FILE"); !ok || v != ttNameFile {
		t.Errorf("Helper(%q) left WG_TUN_NAME_FILE = %q (set: %t), want it untouched", args, v, ok)
	}
}

func TestHelperRejectsUsage(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no argument", nil},
		{"relative path", []string{"rel"}},
		{"two paths", []string{"/a", "/b"}},
	} {
		t.Run(tc.name, func(t *testing.T) { ttHelperRefuses(t, tc.args) })
	}
}

func TestHelperRefusesNonRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("needs a non-root user")
	}
	ttHelperRefuses(t, []string{"/nonexistent/s"})
}
