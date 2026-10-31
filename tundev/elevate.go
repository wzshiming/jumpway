package tundev

import (
	"fmt"
	"strings"
)

var appleQuote = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

// appleScript runs exe's tun-helper on sock as root once the user approves the administrator prompt.
func appleScript(exe, sock string) string {
	return `do shell script quoted form of "` + appleQuote.Replace(exe) + `" & " tun-helper " & quoted form of "` + appleQuote.Replace(sock) + `" with administrator privileges`
}

// classifyExit explains why the launcher of the helper exited without the helper connecting.
func classifyExit(goos string, code int, stderr string) error {
	switch {
	case goos == "darwin" && strings.Contains(stderr, "(-128)"),
		goos == "linux" && (code == 126 || code == 127):
		return ErrDenied
	}
	return fmt.Errorf("tun helper exited %d: %s", code, strings.TrimSpace(stderr))
}
