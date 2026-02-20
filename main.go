package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// ensureSingleInstance uses a Windows Named Mutex to prevent duplicate instances.
// Returns the mutex handle (must be kept alive) or 0 if another instance is running.
func ensureSingleInstance() syscall.Handle {
	name, _ := syscall.UTF16PtrFromString("Global\\ClaudeHUD_SingleInstance")
	h, _, err := kernel32.NewProc("CreateMutexW").Call(
		0,
		1, // bInitialOwner = TRUE
		uintptr(unsafe.Pointer(name)),
	)
	if h == 0 {
		return 0
	}
	// ERROR_ALREADY_EXISTS = 183
	if err == syscall.Errno(183) {
		// Another instance is already running
		kernel32.NewProc("CloseHandle").Call(h)
		return 0
	}
	return syscall.Handle(h)
}

func main() {
	// Lock the main goroutine to the OS thread (required for Win32 GUI)
	runtime.LockOSThread()

	// Prevent duplicate instances
	mutex := ensureSingleInstance()
	if mutex == 0 {
		ShowMessageBox("Claude HUD", "Claude HUD가 이미 실행 중입니다.")
		os.Exit(0)
	}
	defer kernel32.NewProc("CloseHandle").Call(uintptr(mutex))

	// Initialize diagnostic logging
	InitLog()
	defer CloseLog()

	Log("=== Claude HUD starting ===")
	Log("Go runtime: %s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	// Log struct sizes for debugging alignment issues
	LogStructSizes()

	// Recover from panics and log them
	defer func() {
		if r := recover(); r != nil {
			Log("PANIC: %v", r)
			ShowMessageBox("Claude HUD - Error", fmt.Sprintf("Panic: %v", r))
		}
	}()

	// Load or create config
	cfg := LoadConfig()
	Log("Config loaded: %dx%d at (%d,%d), opacity=%d, pin=%v",
		cfg.Width, cfg.Height, cfg.X, cfg.Y, cfg.Opacity, cfg.PinDesktop)

	// Load real data from Claude config files
	data := LoadRealData()
	Log("Real data loaded: %d sessions", len(data.Sessions))

	// Create the HUD window
	win := NewHUDWindow(cfg, data)
	if err := win.Create(); err != nil {
		Log("FATAL: Failed to create window: %v", err)
		ShowMessageBox("Claude HUD - Error",
			fmt.Sprintf("Failed to create window: %v\n\nCheck claude-hud.log for details.", err))
		os.Exit(1)
	}
	Log("Window created: hwnd=0x%X", win.hwnd)

	// Pin to desktop background if configured
	if cfg.PinDesktop {
		ok := PinToDesktop(win.hwnd)
		Log("PinToDesktop: success=%v", ok)
		if !ok {
			Log("WARNING: Desktop pin failed, window will use fallback Z-order")
		}
	}

	// Create system tray icon
	tray := NewTrayIcon(win.hwnd, win.hInstance)
	win.tray = tray
	Log("Tray icon created")

	// Show the HUD window
	win.Show()
	Log("Window shown, entering message loop")

	// Run the Windows message loop (blocks until WM_QUIT)
	RunMessageLoop()

	Log("=== Claude HUD exited normally ===")
}
