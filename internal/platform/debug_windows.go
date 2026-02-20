//go:build windows

package platform

import (
	"syscall"
	"unsafe"
)

// DLL procs used by platform package only
var (
	user32Plat     = syscall.NewLazyDLL("user32.dll")
	procMessageBox = user32Plat.NewProc("MessageBoxW")
)

// ShowMessageBox displays a Windows MessageBox for critical errors.
// Use this sparingly - only for fatal startup errors.
func ShowMessageBox(title, message string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	msgPtr, _ := syscall.UTF16PtrFromString(message)
	procMessageBox.Call(
		0,
		uintptr(unsafe.Pointer(msgPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		0x00000010, // MB_ICONERROR
	)
}

// ShowInfoBox displays an informational MessageBox.
func ShowInfoBox(title, message string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	msgPtr, _ := syscall.UTF16PtrFromString(message)
	procMessageBox.Call(
		0,
		uintptr(unsafe.Pointer(msgPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		0x00000040, // MB_ICONINFORMATION
	)
}
