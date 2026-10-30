package tundev

import (
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// step is one configuration command; v6 marks the IPv6 ones, which IPv4 can do without.
type step struct {
	argv []string
	v6   bool
}

// tools are the paths of the configuration commands.
type tools struct {
	ifconfig, route, ip string
}

func defaultTools(goos string) tools {
	switch goos {
	case "darwin":
		return tools{ifconfig: "/sbin/ifconfig", route: "/sbin/route"}
	case "linux":
		for _, p := range []string{"/sbin/ip", "/usr/sbin/ip", "/bin/ip", "/usr/bin/ip"} {
			if _, err := os.Stat(p); err == nil {
				return tools{ip: p}
			}
		}
		return tools{ip: "ip"}
	}
	return tools{}
}

func steps(goos string, t tools, name string, cfg Config) ([]step, error) {
	var a4, a6 netip.Prefix
	for _, p := range cfg.Addresses {
		if p.Addr().Is4() {
			a4 = p
		} else {
			a6 = p
		}
	}
	var ss []step
	add := func(v6 bool, argv ...string) { ss = append(ss, step{argv: argv, v6: v6}) }
	switch goos {
	case "darwin":
		if a4.IsValid() {
			add(false, t.ifconfig, name, "inet", a4.String(), a4.Addr().String(), "alias")
		}
		if a6.IsValid() {
			add(true, t.ifconfig, name, "inet6", a6.String(), "alias")
		}
		add(false, t.ifconfig, name, "up")
		for _, p := range CaptureRoutes(cfg) {
			if p.Addr().Is4() {
				add(false, t.route, "-n", "add", "-inet", p.String(), "-interface", name)
			} else {
				add(true, t.route, "-n", "add", "-inet6", p.String(), "-interface", name)
			}
		}
	case "linux":
		if a4.IsValid() {
			add(false, t.ip, "-4", "addr", "add", a4.String(), "dev", name)
		}
		if a6.IsValid() {
			add(true, t.ip, "-6", "addr", "add", a6.String(), "dev", name)
		}
		add(false, t.ip, "link", "set", "dev", name, "up")
		for _, p := range CaptureRoutes(cfg) {
			if p.Addr().Is4() {
				add(false, t.ip, "-4", "route", "add", p.String(), "dev", name)
			} else {
				add(true, t.ip, "-6", "route", "add", p.String(), "dev", name)
			}
		}
	default:
		return nil, fmt.Errorf("tundev: cannot configure a TUN on %s", goos)
	}
	return ss, nil
}

// runFunc runs argv without a shell; a failure's error carries the command's combined output.
type runFunc func(ctx context.Context, argv []string) error

var runCommand runFunc = func(ctx context.Context, argv []string) error {
	out, err := exec.CommandContext(ctx, argv[0], argv[1:]...).CombinedOutput()
	if out = bytes.TrimSpace(out); err != nil && len(out) > 0 {
		err = fmt.Errorf("%w: %s", err, out)
	}
	return err
}

func configure(ctx context.Context, goos string, t tools, name string, cfg Config, run runFunc) (warnings []string, err error) {
	ss, err := steps(goos, t, name, cfg)
	if err != nil {
		return nil, err
	}
	hasV4 := slices.ContainsFunc(cfg.Addresses, func(p netip.Prefix) bool { return p.Addr().Is4() })
	skipV6 := false
	for _, s := range ss {
		if s.v6 && skipV6 {
			continue
		}
		if err = run(ctx, s.argv); err == nil {
			continue
		}
		cmd := strings.Join(s.argv, " ")
		if !s.v6 || !hasV4 {
			return nil, fmt.Errorf("%s: %w", cmd, err)
		}
		warnings = append(warnings, fmt.Sprintf("ipv6: %s: %v", cmd, err))
		skipV6 = true
	}
	return warnings, nil
}
