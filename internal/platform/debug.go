package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var logFile *os.File

// InitLog creates a log file next to the executable for diagnostics.
// This is critical because -H windowsgui hides all stdout/stderr output.
func InitLog() {
	exePath, _ := os.Executable()
	logPath := filepath.Join(filepath.Dir(exePath), "claude-hud.log")
	var err error
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		// Last resort: try temp directory
		logPath = filepath.Join(os.TempDir(), "claude-hud.log")
		logFile, _ = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	}
}

// Log writes a timestamped message to the log file
func Log(format string, args ...interface{}) {
	if logFile == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05.000")
	fmt.Fprintf(logFile, "[%s] %s\n", timestamp, msg)
	logFile.Sync() // Flush immediately so crash doesn't lose logs
}

// CloseLog closes the log file
func CloseLog() {
	if logFile != nil {
		logFile.Close()
	}
}

// DLL procs used by platform package only
var (
	user32Plat   = syscall.NewLazyDLL("user32.dll")
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
