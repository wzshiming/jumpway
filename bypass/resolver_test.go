//go:build darwin || linux

package bypass

import "testing"

func TestNewDialerResolvesNamesThroughItself(t *testing.T) {
	d := NewDialer(nil)
	if d == nil || d.Resolver == nil {
		t.Fatal("NewDialer(nil).Resolver = nil, want a pure-Go resolver dialing through the bypass dialer")
	}
	if !d.Resolver.PreferGo {
		t.Error("Resolver.PreferGo = false, want true so lookups go through Resolver.Dial")
	}
	if d.Resolver.Dial == nil {
		t.Error("Resolver.Dial = nil, want the dialer's own DialContext")
	}
}
