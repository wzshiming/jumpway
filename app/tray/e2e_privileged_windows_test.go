//go:build e2e && windows

package tray

import "golang.org/x/sys/windows"

const ttE2ENotPrivileged = "needs an elevated process, as a non-elevated tun.Open would prompt for UAC"

func ttE2EPrivileged() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
