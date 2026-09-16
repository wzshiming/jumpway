//go:build !darwin && !linux && !windows

package netproc

import (
	"context"
	"errors"
	"net/netip"
)

// Owners is unsupported on this platform.
func Owners(ctx context.Context) (map[netip.AddrPort]Process, error) {
	return nil, errors.ErrUnsupported
}
