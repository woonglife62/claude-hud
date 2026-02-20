//go:build !windows

package platform

// PinToDesktop is a no-op on non-Windows platforms.
// TODO: Implement NSWindow level (macOS) and _NET_WM_WINDOW_TYPE_DESKTOP (Linux).
func PinToDesktop(hwnd uintptr) bool {
	return false
}

// UnpinFromDesktop is a no-op on non-Windows platforms.
func UnpinFromDesktop(hwnd uintptr) {}
