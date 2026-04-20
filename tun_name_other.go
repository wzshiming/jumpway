//go:build !darwin

package jumpway

// DefaultTUNName returns the default TUN device name for the current platform.
func DefaultTUNName() string {
	return "jumpway0"
}
