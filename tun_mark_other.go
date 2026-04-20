//go:build !linux

package jumpway

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
)

// NewMarkedDialer returns the default local dialer on non-Linux platforms.
// On Linux, this returns a dialer that sets SO_MARK to bypass TUN routing.
func NewMarkedDialer() bridge.Dialer {
	return local.LOCAL
}
