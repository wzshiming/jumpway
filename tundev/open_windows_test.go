//go:build windows

package tundev

import (
	"errors"
	"testing"
)

func TestOpenUnsupportedOnWindows(t *testing.T) {
	if _, err := Open(t.Context(), ttDefaultConfig()); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("Open = %v, want errors.ErrUnsupported", err)
	}
}

func TestHelperUnsupportedOnWindows(t *testing.T) {
	if got := Helper([]string{"/x"}); got != 2 {
		t.Errorf("Helper([/x]) = %d, want 2", got)
	}
}
