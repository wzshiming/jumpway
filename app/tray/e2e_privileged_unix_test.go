//go:build e2e && (darwin || linux)

package tray

import "os"

const ttE2ENotPrivileged = "needs root, as an unprivileged tun.Open would prompt to elevate"

func ttE2EPrivileged() bool {
	return os.Geteuid() == 0
}
