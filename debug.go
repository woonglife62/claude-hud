package main

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

// ShowMessageBox displays a Windows MessageBox for critical errors.
// Use this sparingly - only for fatal startup errors.
func ShowMessageBox(title, message string) {
	procMessageBox := user32.NewProc("MessageBoxW")
	procMessageBox.Call(
		0,
		uintptr(unsafe.Pointer(utf16Ptr(message))),
		uintptr(unsafe.Pointer(utf16Ptr(title))),
		0x00000010, // MB_ICONERROR
	)
}

// ShowInfoBox displays an informational MessageBox.
func ShowInfoBox(title, message string) {
	procMessageBox := user32.NewProc("MessageBoxW")
	procMessageBox.Call(
		0,
		uintptr(unsafe.Pointer(utf16Ptr(message))),
		uintptr(unsafe.Pointer(utf16Ptr(title))),
		0x00000040, // MB_ICONINFORMATION
	)
}

// LogStructSizes logs the sizes of critical structs for debugging
func LogStructSizes() {
	Log("Struct sizes:")
	Log("  WNDCLASSEX:       %d bytes (expect 80)", unsafe.Sizeof(WNDCLASSEX{}))
	Log("  MSG:              %d bytes (expect 48)", unsafe.Sizeof(MSG{}))
	Log("  PAINTSTRUCT:      %d bytes (expect 72)", unsafe.Sizeof(PAINTSTRUCT{}))
	Log("  NOTIFYICONDATA:   %d bytes (expect 976)", unsafe.Sizeof(NOTIFYICONDATA{}))
	Log("  TRACKMOUSEEVENT:  %d bytes (expect 24)", unsafe.Sizeof(TRACKMOUSEEVENT{}))
	Log("  LOGFONT:          %d bytes (expect 92)", unsafe.Sizeof(LOGFONT{}))
	Log("  POINT:            %d bytes (expect 8)", unsafe.Sizeof(POINT{}))
	Log("  RECT:             %d bytes (expect 16)", unsafe.Sizeof(RECT{}))
	Log("  GUID:             %d bytes (expect 16)", unsafe.Sizeof(GUID{}))
	Log("  syscall.Handle:   %d bytes", unsafe.Sizeof(syscall.Handle(0)))
	Log("  uintptr:          %d bytes", unsafe.Sizeof(uintptr(0)))
}
