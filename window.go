package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	// user32
	procRegisterClassEx            = user32.NewProc("RegisterClassExW")
	procCreateWindowEx             = user32.NewProc("CreateWindowExW")
	procDefWindowProc              = user32.NewProc("DefWindowProcW")
	procGetMessage                 = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessage            = user32.NewProc("DispatchMessageW")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procUpdateWindow               = user32.NewProc("UpdateWindow")
	procIsWindowVisible            = user32.NewProc("IsWindowVisible")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
	procSetTimer                   = user32.NewProc("SetTimer")
	procKillTimer                  = user32.NewProc("KillTimer")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procLoadCursor                 = user32.NewProc("LoadCursorW")
	procGetCursorPos               = user32.NewProc("GetCursorPos")
	procReleaseCapture             = user32.NewProc("ReleaseCapture")
	procSendMessage                = user32.NewProc("SendMessageW")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procFindWindow                 = user32.NewProc("FindWindowW")
	procFindWindowEx               = user32.NewProc("FindWindowExW")
	procEnumWindows                = user32.NewProc("EnumWindows")
	procSetParent                  = user32.NewProc("SetParent")
	procTrackMouseEvent            = user32.NewProc("TrackMouseEvent")
	procPostMessage                = user32.NewProc("PostMessageW")
	procFillRect                   = user32.NewProc("FillRect")
	procDrawText                   = user32.NewProc("DrawTextW")
	procMonitorFromWindow          = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfo             = user32.NewProc("GetMonitorInfoW")

	// kernel32
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")

	// gdi32
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procRoundRect              = gdi32.NewProc("RoundRect")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procCreateFontIndirect     = gdi32.NewProc("CreateFontIndirectW")
	procTextOut                = gdi32.NewProc("TextOutW")
	procGetStockObject         = gdi32.NewProc("GetStockObject")
	procMoveToEx               = gdi32.NewProc("MoveToEx")
	procLineTo                 = gdi32.NewProc("LineTo")
	procSaveDC                 = gdi32.NewProc("SaveDC")
	procRestoreDC              = gdi32.NewProc("RestoreDC")
	procIntersectClipRect      = gdi32.NewProc("IntersectClipRect")
)

// Window style constants
const (
	WS_POPUP         = 0x80000000
	WS_THICKFRAME    = 0x00040000 // enables resize cursors at window edges
	WS_EX_LAYERED    = 0x00080000
	WS_EX_TOOLWINDOW = 0x00000080
	WS_EX_TOPMOST    = 0x00000008
	WS_EX_NOACTIVATE = 0x08000000

	CS_HREDRAW = 0x0002
	CS_VREDRAW = 0x0001

	SW_SHOW = 5
	SW_HIDE = 0

	WM_DESTROY       = 0x0002
	WM_PAINT         = 0x000F
	WM_CLOSE         = 0x0010
	WM_TIMER         = 0x0113
	WM_LBUTTONDOWN   = 0x0201
	WM_MOUSEMOVE     = 0x0200
	WM_MOUSEWHEEL    = 0x020A
	WM_MOUSELEAVE    = 0x02A3
	WM_NCLBUTTONDOWN = 0x00A1
	WM_ERASEBKGND    = 0x0014
	WM_MOVING        = 0x0216
	WM_USER          = 0x0400

	WM_NCHITTEST     = 0x0084
	WM_SIZE          = 0x0005
	WM_GETMINMAXINFO = 0x0024

	WM_TRAYICON = WM_USER + 1

	HTCLIENT      = 1
	HTCAPTION     = 2
	HTLEFT        = 10
	HTRIGHT       = 11
	HTTOP         = 12
	HTTOPLEFT     = 13
	HTTOPRIGHT    = 14
	HTBOTTOM      = 15
	HTBOTTOMLEFT  = 16
	HTBOTTOMRIGHT = 17

	RESIZE_BORDER = 6 // pixels - invisible resize grab zone at window edges

	SRCCOPY = 0x00CC0020

	TRANSPARENT_BK = 1 // renamed to avoid collision with windows constant

	IDC_ARROW = 32512

	LWA_ALPHA = 0x02

	TIMER_REPAINT       = 1 // repaint-only timer (no I/O), every 1s
	TIMER_REFRESH_PRESS = 2 // one-shot timer for refresh button press feedback
	TIMER_DATA_REFRESH  = 3 // triggers background data fetch

	WM_DATA_READY = WM_USER + 2 // posted by background goroutine when refresh completes

	SWP_NOSIZE     = 0x0001
	SWP_NOMOVE     = 0x0002
	SWP_NOACTIVATE = 0x0010

	HWND_BOTTOM = 1

	DT_LEFT         = 0x00000000
	DT_CENTER       = 0x00000001
	DT_RIGHT        = 0x00000002
	DT_SINGLELINE   = 0x00000020
	DT_NOPREFIX     = 0x00000800
	DT_END_ELLIPSIS = 0x00008000

	TME_LEAVE = 0x00000002

	PS_SOLID = 0

	SNAP_DISTANCE = 4  // pixels - snap when within this distance of screen edge
	SNAP_MARGIN   = -2 // pixels - gap between window edge and screen edge after snapping
)

// WNDCLASSEX structure
type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}

// MSG structure
type MSG struct {
	HWnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

// POINT structure
type POINT struct {
	X, Y int32
}

// RECT structure
type RECT struct {
	Left, Top, Right, Bottom int32
}

// MONITORINFO structure
type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}

// PAINTSTRUCT structure
type PAINTSTRUCT struct {
	HDC         syscall.Handle
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

// LOGFONT structure
type LOGFONT struct {
	LfHeight         int32
	LfWidth          int32
	LfEscapement     int32
	LfOrientation    int32
	LfWeight         int32
	LfItalic         byte
	LfUnderline      byte
	LfStrikeOut      byte
	LfCharSet        byte
	LfOutPrecision   byte
	LfClipPrecision  byte
	LfQuality        byte
	LfPitchAndFamily byte
	LfFaceName       [32]uint16
}

// MINMAXINFO structure for WM_GETMINMAXINFO
type MINMAXINFO struct {
	PtReserved     POINT
	PtMaxSize      POINT
	PtMaxPosition  POINT
	PtMinTrackSize POINT
	PtMaxTrackSize POINT
}

// TRACKMOUSEEVENT structure
type TRACKMOUSEEVENT struct {
	CbSize      uint32
	DwFlags     uint32
	HwndTrack   syscall.Handle
	DwHoverTime uint32
}

// COLORREF creates a Windows color value (0x00BBGGRR)
func COLORREF(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

// utf16Ptr converts a Go string to a UTF16 pointer
func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

// utf16Len returns the number of UTF-16 code units (excluding null terminator)
func utf16Len(s string) int {
	u, _ := syscall.UTF16FromString(s)
	if len(u) > 0 {
		return len(u) - 1 // exclude null terminator
	}
	return 0
}

// clampInt clamps a value between min and max
func clampInt(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}

// abs32 returns the absolute value of an int32
func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// Card layout constants shared between render and scroll calculation
const (
	cardBaseHeight     = 36
	cardAgentHeight    = 31
	cardSubAgentHeight = 28
	cardMessagesHeight = 18
	cardNoAgentsHeight = 16 // "직접 작업 중" line when expanded with no agents
	cardStrategyHeight = 18 // "⚡ strategy" line when active skill is present
	cardMinHeight      = 40
	cardGap            = 8
)

// sessionHitRect tracks the clickable area of a session card header
type sessionHitRect struct {
	sessionKey string // stable key (directory name) for session identity
	top        int32
	bottom     int32 // bottom of the header area (not the full card)
}

// HUDWindow manages the main HUD window
type HUDWindow struct {
	hwnd             syscall.Handle
	hInstance        syscall.Handle
	cfg              *Config
	data             *HUDData
	renderer         *Renderer
	hovered          bool
	scrollY          int
	visible          bool
	tray             *TrayIcon
	iconHandle       syscall.Handle   // custom icon handle for cleanup
	expandedSessions map[string]bool  // track which sessions are expanded (keyed by DirKey)
	sessionRects     []sessionHitRect // track session card positions for click detection
	refreshBtnRect   RECT             // clickable area for the refresh button
	refreshPressed   bool             // true during button press animation
	// Async data refresh
	pendingMu   sync.Mutex
	pending     *RefreshResult
	refreshBusy int32 // atomic: 1 if background refresh goroutine is running
}

var globalWindow *HUDWindow

// NewHUDWindow creates a new HUD window
func NewHUDWindow(cfg *Config, data *HUDData) *HUDWindow {
	w := &HUDWindow{
		cfg:              cfg,
		data:             data,
		visible:          true,
		expandedSessions: make(map[string]bool),
	}
	globalWindow = w
	return w
}

// Create registers and creates the window
func (w *HUDWindow) Create() error {
	hInstance, _, _ := procGetModuleHandle.Call(0)
	w.hInstance = syscall.Handle(hInstance)
	Log("  GetModuleHandle: 0x%X", hInstance)

	cursor, _, _ := procLoadCursor.Call(0, uintptr(IDC_ARROW))
	Log("  LoadCursor: 0x%X", cursor)

	className := utf16Ptr("ClaudeHUDClass")

	// Create custom Claude icon (purple circle with "C")
	icon := uintptr(createClaudeIcon())
	if icon == 0 {
		icon, _, _ = user32.NewProc("LoadIconW").Call(0, uintptr(32512)) // IDI_APPLICATION fallback
	}
	w.iconHandle = syscall.Handle(icon)
	Log("  LoadIcon: 0x%X", icon)

	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     w.hInstance,
		HIcon:         syscall.Handle(icon),
		HCursor:       syscall.Handle(cursor),
		LpszClassName: className,
		HIconSm:       syscall.Handle(icon),
	}

	ret, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	Log("  RegisterClassEx: ret=%d, err=%v", ret, err)
	if ret == 0 {
		// Class might already be registered from previous run - not fatal
		Log("  RegisterClassEx failed (may already exist): %v", err)
	}

	exStyle := uint32(WS_EX_LAYERED | WS_EX_TOOLWINDOW)
	if !w.cfg.PinDesktop {
		exStyle |= WS_EX_TOPMOST
	}
	style := uint32(WS_POPUP | WS_THICKFRAME)
	Log("  ExStyle: 0x%X, Style: 0x%X", exStyle, style)

	// Keep className pointer alive during CreateWindowEx
	windowTitle := utf16Ptr("Claude HUD")

	hwnd, _, err := procCreateWindowEx.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		uintptr(style),
		uintptr(w.cfg.X),
		uintptr(w.cfg.Y),
		uintptr(w.cfg.Width),
		uintptr(w.cfg.Height),
		0, 0,
		uintptr(w.hInstance),
		0,
	)
	w.hwnd = syscall.Handle(hwnd)
	Log("  CreateWindowEx: hwnd=0x%X, err=%v", hwnd, err)

	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx failed: %v", err)
	}

	// Set layered window alpha
	ret2, _, _ := procSetLayeredWindowAttributes.Call(
		hwnd, 0, uintptr(w.cfg.Opacity), LWA_ALPHA,
	)
	Log("  SetLayeredWindowAttributes: ret=%d, opacity=%d", ret2, w.cfg.Opacity)

	// Create renderer
	w.renderer = NewRenderer(w.hwnd, w.cfg.Width, w.cfg.Height)
	Log("  Renderer created")

	return nil
}

// Show displays the window
func (w *HUDWindow) Show() {
	procShowWindow.Call(uintptr(w.hwnd), SW_SHOW)
	procUpdateWindow.Call(uintptr(w.hwnd))
	w.visible = true

	// Repaint timer: redraws the window every 1s (no I/O, just GDI)
	procSetTimer.Call(uintptr(w.hwnd), TIMER_REPAINT, 1000, 0)
	// Data refresh timer: triggers background data fetch at configured interval
	procSetTimer.Call(uintptr(w.hwnd), TIMER_DATA_REFRESH, uintptr(w.cfg.RefreshMs), 0)
	// Kick off the first data fetch immediately
	w.triggerBackgroundRefresh()
}

// Hide hides the window
func (w *HUDWindow) Hide() {
	procShowWindow.Call(uintptr(w.hwnd), SW_HIDE)
	w.visible = false
}

// ToggleVisibility toggles the window visibility
func (w *HUDWindow) ToggleVisibility() {
	if w.visible {
		w.Hide()
	} else {
		procShowWindow.Call(uintptr(w.hwnd), SW_SHOW)
		w.visible = true
	}
}

// Destroy cleans up the window
func (w *HUDWindow) Destroy() {
	procKillTimer.Call(uintptr(w.hwnd), TIMER_REPAINT)
	procKillTimer.Call(uintptr(w.hwnd), TIMER_DATA_REFRESH)
	if w.renderer != nil {
		w.renderer.Cleanup()
	}
	// Destroy custom icon to prevent GDI handle leak
	if w.iconHandle != 0 {
		user32.NewProc("DestroyIcon").Call(uintptr(w.iconHandle))
	}
	SaveConfig(w.cfg)
}

// triggerBackgroundRefresh starts a goroutine to fetch data asynchronously.
// Only one refresh runs at a time; overlapping triggers are ignored.
func (w *HUDWindow) triggerBackgroundRefresh() {
	if !atomic.CompareAndSwapInt32(&w.refreshBusy, 0, 1) {
		return // already running
	}
	plan := w.data.Usage.Plan // read once (set at startup, never changes)
	go func() {
		defer atomic.StoreInt32(&w.refreshBusy, 0)
		result := FetchRefreshData(plan)
		w.pendingMu.Lock()
		w.pending = &result
		w.pendingMu.Unlock()
		procPostMessage.Call(uintptr(w.hwnd), WM_DATA_READY, 0, 0)
	}()
}

// Redraw triggers a repaint
func (w *HUDWindow) Redraw() {
	procInvalidateRect.Call(uintptr(w.hwnd), 0, 1)
}

// calcMaxScroll estimates the maximum scroll offset based on content height.
// Uses shared card layout constants to stay in sync with render.go's calcCardHeight.
// Accounts for expanded/collapsed state of each session card.
func (w *HUDWindow) calcMaxScroll() int {
	if w.data == nil || len(w.data.Sessions) == 0 {
		return 0
	}
	// Calculate total content height for sessions using shared constants
	totalSessionHeight := 0
	for _, s := range w.data.Sessions {
		if w.expandedSessions[s.DirKey] {
			// Expanded: full height with agents
			h := cardBaseHeight
			if s.ActiveSkill != nil {
				h += cardStrategyHeight
			}
			if len(s.Agents) == 0 {
				h += cardNoAgentsHeight
			}
			for _, a := range s.Agents {
				h += 16 // agent name line
				if a.Task != "" {
					h += 15 // task description line
				}
				h += len(a.SubAgents) * cardSubAgentHeight
			}
			if s.Messages > 0 {
				h += cardMessagesHeight
			}
			if h < cardMinHeight {
				h = cardMinHeight
			}
			totalSessionHeight += h + cardGap
		} else {
			// Collapsed: just header height
			totalSessionHeight += cardMinHeight + cardGap
		}
	}
	// Available height for sessions area (window height minus header/usage area)
	availableHeight := w.cfg.Height - 200
	if totalSessionHeight <= availableHeight {
		return 0
	}
	return totalSessionHeight - availableHeight
}

// RunMessageLoop runs the Windows message loop
func RunMessageLoop() {
	var msg MSG
	for {
		ret, _, _ := procGetMessage.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0,
		)
		if ret == 0 || int32(ret) == -1 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// wndProc is the window procedure callback
func wndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	w := globalWindow
	if w == nil {
		ret, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
		return ret
	}

	switch msg {
	case WM_NCHITTEST:
		// Enable resize by returning appropriate HT* values at window edges
		screenX := int32(int16(lParam & 0xFFFF))
		screenY := int32(int16((lParam >> 16) & 0xFFFF))

		var wr RECT
		procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&wr)))

		x := screenX - wr.Left
		y := screenY - wr.Top
		winW := wr.Right - wr.Left
		winH := wr.Bottom - wr.Top
		b := int32(RESIZE_BORDER)

		// Corners first (priority over edges)
		if x < b && y < b {
			return HTTOPLEFT
		}
		if x >= winW-b && y < b {
			return HTTOPRIGHT
		}
		if x < b && y >= winH-b {
			return HTBOTTOMLEFT
		}
		if x >= winW-b && y >= winH-b {
			return HTBOTTOMRIGHT
		}
		// Edges
		if x < b {
			return HTLEFT
		}
		if x >= winW-b {
			return HTRIGHT
		}
		if y < b {
			return HTTOP
		}
		if y >= winH-b {
			return HTBOTTOM
		}
		return HTCLIENT

	case WM_SIZE:
		newW := int(lParam & 0xFFFF)
		newH := int((lParam >> 16) & 0xFFFF)
		if newW > 0 && newH > 0 {
			w.cfg.Width = newW
			w.cfg.Height = newH
			if w.renderer != nil {
				w.renderer.width = newW
				w.renderer.height = newH
			}
		}
		return 0

	case WM_GETMINMAXINFO:
		mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
		mmi.PtMinTrackSize.X = 280
		mmi.PtMinTrackSize.Y = 300
		return 0

	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if w.renderer != nil {
			result := w.renderer.Render(syscall.Handle(hdc), w.data, w.cfg, w.scrollY, w.expandedSessions, w.refreshPressed)
			w.sessionRects = result.sessions
			w.refreshBtnRect = result.refreshBtn
		}
		procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		return 0

	case WM_ERASEBKGND:
		return 1 // Prevent flicker

	case WM_DATA_READY:
		// Background goroutine finished; apply pending data on the UI thread
		w.pendingMu.Lock()
		result := w.pending
		w.pending = nil
		w.pendingMu.Unlock()
		if result != nil {
			if result.Windows != nil {
				w.data.Usage.Windows = result.Windows
			}
			w.data.Sessions = result.Sessions
			// Auto-clamp scroll after data refresh (session count may change)
			maxScroll := w.calcMaxScroll()
			if w.scrollY > maxScroll {
				w.scrollY = maxScroll
			}
			w.Redraw()
		}
		return 0

	case WM_TIMER:
		if wParam == TIMER_REPAINT {
			w.Redraw()
		} else if wParam == TIMER_DATA_REFRESH {
			w.triggerBackgroundRefresh()
		} else if wParam == TIMER_REFRESH_PRESS {
			// Reset button press visual
			procKillTimer.Call(uintptr(w.hwnd), TIMER_REFRESH_PRESS)
			w.refreshPressed = false
			w.Redraw()
		}
		return 0

	case WM_MOVING:
		// lParam is a pointer to RECT with the proposed window position
		proposedRect := (*RECT)(unsafe.Pointer(lParam))

		// Get the monitor for the current window position
		// MONITOR_DEFAULTTONEAREST = 2
		hMonitor, _, _ := procMonitorFromWindow.Call(uintptr(w.hwnd), 2)

		var mi MONITORINFO
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		procGetMonitorInfo.Call(hMonitor, uintptr(unsafe.Pointer(&mi)))

		// Work area (excludes taskbar)
		workArea := mi.RcWork

		// Window dimensions
		winW := proposedRect.Right - proposedRect.Left
		winH := proposedRect.Bottom - proposedRect.Top

		// Snap to left edge (with margin)
		if abs32(proposedRect.Left-(workArea.Left+SNAP_MARGIN)) < SNAP_DISTANCE {
			proposedRect.Left = workArea.Left + SNAP_MARGIN
			proposedRect.Right = proposedRect.Left + winW
		}

		// Snap to right edge (with margin)
		if abs32(proposedRect.Right-(workArea.Right-SNAP_MARGIN)) < SNAP_DISTANCE {
			proposedRect.Right = workArea.Right - SNAP_MARGIN
			proposedRect.Left = proposedRect.Right - winW
		}

		// Snap to top edge (with margin)
		if abs32(proposedRect.Top-(workArea.Top+SNAP_MARGIN)) < SNAP_DISTANCE {
			proposedRect.Top = workArea.Top + SNAP_MARGIN
			proposedRect.Bottom = proposedRect.Top + winH
		}

		// Snap to bottom edge (with margin)
		if abs32(proposedRect.Bottom-(workArea.Bottom-SNAP_MARGIN)) < SNAP_DISTANCE {
			proposedRect.Bottom = workArea.Bottom - SNAP_MARGIN
			proposedRect.Top = proposedRect.Bottom - winH
		}

		return 1 // Return TRUE to indicate we modified the rect

	case WM_LBUTTONDOWN:
		// Get click position (sign-extend for multi-monitor correctness)
		clickX := int32(int16(lParam & 0xFFFF))
		clickY := int32(int16((lParam >> 16) & 0xFFFF))

		// Check if click is on the refresh button
		rb := w.refreshBtnRect
		if clickX >= rb.Left && clickX <= rb.Right && clickY >= rb.Top && clickY <= rb.Bottom {
			Log("Manual refresh triggered")
			w.refreshPressed = true
			w.Redraw()
			w.triggerBackgroundRefresh()
			// One-shot timer to reset button press visual after 100ms
			procSetTimer.Call(uintptr(w.hwnd), TIMER_REFRESH_PRESS, 100, 0)
			return 0
		}

		// Check if click is on a session card header
		handled := false
		for _, rect := range w.sessionRects {
			if clickY >= rect.top && clickY <= rect.bottom && clickX >= 12 && clickX <= int32(w.cfg.Width)-12 {
				// Toggle expanded state using stable directory key
				if w.expandedSessions[rect.sessionKey] {
					delete(w.expandedSessions, rect.sessionKey)
				} else {
					w.expandedSessions[rect.sessionKey] = true
				}
				w.Redraw()
				handled = true
				break
			}
		}

		if !handled {
			// Default: enable dragging by simulating caption click
			procReleaseCapture.Call()
			procSendMessage.Call(uintptr(hwnd), WM_NCLBUTTONDOWN, HTCAPTION, 0)
		}
		return 0

	case WM_MOUSEMOVE:
		if !w.hovered {
			w.hovered = true
			// Set higher opacity on hover
			newOpacity := clampInt(int(w.cfg.Opacity)+40, 0, 255)
			procSetLayeredWindowAttributes.Call(
				uintptr(hwnd), 0, uintptr(newOpacity), LWA_ALPHA,
			)
			// Track mouse leave
			tme := TRACKMOUSEEVENT{
				CbSize:    uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})),
				DwFlags:   TME_LEAVE,
				HwndTrack: hwnd,
			}
			procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
		}
		return 0

	case WM_MOUSELEAVE:
		w.hovered = false
		procSetLayeredWindowAttributes.Call(
			uintptr(hwnd), 0, uintptr(w.cfg.Opacity), LWA_ALPHA,
		)
		return 0

	case WM_MOUSEWHEEL:
		delta := int16(wParam >> 16)
		if delta > 0 {
			w.scrollY -= 30
		} else {
			w.scrollY += 30
		}
		if w.scrollY < 0 {
			w.scrollY = 0
		}
		// Clamp max scroll to prevent scrolling beyond content
		maxScroll := w.calcMaxScroll()
		if w.scrollY > maxScroll {
			w.scrollY = maxScroll
		}
		w.Redraw()
		return 0

	case WM_CLOSE:
		Log("WM_CLOSE received")
		// Save position and size before destroying
		var rect RECT
		ret, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
		if ret != 0 {
			w.cfg.X = int(rect.Left)
			w.cfg.Y = int(rect.Top)
			w.cfg.Width = int(rect.Right - rect.Left)
			w.cfg.Height = int(rect.Bottom - rect.Top)
		}
		w.Destroy()
		// Remove tray icon
		if w.tray != nil {
			w.tray.Remove()
		}
		// DestroyWindow triggers WM_DESTROY which calls PostQuitMessage
		user32.NewProc("DestroyWindow").Call(uintptr(hwnd))
		return 0

	case WM_DESTROY:
		Log("WM_DESTROY received")
		procPostQuitMessage.Call(0)
		return 0

	case WM_TRAYICON:
		handleTrayMessage(w, lParam)
		return 0
	}

	ret, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}
