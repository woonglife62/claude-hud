package platform

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	advapi32            = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx    = advapi32.NewProc("RegOpenKeyExW")
	procRegSetValueEx   = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValue  = advapi32.NewProc("RegDeleteValueW")
	procRegCloseKey     = advapi32.NewProc("RegCloseKey")
	procRegQueryValueEx = advapi32.NewProc("RegQueryValueExW")
)

const (
	HKEY_CURRENT_USER = 0x80000001
	KEY_SET_VALUE     = 0x0002
	KEY_QUERY_VALUE   = 0x0001
	REG_SZ            = 1
)

const autoStartKeyPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
const autoStartValueName = "ClaudeHUD"

// SetAutoStart enables or disables auto-start on Windows login
func SetAutoStart(enable bool) error {
	keyPath, _ := syscall.UTF16PtrFromString(autoStartKeyPath)
	var hKey syscall.Handle
	ret, _, err := procRegOpenKeyEx.Call(
		HKEY_CURRENT_USER,
		uintptr(unsafe.Pointer(keyPath)),
		0,
		KEY_SET_VALUE,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return err
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	valueName, _ := syscall.UTF16PtrFromString(autoStartValueName)

	if enable {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		exePathUTF16, _ := syscall.UTF16FromString(exePath)
		ret, _, err = procRegSetValueEx.Call(
			uintptr(hKey),
			uintptr(unsafe.Pointer(valueName)),
			0,
			REG_SZ,
			uintptr(unsafe.Pointer(&exePathUTF16[0])),
			uintptr(len(exePathUTF16)*2),
		)
		if ret != 0 {
			return err
		}
	} else {
		procRegDeleteValue.Call(
			uintptr(hKey),
			uintptr(unsafe.Pointer(valueName)),
		)
	}
	return nil
}

// IsAutoStartEnabled checks if auto-start is configured
func IsAutoStartEnabled() bool {
	keyPath, _ := syscall.UTF16PtrFromString(autoStartKeyPath)
	var hKey syscall.Handle
	ret, _, _ := procRegOpenKeyEx.Call(
		HKEY_CURRENT_USER,
		uintptr(unsafe.Pointer(keyPath)),
		0,
		KEY_QUERY_VALUE,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return false
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	valueName, _ := syscall.UTF16PtrFromString(autoStartValueName)
	ret, _, _ = procRegQueryValueEx.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(valueName)),
		0, 0, 0, 0,
	)
	return ret == 0
}
