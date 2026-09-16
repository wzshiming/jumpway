//go:build darwin

package netproc

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"os/exec"
)

// Owners returns established TCP owners the caller may inspect, normally the same user without root.
func Owners(ctx context.Context) (map[netip.AddrPort]Process, error) {
	output, err := exec.CommandContext(ctx, "/usr/sbin/lsof", "-nP", "-iTCP", "-sTCP:ESTABLISHED", "-F", "pcn").Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if len(output) != 0 {
		return parseLsof(bytes.NewReader(output)), nil
	}
	var exitError *exec.ExitError
	if err != nil && !errors.As(err, &exitError) {
		return nil, err
	}
	return make(map[netip.AddrPort]Process), nil
}
