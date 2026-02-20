//go:build !windows && !darwin && !linux

package platform

// SetAutoStart is a no-op on unsupported platforms.
func SetAutoStart(enable bool) error {
	return nil
}

// IsAutoStartEnabled returns false on unsupported platforms.
func IsAutoStartEnabled() bool {
	return false
}
