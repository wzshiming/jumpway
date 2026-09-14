//go:build !darwin

package tray

func runOnMain(fn func()) { fn() }
