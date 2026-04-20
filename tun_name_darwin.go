package jumpway

// DefaultTUNName returns the default TUN device name for the current platform.
// On macOS, utun devices must be named "utun" (auto-assign) or "utunN".
func DefaultTUNName() string {
	return "utun"
}
