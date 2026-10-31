//go:build darwin || linux

package tundev

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestAppleScriptQuotesBothPaths(t *testing.T) {
	got := appleScript("/Applications/Jump Way.app/x", `/tmp/a'b"c\d/s`)
	const want = `do shell script quoted form of "/Applications/Jump Way.app/x" & " tun-helper " & quoted form of "/tmp/a'b\"c\\d/s" with administrator privileges`
	if got != want {
		t.Errorf("appleScript =\n%s\nwant\n%s", got, want)
	}
}

func TestClassifyExit(t *testing.T) {
	// The generic rows' stderr holds no digits, so finding the code proves it is reported.
	for _, tc := range []struct {
		name   string
		goos   string
		code   int
		stderr string
		denied bool
	}{
		{"darwin user canceled", "darwin", 1, "0:156: execution error: User canceled. (-128)\n", true},
		{"darwin other failure", "darwin", 1, "tun-helper: create tun: operation not permitted\n", false},
		{"linux 126", "linux", 126, "Error executing command as another user: Request dismissed\n", true},
		{"linux 127", "linux", 127, "Error executing command as another user: Not authorized\n", true},
		{"linux 1", "linux", 1, "tun-helper: create tun: operation not permitted\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyExit(tc.goos, tc.code, tc.stderr)
			if tc.denied {
				if !errors.Is(err, ErrDenied) {
					t.Errorf("classifyExit = %v, want ErrDenied", err)
				}
				return
			}
			if err == nil || errors.Is(err, ErrDenied) || !strings.Contains(err.Error(), strconv.Itoa(tc.code)) {
				t.Errorf("classifyExit = %v, want a generic error containing exit code %d", err, tc.code)
			}
		})
	}
}
