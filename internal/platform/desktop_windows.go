//go:build windows

package platform

import (
	"syscall"
	"unsafe"
)

// DLL procs used by desktop pinning
var (
	user32Desktop      = syscall.NewLazyDLL("user32.dll")
	procFindWindow     = user32Desktop.NewProc("FindWindowW")
	procFindWindowEx   = user32Desktop.NewProc("FindWindowExW")
	procEnumWindows    = user32Desktop.NewProc("EnumWindows")
	procSetParent      = user32Desktop.NewProc("SetParent")
	procSendMessage    = user32Desktop.NewProc("SendMessageW")
	procSetWindowPos   = user32Desktop.NewProc("SetWindowPos")
)

const (
	SWP_NOMOVE     = 0x0002
	SWP_NOSIZE     = 0x0001
	SWP_NOACTIVATE = 0x0010
	HWND_BOTTOM    = 1
)

// PinToDesktop pins the window behind desktop icons using the WorkerW technique.
// This mimics how DesktopCal and similar apps sit on the desktop background.
//
// The technique:
// 1. Find the "Progman" window (Program Manager)
// 2. Send it a special undocumented message (0x052C) to spawn a WorkerW window
// 3. Enumerate top-level windows to find the WorkerW that contains SHELLDLL_DefView
// 4. Get the sibling WorkerW window after it
// 5. Set our window as a child of that WorkerW
func PinToDesktop(hwnd syscall.Handle) bool {
	progmanClass, _ := syscall.UTF16PtrFromString("Progman")
	// Find Progman (the Program Manager window)
	progman, _, _ := procFindWindow.Call(
		uintptr(unsafe.Pointer(progmanClass)),
		0,
	)
	if progman == 0 {
		return false
	}

	// Send the magic undocumented message to make Progman spawn a WorkerW window.
	// This creates a layered window hierarchy for the desktop wallpaper.
	procSendMessage.Call(progman, 0x052C, 0xD, 0)
	procSendMessage.Call(progman, 0x052C, 0xD, 1)

	// Find the right WorkerW window.
	var workerW uintptr

	shellDefViewClass, _ := syscall.UTF16PtrFromString("SHELLDLL_DefView")
	workerWClass, _ := syscall.UTF16PtrFromString("WorkerW")

	enumCallback := syscall.NewCallback(func(topHwnd syscall.Handle, lParam uintptr) uintptr {
		// Check if this top-level window has a SHELLDLL_DefView child
		defView, _, _ := procFindWindowEx.Call(
			uintptr(topHwnd),
			0,
			uintptr(unsafe.Pointer(shellDefViewClass)),
			0,
		)
		if defView != 0 {
			// Found the window containing SHELLDLL_DefView.
			// The WorkerW we want is the next sibling after this window.
			h, _, _ := procFindWindowEx.Call(
				0,                // parent = desktop
				uintptr(topHwnd), // search after this window
				uintptr(unsafe.Pointer(workerWClass)),
				0,
			)
			workerW = h
			return 0 // Stop enumeration - we found it
		}
		return 1 // Continue enumeration
	})

	procEnumWindows.Call(enumCallback, 0)

	if workerW != 0 {
		// Set our window as a child of the WorkerW behind the desktop icons
		procSetParent.Call(uintptr(hwnd), workerW)
		return true
	}

	// Fallback: if WorkerW technique fails, place window at the bottom of Z-order
	procSetWindowPos.Call(
		uintptr(hwnd),
		HWND_BOTTOM,
		0, 0, 0, 0,
		SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE,
	)
	return false
}

// UnpinFromDesktop removes the window from the desktop background
// and restores it as a normal top-level window.
func UnpinFromDesktop(hwnd syscall.Handle) {
	// Setting parent to 0 (NULL) makes it a top-level window again
	procSetParent.Call(uintptr(hwnd), 0)
}
