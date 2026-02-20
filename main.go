package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"claude-hud/internal/config"
	"claude-hud/internal/data"
	"claude-hud/internal/i18n"
	"claude-hud/internal/platform"
	"claude-hud/internal/ui"
)

// ensureSingleInstance uses a Windows Named Mutex to prevent duplicate instances.
// Returns the mutex handle (must be kept alive) or 0 if another instance is running.
func ensureSingleInstance() syscall.Handle {
	name, _ := syscall.UTF16PtrFromString("Global\\ClaudeHUD_SingleInstance")
	h, _, err := ui.Kernel32.NewProc("CreateMutexW").Call(
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
		ui.Kernel32.NewProc("CloseHandle").Call(h)
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
		platform.ShowMessageBox("Claude HUD", "Claude HUD가 이미 실행 중입니다.")
		os.Exit(0)
	}
	defer ui.Kernel32.NewProc("CloseHandle").Call(uintptr(mutex))

	// Initialize diagnostic logging
	platform.InitLog()
	defer platform.CloseLog()

	platform.Log("=== Claude HUD starting ===")
	platform.Log("Go runtime: %s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	// Log struct sizes for debugging alignment issues
	for name, size := range ui.GetStructSizes() {
		platform.Log("  %s: %d bytes", name, size)
	}
	platform.Log("  syscall.Handle: %d bytes", unsafe.Sizeof(syscall.Handle(0)))
	platform.Log("  uintptr: %d bytes", unsafe.Sizeof(uintptr(0)))

	// Recover from panics and log them
	defer func() {
		if r := recover(); r != nil {
			platform.Log("PANIC: %v", r)
			platform.ShowMessageBox("Claude HUD - Error", fmt.Sprintf("Panic: %v", r))
		}
	}()

	// Enable per-monitor DPI awareness (Windows 10+, fallback to system-aware)
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4 (as uintptr cast to pointer-sized signed)
	const dpiAwarenessPerMonitorV2 = ^uintptr(3) // -4 as uintptr
	ret, _, _ := ui.ProcSetProcessDpiAwarenessCtx.Call(dpiAwarenessPerMonitorV2)
	if ret == 0 {
		// Fallback: try SetProcessDPIAware for older Windows
		ui.User32.NewProc("SetProcessDPIAware").Call()
	}
	platform.Log("DPI awareness initialized")

	// Load or create config
	cfg := config.LoadConfig()
	platform.Log("Config loaded: %dx%d at (%d,%d), opacity=%d, pin=%v",
		cfg.Width, cfg.Height, cfg.X, cfg.Y, cfg.Opacity, cfg.PinDesktop)

	// Initialize locale from config
	i18n.SetLanguage(cfg.Language)

	// Load real data from Claude config files
	hudData := data.LoadRealData()
	platform.Log("Real data loaded: %d sessions", len(hudData.Sessions))

	// Create the HUD window
	win := ui.NewHUDWindow(cfg, hudData)
	if err := win.Create(); err != nil {
		platform.Log("FATAL: Failed to create window: %v", err)
		platform.ShowMessageBox("Claude HUD - Error",
			fmt.Sprintf("Failed to create window: %v\n\nCheck claude-hud.log for details.", err))
		os.Exit(1)
	}
	platform.Log("Window created: hwnd=0x%X", win.Hwnd)

	// Pin to desktop background if configured
	if cfg.PinDesktop {
		ok := platform.PinToDesktop(win.Hwnd)
		platform.Log("PinToDesktop: success=%v", ok)
		if !ok {
			platform.Log("WARNING: Desktop pin failed, window will use fallback Z-order")
		}
	}

	// Create system tray icon
	tray := ui.NewTrayIcon(win.Hwnd, win.HInstance)
	win.Tray = tray
	platform.Log("Tray icon created")

	// Show the HUD window
	win.Show()
	platform.Log("Window shown, entering message loop")

	// Run the Windows message loop (blocks until WM_QUIT)
	ui.RunMessageLoop()

	platform.Log("=== Claude HUD exited normally ===")
}
