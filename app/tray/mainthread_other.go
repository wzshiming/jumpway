//go:build !darwin

package tray

func runOnMain(fn func()) error {
	fn()
	return nil
}
