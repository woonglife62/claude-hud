//go:build !windows

package platform

// SetAutoStart is a no-op on non-Windows platforms.
// TODO: Implement LaunchAgent (macOS) and .desktop autostart (Linux).
func SetAutoStart(enable bool) error {
	return nil
}

// IsAutoStartEnabled returns false on non-Windows platforms.
// TODO: Implement LaunchAgent check (macOS) and .desktop check (Linux).
func IsAutoStartEnabled() bool {
	return false
}
