package ui

import (
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"claude-hud/internal/config"
	"claude-hud/internal/data"
	"claude-hud/internal/model"
	"claude-hud/internal/platform"
)

// HUDWindow manages the main HUD window
type HUDWindow struct {
	Hwnd             syscall.Handle
	HInstance        syscall.Handle
	Cfg              *config.Config
	Data             *model.HUDData
	Renderer         *Renderer
	Hovered          bool
	ScrollY          int
	Visible          bool
	Tray             *TrayIcon
	IconHandle       syscall.Handle
	ExpandedSessions map[string]bool
	SessionRects     []SessionHitRect
	RefreshBtnRect   RECT
	RefreshPressed   bool
	DpiScale         float64
	// Async data refresh
	PendingMu   sync.Mutex
	Pending     *model.RefreshResult
	RefreshBusy int32 // atomic: 1 if background refresh goroutine is running
}

// GlobalWindow is the singleton window instance used by wndProc callback
var GlobalWindow *HUDWindow

// NewHUDWindow creates a new HUD window
func NewHUDWindow(cfg *config.Config, d *model.HUDData) *HUDWindow {
	w := &HUDWindow{
		Cfg:              cfg,
		Data:             d,
		Visible:          true,
		ExpandedSessions: make(map[string]bool),
	}
	GlobalWindow = w
	return w
}

// Create registers and creates the window
func (w *HUDWindow) Create() error {
	hInstance, _, _ := ProcGetModuleHandle.Call(0)
	w.HInstance = syscall.Handle(hInstance)
	platform.Log("  GetModuleHandle: 0x%X", hInstance)

	cursor, _, _ := ProcLoadCursor.Call(0, uintptr(IDC_ARROW))
	platform.Log("  LoadCursor: 0x%X", cursor)

	className := Utf16Ptr("ClaudeHUDClass")

	// Create custom Claude icon (purple circle with "C")
	icon := uintptr(CreateClaudeIcon())
	if icon == 0 {
		icon, _, _ = User32.NewProc("LoadIconW").Call(0, uintptr(32512)) // IDI_APPLICATION fallback
	}
	w.IconHandle = syscall.Handle(icon)
	platform.Log("  LoadIcon: 0x%X", icon)

	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     w.HInstance,
		HIcon:         syscall.Handle(icon),
		HCursor:       syscall.Handle(cursor),
		LpszClassName: className,
		HIconSm:       syscall.Handle(icon),
	}

	ret, _, err := ProcRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	platform.Log("  RegisterClassEx: ret=%d, err=%v", ret, err)
	if ret == 0 {
		platform.Log("  RegisterClassEx failed (may already exist): %v", err)
	}

	exStyle := uint32(WS_EX_LAYERED | WS_EX_TOOLWINDOW)
	if !w.Cfg.PinDesktop {
		exStyle |= WS_EX_TOPMOST
	}
	style := uint32(WS_POPUP | WS_THICKFRAME)
	platform.Log("  ExStyle: 0x%X, Style: 0x%X", exStyle, style)

	windowTitle := Utf16Ptr("Claude HUD")

	hwnd, _, err := ProcCreateWindowEx.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		uintptr(style),
		uintptr(w.Cfg.X),
		uintptr(w.Cfg.Y),
		uintptr(w.Cfg.Width),
		uintptr(w.Cfg.Height),
		0, 0,
		uintptr(w.HInstance),
		0,
	)
	w.Hwnd = syscall.Handle(hwnd)
	platform.Log("  CreateWindowEx: hwnd=0x%X, err=%v", hwnd, err)

	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx failed: %v", err)
	}

	// Set layered window alpha
	ret2, _, _ := ProcSetLayeredWindowAttributes.Call(
		hwnd, 0, uintptr(w.Cfg.Opacity), LWA_ALPHA,
	)
	platform.Log("  SetLayeredWindowAttributes: ret=%d, opacity=%d", ret2, w.Cfg.Opacity)

	// Query DPI for the window and calculate scale
	dpiRaw, _, _ := ProcGetDpiForWindow.Call(uintptr(w.Hwnd))
	if dpiRaw == 0 {
		dpiRaw = 96
	}
	w.DpiScale = float64(dpiRaw) / 96.0
	platform.Log("  DPI: %d, scale: %.2f", dpiRaw, w.DpiScale)

	// Create renderer with DPI scale
	w.Renderer = NewRendererWithDPI(w.Hwnd, w.Cfg.Width, w.Cfg.Height, w.DpiScale)
	platform.Log("  Renderer created")

	// Apply theme from config
	if w.Cfg.ThemeMode == "light" {
		w.Renderer.RebuildCache(LightScheme)
	}

	// Register global hotkey Ctrl+Shift+H to toggle visibility
	ret3, _, _ := ProcRegisterHotKey.Call(uintptr(hwnd), HOTKEY_TOGGLE, MOD_CONTROL|MOD_SHIFT, 'H')
	platform.Log("  RegisterHotKey(Ctrl+Shift+H): ret=%d", ret3)

	return nil
}

// Show displays the window
func (w *HUDWindow) Show() {
	ProcShowWindow.Call(uintptr(w.Hwnd), SW_SHOW)
	ProcUpdateWindow.Call(uintptr(w.Hwnd))
	w.Visible = true

	ProcSetTimer.Call(uintptr(w.Hwnd), TIMER_REPAINT, 1000, 0)
	ProcSetTimer.Call(uintptr(w.Hwnd), TIMER_DATA_REFRESH, uintptr(w.Cfg.RefreshMs), 0)
	w.TriggerBackgroundRefresh()
}

// Hide hides the window
func (w *HUDWindow) Hide() {
	ProcShowWindow.Call(uintptr(w.Hwnd), SW_HIDE)
	w.Visible = false
}

// ToggleVisibility toggles the window visibility
func (w *HUDWindow) ToggleVisibility() {
	if w.Visible {
		w.Hide()
	} else {
		ProcShowWindow.Call(uintptr(w.Hwnd), SW_SHOW)
		w.Visible = true
	}
}

// Destroy cleans up the window
func (w *HUDWindow) Destroy() {
	ProcKillTimer.Call(uintptr(w.Hwnd), TIMER_REPAINT)
	ProcKillTimer.Call(uintptr(w.Hwnd), TIMER_DATA_REFRESH)
	ProcUnregisterHotKey.Call(uintptr(w.Hwnd), HOTKEY_TOGGLE)
	if w.Renderer != nil {
		w.Renderer.Cleanup()
	}
	if w.IconHandle != 0 {
		User32.NewProc("DestroyIcon").Call(uintptr(w.IconHandle))
	}
	config.SaveConfig(w.Cfg)
}

// TriggerBackgroundRefresh starts a goroutine to fetch data asynchronously.
func (w *HUDWindow) TriggerBackgroundRefresh() {
	if !atomic.CompareAndSwapInt32(&w.RefreshBusy, 0, 1) {
		return
	}
	plan := w.Data.Usage.Plan
	go func() {
		defer atomic.StoreInt32(&w.RefreshBusy, 0)
		result := data.FetchRefreshData(plan)
		w.PendingMu.Lock()
		w.Pending = &result
		w.PendingMu.Unlock()
		ProcPostMessage.Call(uintptr(w.Hwnd), WM_DATA_READY, 0, 0)
	}()
}

// Redraw triggers a repaint
func (w *HUDWindow) Redraw() {
	ProcInvalidateRect.Call(uintptr(w.Hwnd), 0, 1)
}

// CalcMaxScroll estimates the maximum scroll offset based on content height.
func (w *HUDWindow) CalcMaxScroll() int {
	if w.Data == nil || len(w.Data.Sessions) == 0 {
		return 0
	}
	totalSessionHeight := 0
	for _, s := range w.Data.Sessions {
		if w.ExpandedSessions[s.DirKey] {
			h := CardBaseHeight
			if s.ActiveSkill != nil {
				h += CardStrategyHeight
			}
			if len(s.Agents) == 0 {
				h += CardNoAgentsHeight
			}
			for _, a := range s.Agents {
				h += 16
				if a.Task != "" {
					h += 15
				}
				h += len(a.SubAgents) * CardSubAgentHeight
			}
			if s.Messages > 0 {
				h += CardMessagesHeight
			}
			if h < CardMinHeight {
				h = CardMinHeight
			}
			totalSessionHeight += h + CardGap
		} else {
			totalSessionHeight += CardMinHeight + CardGap
		}
	}
	availableHeight := w.Cfg.Height - 200
	if totalSessionHeight <= availableHeight {
		return 0
	}
	return totalSessionHeight - availableHeight
}

// RunMessageLoop runs the Windows message loop
func RunMessageLoop() {
	var msg MSG
	for {
		ret, _, _ := ProcGetMessage.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0,
		)
		if ret == 0 || int32(ret) == -1 {
			break
		}
		ProcTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		ProcDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// wndProc is the window procedure callback
func wndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	w := GlobalWindow
	if w == nil {
		ret, _, _ := ProcDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
		return ret
	}

	switch msg {
	case WM_NCHITTEST:
		screenX := int32(int16(lParam & 0xFFFF))
		screenY := int32(int16((lParam >> 16) & 0xFFFF))

		var wr RECT
		ProcGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&wr)))

		x := screenX - wr.Left
		y := screenY - wr.Top
		winW := wr.Right - wr.Left
		winH := wr.Bottom - wr.Top
		b := int32(RESIZE_BORDER)

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
			w.Cfg.Width = newW
			w.Cfg.Height = newH
			if w.Renderer != nil {
				w.Renderer.Width = newW
				w.Renderer.Height = newH
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
		hdc, _, _ := ProcBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if w.Renderer != nil {
			result := w.Renderer.Render(syscall.Handle(hdc), w.Data, w.Cfg, w.ScrollY, w.ExpandedSessions, w.RefreshPressed)
			w.SessionRects = result.Sessions
			w.RefreshBtnRect = result.RefreshBtn
		}
		ProcEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		return 0

	case WM_ERASEBKGND:
		return 1

	case WM_DATA_READY:
		w.PendingMu.Lock()
		result := w.Pending
		w.Pending = nil
		w.PendingMu.Unlock()
		if result != nil {
			if result.Windows != nil {
				w.Data.Usage.Windows = result.Windows
				data.UpdateTrend(&w.Data.Usage)
			}
			w.Data.Sessions = result.Sessions
			if result.ClosedSessions != nil {
				w.Data.ClosedSessions = result.ClosedSessions
			}
			if result.ModelBreakdown != nil {
				w.Data.Usage.ModelBreakdown = result.ModelBreakdown
			}
			CheckAndNotify(w.Tray, w.Data, w.Cfg)
			maxScroll := w.CalcMaxScroll()
			if w.ScrollY > maxScroll {
				w.ScrollY = maxScroll
			}
			w.Redraw()
		}
		return 0

	case WM_TIMER:
		if wParam == TIMER_REPAINT {
			w.Redraw()
		} else if wParam == TIMER_DATA_REFRESH {
			w.TriggerBackgroundRefresh()
		} else if wParam == TIMER_REFRESH_PRESS {
			ProcKillTimer.Call(uintptr(w.Hwnd), TIMER_REFRESH_PRESS)
			w.RefreshPressed = false
			w.Redraw()
		}
		return 0

	case WM_MOVING:
		proposedRect := (*RECT)(unsafe.Pointer(lParam))

		hMonitor, _, _ := ProcMonitorFromWindow.Call(uintptr(w.Hwnd), 2)

		var mi MONITORINFO
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		ProcGetMonitorInfo.Call(hMonitor, uintptr(unsafe.Pointer(&mi)))

		workArea := mi.RcWork

		winW := proposedRect.Right - proposedRect.Left
		winH := proposedRect.Bottom - proposedRect.Top

		if Abs32(proposedRect.Left-(workArea.Left+SNAP_MARGIN)) < SNAP_DISTANCE {
			proposedRect.Left = workArea.Left + SNAP_MARGIN
			proposedRect.Right = proposedRect.Left + winW
		}

		if Abs32(proposedRect.Right-(workArea.Right-SNAP_MARGIN)) < SNAP_DISTANCE {
			proposedRect.Right = workArea.Right - SNAP_MARGIN
			proposedRect.Left = proposedRect.Right - winW
		}

		if Abs32(proposedRect.Top-(workArea.Top+SNAP_MARGIN)) < SNAP_DISTANCE {
			proposedRect.Top = workArea.Top + SNAP_MARGIN
			proposedRect.Bottom = proposedRect.Top + winH
		}

		if Abs32(proposedRect.Bottom-(workArea.Bottom-SNAP_MARGIN)) < SNAP_DISTANCE {
			proposedRect.Bottom = workArea.Bottom - SNAP_MARGIN
			proposedRect.Top = proposedRect.Bottom - winH
		}

		return 1

	case WM_LBUTTONDOWN:
		clickX := int32(int16(lParam & 0xFFFF))
		clickY := int32(int16((lParam >> 16) & 0xFFFF))

		rb := w.RefreshBtnRect
		if clickX >= rb.Left && clickX <= rb.Right && clickY >= rb.Top && clickY <= rb.Bottom {
			platform.Log("Manual refresh triggered")
			w.RefreshPressed = true
			w.Redraw()
			w.TriggerBackgroundRefresh()
			ProcSetTimer.Call(uintptr(w.Hwnd), TIMER_REFRESH_PRESS, 100, 0)
			return 0
		}

		handled := false
		for _, rect := range w.SessionRects {
			if clickY >= rect.Top && clickY <= rect.Bottom && clickX >= 12 && clickX <= int32(w.Cfg.Width)-12 {
				if w.ExpandedSessions[rect.SessionKey] {
					delete(w.ExpandedSessions, rect.SessionKey)
				} else {
					w.ExpandedSessions[rect.SessionKey] = true
				}
				w.Redraw()
				handled = true
				break
			}
		}

		if !handled {
			ProcReleaseCapture.Call()
			ProcSendMessage.Call(uintptr(hwnd), WM_NCLBUTTONDOWN, HTCAPTION, 0)
		}
		return 0

	case WM_MOUSEMOVE:
		if !w.Hovered {
			w.Hovered = true
			newOpacity := ClampInt(int(w.Cfg.Opacity)+40, 0, 255)
			ProcSetLayeredWindowAttributes.Call(
				uintptr(hwnd), 0, uintptr(newOpacity), LWA_ALPHA,
			)
			tme := TRACKMOUSEEVENT{
				CbSize:    uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})),
				DwFlags:   TME_LEAVE,
				HwndTrack: hwnd,
			}
			ProcTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
		}
		return 0

	case WM_MOUSELEAVE:
		w.Hovered = false
		ProcSetLayeredWindowAttributes.Call(
			uintptr(hwnd), 0, uintptr(w.Cfg.Opacity), LWA_ALPHA,
		)
		return 0

	case WM_MOUSEWHEEL:
		delta := int16(wParam >> 16)
		if delta > 0 {
			w.ScrollY -= 30
		} else {
			w.ScrollY += 30
		}
		if w.ScrollY < 0 {
			w.ScrollY = 0
		}
		maxScroll := w.CalcMaxScroll()
		if w.ScrollY > maxScroll {
			w.ScrollY = maxScroll
		}
		w.Redraw()
		return 0

	case WM_CLOSE:
		platform.Log("WM_CLOSE received")
		var rect RECT
		ret, _, _ := ProcGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
		if ret != 0 {
			w.Cfg.X = int(rect.Left)
			w.Cfg.Y = int(rect.Top)
			w.Cfg.Width = int(rect.Right - rect.Left)
			w.Cfg.Height = int(rect.Bottom - rect.Top)
		}
		w.Destroy()
		if w.Tray != nil {
			w.Tray.Remove()
		}
		User32.NewProc("DestroyWindow").Call(uintptr(hwnd))
		return 0

	case WM_DPICHANGED:
		newDPI := uint32(wParam & 0xFFFF)
		if newDPI == 0 {
			newDPI = 96
		}
		w.DpiScale = float64(newDPI) / 96.0
		platform.Log("WM_DPICHANGED: dpi=%d, scale=%.2f", newDPI, w.DpiScale)
		// Move/resize window to the suggested rect
		suggested := (*RECT)(unsafe.Pointer(lParam))
		ProcSetWindowPos.Call(
			uintptr(hwnd),
			0,
			uintptr(suggested.Left),
			uintptr(suggested.Top),
			uintptr(suggested.Right-suggested.Left),
			uintptr(suggested.Bottom-suggested.Top),
			SWP_NOACTIVATE,
		)
		w.Cfg.Width = int(suggested.Right - suggested.Left)
		w.Cfg.Height = int(suggested.Bottom - suggested.Top)
		// Recreate renderer with new DPI scale
		if w.Renderer != nil {
			w.Renderer.Cleanup()
		}
		w.Renderer = NewRendererWithDPI(w.Hwnd, w.Cfg.Width, w.Cfg.Height, w.DpiScale)
		w.Redraw()
		return 0

	case WM_DESTROY:
		platform.Log("WM_DESTROY received")
		ProcPostQuitMessage.Call(0)
		return 0

	case WM_TRAYICON:
		HandleTrayMessage(w, lParam)
		return 0

	case WM_HOTKEY:
		if wParam == HOTKEY_TOGGLE {
			w.ToggleVisibility()
		}
		return 0
	}

	ret, _, _ := ProcDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}
