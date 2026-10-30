//go:build windows

package bypass

import (
	"errors"
	"testing"
)

func TestNewDialerIsPlainOnWindows(t *testing.T) {
	d := NewDialer(nil)
	if d == nil {
		t.Fatal("NewDialer(nil) = nil, want a plain *net.Dialer")
	}
	if d.Control != nil {
		t.Error("NewDialer(nil).Control != nil, want nil: windows has no bypass binding")
	}
	if d.Resolver != nil {
		t.Error("NewDialer(nil).Resolver != nil, want nil: windows keeps the default resolver")
	}
}

func TestDefaultInterfaceIsUnsupportedOnWindows(t *testing.T) {
	for _, ipv6 := range []bool{false, true} {
		if iface, err := DefaultInterface(ipv6); !errors.Is(err, errors.ErrUnsupported) {
			t.Errorf("DefaultInterface(%t) = (%v, %v), want errors.ErrUnsupported", ipv6, iface, err)
		}
	}
}
