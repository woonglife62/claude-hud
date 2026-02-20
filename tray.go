package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	shell32             = syscall.NewLazyDLL("shell32.dll")
	procShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")

	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

// GUID structure for notification icon identification
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// NOTIFYICONDATA_V4 - Full structure for Windows Vista+ (Shell_NotifyIcon Version 4)
// This is required for proper notification area behavior on Windows 10/11,
// including "숨겨진 아이콘 표시" (Show hidden icons) in the system tray.
//
// Memory layout must exactly match the C NOTIFYICONDATAW struct:
// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-notifyicondataw
type NOTIFYICONDATA struct {
	CbSize           uint32
	HWnd             syscall.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            syscall.Handle
	SzTip            [128]uint16  // Tooltip text
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16  // Balloon notification text
	UVersion         uint32       // union with uTimeout
	SzInfoTitle      [64]uint16   // Balloon notification title
	DwInfoFlags      uint32
	GuidItem         GUID         // GUID for icon identification (V3+)
	HBalloonIcon     syscall.Handle // Custom balloon icon (V4+, Vista+)
}

const (
	NIM_ADD        = 0x00000000
	NIM_MODIFY     = 0x00000001
	NIM_DELETE     = 0x00000002
	NIM_SETVERSION = 0x00000004

	NIF_MESSAGE  = 0x00000001
	NIF_ICON     = 0x00000002
	NIF_TIP      = 0x00000004
	NIF_STATE    = 0x00000008
	NIF_INFO     = 0x00000010
	NIF_GUID     = 0x00000020
	NIF_SHOWTIP  = 0x00000080 // Windows Vista+: show tooltip even in notification area

	// Notification icon states
	NIS_HIDDEN     = 0x00000001
	NIS_SHAREDICON = 0x00000002

	NOTIFYICON_VERSION_4 = 4

	// Tray icon click messages (these come through lParam in WM_TRAYICON)
	WM_LBUTTONUP   = 0x0202
	WM_RBUTTONUP   = 0x0205
	WM_LBUTTONDBLCLK = 0x0203

	// Menu constants
	TPM_RIGHTALIGN  = 0x0008
	TPM_BOTTOMALIGN = 0x0020
	TPM_RETURNCMD   = 0x0100

	MF_STRING    = 0x00000000
	MF_SEPARATOR = 0x00000800
	MF_CHECKED   = 0x00000008

	// Menu item IDs
	ID_SHOW        = 1001
	ID_OPACITY_60  = 1002
	ID_OPACITY_80  = 1003
	ID_OPACITY_100 = 1004
	ID_PIN_DESKTOP = 1005
	ID_EXIT        = 1006
	ID_AUTOSTART   = 1007
	ID_NOTIFY      = 1008
)

// ClaudeHUD GUID - unique identifier for the notification icon.
// This ensures Windows remembers the icon's visibility preference
// across sessions and shows it in the "hidden icons" overflow area.
var claudeHUDGUID = GUID{
	Data1: 0xC1A0DE01,
	Data2: 0x4855,
	Data3: 0x4400,
	Data4: [8]byte{0xC0, 0xDE, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
}

// TrayIcon manages the system tray icon
type TrayIcon struct {
	nid NOTIFYICONDATA
}

// notifyState tracks which threshold notifications have fired per window
type notifyState struct {
	fiveHourT1Fired   bool
	fiveHourT2Fired   bool
	weeklyT1Fired     bool
	weeklyT2Fired     bool
	lastFiveHourReset time.Time
	lastWeeklyReset   time.Time
}

var notifyTracker notifyState

// ShowBalloon shows a balloon notification from the tray icon
func (t *TrayIcon) ShowBalloon(title, message string) {
	titleUTF16 := syscall.StringToUTF16(title)
	msgUTF16 := syscall.StringToUTF16(message)
	for i := 0; i < len(titleUTF16) && i < 63; i++ {
		t.nid.SzInfoTitle[i] = titleUTF16[i]
	}
	for i := 0; i < len(msgUTF16) && i < 255; i++ {
		t.nid.SzInfo[i] = msgUTF16[i]
	}
	t.nid.UFlags |= NIF_INFO
	t.nid.DwInfoFlags = 0x00000001 // NIIF_INFO
	procShellNotifyIcon.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&t.nid)))
	// Clear after showing
	t.nid.UFlags &^= NIF_INFO
}

// checkAndNotify checks usage thresholds and fires balloon notifications as needed
func checkAndNotify(tray *TrayIcon, data *HUDData, cfg *Config) {
	if !cfg.NotifyEnabled || tray == nil {
		return
	}
	for _, w := range data.Usage.Windows {
		isFiveHour := strings.Contains(w.Label, "시간")
		resetTime := w.ResetAt
		usage := w.UsagePct

		if isFiveHour {
			// Check if window reset (clear fired flags)
			if !resetTime.Equal(notifyTracker.lastFiveHourReset) {
				notifyTracker.fiveHourT1Fired = false
				notifyTracker.fiveHourT2Fired = false
				notifyTracker.lastFiveHourReset = resetTime
			}
			remaining := FormatTimeRemaining(resetTime)
			if usage >= cfg.NotifyThreshold2 && !notifyTracker.fiveHourT2Fired {
				notifyTracker.fiveHourT2Fired = true
				tray.ShowBalloon("Claude HUD - 사용량 경고",
					fmt.Sprintf("5시간 사용량 %.0f%% 도달 (리셋: %s)", usage*100, remaining))
			} else if usage >= cfg.NotifyThreshold1 && !notifyTracker.fiveHourT1Fired {
				notifyTracker.fiveHourT1Fired = true
				tray.ShowBalloon("Claude HUD - 사용량 알림",
					fmt.Sprintf("5시간 사용량 %.0f%% (리셋: %s)", usage*100, remaining))
			}
		} else {
			// Weekly window
			if !resetTime.Equal(notifyTracker.lastWeeklyReset) {
				notifyTracker.weeklyT1Fired = false
				notifyTracker.weeklyT2Fired = false
				notifyTracker.lastWeeklyReset = resetTime
			}
			remaining := FormatTimeRemaining(resetTime)
			if usage >= cfg.NotifyThreshold2 && !notifyTracker.weeklyT2Fired {
				notifyTracker.weeklyT2Fired = true
				tray.ShowBalloon("Claude HUD - 사용량 경고",
					fmt.Sprintf("주간 사용량 %.0f%% 도달 (리셋: %s)", usage*100, remaining))
			} else if usage >= cfg.NotifyThreshold1 && !notifyTracker.weeklyT1Fired {
				notifyTracker.weeklyT1Fired = true
				tray.ShowBalloon("Claude HUD - 사용량 알림",
					fmt.Sprintf("주간 사용량 %.0f%% (리셋: %s)", usage*100, remaining))
			}
		}
	}
}

// createClaudeIcon creates a custom 16x16 icon with a purple "C" for the system tray.
// This ensures the icon looks distinctive in the hidden icons panel.
func createClaudeIcon() syscall.Handle {
	// Create a 16x16 32-bit icon using GDI
	screenDC, _, _ := user32.NewProc("GetDC").Call(0)
	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	bmp, _, _ := procCreateCompatibleBitmap.Call(screenDC, 16, 16)
	oldBmp, _, _ := procSelectObject.Call(memDC, bmp)

	// Fill background with dark purple
	bgBrush, _, _ := procCreateSolidBrush.Call(uintptr(COLORREF(100, 60, 200)))
	bgRect := RECT{0, 0, 16, 16}
	procFillRect.Call(memDC, uintptr(unsafe.Pointer(&bgRect)), bgBrush)
	procDeleteObject.Call(bgBrush)

	// Draw a brighter purple circle (border)
	circleBrush, _, _ := procCreateSolidBrush.Call(uintptr(COLORREF(160, 120, 255)))
	circlePen, _, _ := procCreatePen.Call(PS_SOLID, 1, uintptr(COLORREF(160, 120, 255)))
	oldBrush2, _, _ := procSelectObject.Call(memDC, circleBrush)
	oldPen2, _, _ := procSelectObject.Call(memDC, circlePen)
	// Draw filled ellipse (circle)
	gdi32.NewProc("Ellipse").Call(memDC, 1, 1, 15, 15)
	procSelectObject.Call(memDC, oldBrush2)
	procSelectObject.Call(memDC, oldPen2)
	procDeleteObject.Call(circleBrush)
	procDeleteObject.Call(circlePen)

	// Draw "C" letter in white
	font := createFont("Segoe UI", -11, 700)
	procSelectObject.Call(memDC, uintptr(font))
	procSetBkMode.Call(memDC, TRANSPARENT_BK)
	procSetTextColor.Call(memDC, uintptr(COLORREF(255, 255, 255)))
	cText, _ := syscall.UTF16PtrFromString("C")
	cRect := RECT{0, 1, 16, 16}
	procDrawText.Call(memDC, uintptr(unsafe.Pointer(cText)), 1,
		uintptr(unsafe.Pointer(&cRect)),
		uintptr(DT_SINGLELINE|DT_NOPREFIX|0x01|0x04)) // DT_CENTER=1, DT_VCENTER=4
	procDeleteObject.Call(uintptr(font))

	procSelectObject.Call(memDC, oldBmp)

	// Create a mask bitmap (all black = fully opaque)
	maskBmp, _, _ := gdi32.NewProc("CreateBitmap").Call(16, 16, 1, 1, 0)

	// Create the icon from color + mask bitmaps
	type ICONINFO struct {
		FIcon    int32
		XHotspot uint32
		YHotspot uint32
		HbmMask  syscall.Handle
		HbmColor syscall.Handle
	}
	ii := ICONINFO{
		FIcon:    1, // TRUE = icon
		HbmMask:  syscall.Handle(maskBmp),
		HbmColor: syscall.Handle(bmp),
	}
	icon, _, _ := user32.NewProc("CreateIconIndirect").Call(uintptr(unsafe.Pointer(&ii)))

	// Cleanup intermediate GDI objects
	procDeleteObject.Call(bmp)
	procDeleteObject.Call(maskBmp)
	procDeleteDC.Call(memDC)
	user32.NewProc("ReleaseDC").Call(0, screenDC)

	return syscall.Handle(icon)
}

// NewTrayIcon creates and shows a system tray icon with full
// notification area support, including the hidden icons panel.
func NewTrayIcon(hwnd syscall.Handle, hInstance syscall.Handle) *TrayIcon {
	t := &TrayIcon{}

	// Create a custom Claude icon (purple circle with "C")
	icon := uintptr(createClaudeIcon())
	if icon == 0 {
		// Fallback to default application icon
		icon, _, _ = user32.NewProc("LoadIconW").Call(0, uintptr(32512))
	}

	t.nid = NOTIFYICONDATA{
		CbSize:           uint32(unsafe.Sizeof(NOTIFYICONDATA{})),
		HWnd:             hwnd,
		UID:              1,
		UFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP | NIF_GUID,
		UCallbackMessage: WM_TRAYICON,
		HIcon:            syscall.Handle(icon),
		GuidItem:         claudeHUDGUID,
	}

	// Set tooltip text
	tip := "Claude HUD - Desktop Monitor"
	tipRunes := []rune(tip)
	for i := 0; i < len(tipRunes) && i < 127; i++ {
		t.nid.SzTip[i] = uint16(tipRunes[i])
	}

	// Add the icon to the notification area
	Log("  Tray NID CbSize=%d, Icon=0x%X", t.nid.CbSize, t.nid.HIcon)
	ret, _, err := procShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&t.nid)))
	Log("  Shell_NotifyIcon(NIM_ADD with GUID): ret=%d, err=%v", ret, err)
	if ret == 0 {
		// GUID-based add can fail on first run or if GUID was never registered.
		// Retry without GUID flag.
		t.nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP
		ret2, _, err2 := procShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&t.nid)))
		Log("  Shell_NotifyIcon(NIM_ADD no GUID): ret=%d, err=%v", ret2, err2)
	}

	// Set the icon to use Version 4 behavior (Windows Vista+)
	t.nid.UVersion = NOTIFYICON_VERSION_4
	ret3, _, _ := procShellNotifyIcon.Call(NIM_SETVERSION, uintptr(unsafe.Pointer(&t.nid)))
	Log("  Shell_NotifyIcon(NIM_SETVERSION): ret=%d", ret3)

	return t
}

// UpdateTooltip updates the tray icon tooltip text
func (t *TrayIcon) UpdateTooltip(text string) {
	// Clear existing tooltip
	for i := range t.nid.SzTip {
		t.nid.SzTip[i] = 0
	}
	tipRunes := []rune(text)
	for i := 0; i < len(tipRunes) && i < 127; i++ {
		t.nid.SzTip[i] = uint16(tipRunes[i])
	}
	t.nid.UFlags = NIF_TIP | NIF_SHOWTIP
	procShellNotifyIcon.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&t.nid)))
}

// Remove removes the tray icon from the notification area
func (t *TrayIcon) Remove() {
	procShellNotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(&t.nid)))
	// Destroy the icon handle to prevent GDI leak
	if t.nid.HIcon != 0 {
		user32.NewProc("DestroyIcon").Call(uintptr(t.nid.HIcon))
		t.nid.HIcon = 0
	}
}

// handleTrayMessage processes tray icon messages
func handleTrayMessage(w *HUDWindow, lParam uintptr) {
	// For NOTIFYICON_VERSION_4, the message is in LOWORD of lParam
	msg := lParam & 0xFFFF

	switch msg {
	case WM_RBUTTONUP:
		showTrayMenu(w)
	case WM_LBUTTONUP:
		w.ToggleVisibility()
	case WM_LBUTTONDBLCLK:
		// Double-click: always show
		if !w.visible {
			w.ToggleVisibility()
		}
	}
}

// showTrayMenu shows the right-click context menu
func showTrayMenu(w *HUDWindow) {
	hMenu, _, _ := procCreatePopupMenu.Call()

	// Show/Hide toggle
	if w.visible {
		appendMenuItem(hMenu, ID_SHOW, "Hide HUD")
	} else {
		appendMenuItem(hMenu, ID_SHOW, "Show HUD")
	}
	appendSeparator(hMenu)

	// Opacity options
	flags60 := uint32(MF_STRING)
	flags80 := uint32(MF_STRING)
	flags100 := uint32(MF_STRING)
	switch {
	case w.cfg.Opacity <= 160:
		flags60 |= MF_CHECKED
	case w.cfg.Opacity <= 210:
		flags80 |= MF_CHECKED
	default:
		flags100 |= MF_CHECKED
	}

	procAppendMenu.Call(hMenu, uintptr(flags60), ID_OPACITY_60,
		uintptr(unsafe.Pointer(utf16Ptr("Opacity: 60%"))))
	procAppendMenu.Call(hMenu, uintptr(flags80), ID_OPACITY_80,
		uintptr(unsafe.Pointer(utf16Ptr("Opacity: 80%"))))
	procAppendMenu.Call(hMenu, uintptr(flags100), ID_OPACITY_100,
		uintptr(unsafe.Pointer(utf16Ptr("Opacity: 100%"))))
	appendSeparator(hMenu)

	// Pin to desktop option
	pinFlags := uint32(MF_STRING)
	if w.cfg.PinDesktop {
		pinFlags |= MF_CHECKED
	}
	procAppendMenu.Call(hMenu, uintptr(pinFlags), ID_PIN_DESKTOP,
		uintptr(unsafe.Pointer(utf16Ptr("Pin to Desktop"))))
	appendSeparator(hMenu)

	// Auto-start on login option
	autoStartFlags := uint32(MF_STRING)
	if isAutoStartEnabled() {
		autoStartFlags |= MF_CHECKED
	}
	procAppendMenu.Call(hMenu, uintptr(autoStartFlags), ID_AUTOSTART,
		uintptr(unsafe.Pointer(utf16Ptr("Start with Windows"))))

	// Notifications toggle
	notifyFlags := uint32(MF_STRING)
	if w.cfg.NotifyEnabled {
		notifyFlags |= MF_CHECKED
	}
	procAppendMenu.Call(hMenu, uintptr(notifyFlags), ID_NOTIFY,
		uintptr(unsafe.Pointer(utf16Ptr("알림"))))
	appendSeparator(hMenu)

	appendMenuItem(hMenu, ID_EXIT, "Exit")

	// Get cursor position for menu placement
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// Set foreground to ensure menu closes when clicking away
	procSetForegroundWindow.Call(uintptr(w.hwnd))

	ret, _, _ := procTrackPopupMenu.Call(
		hMenu,
		TPM_RIGHTALIGN|TPM_BOTTOMALIGN|TPM_RETURNCMD,
		uintptr(pt.X), uintptr(pt.Y),
		0,
		uintptr(w.hwnd),
		0,
	)

	procDestroyMenu.Call(hMenu)

	// Handle menu selection
	switch ret {
	case ID_SHOW:
		w.ToggleVisibility()
	case ID_OPACITY_60:
		w.cfg.Opacity = 153
		procSetLayeredWindowAttributes.Call(uintptr(w.hwnd), 0, 153, LWA_ALPHA)
	case ID_OPACITY_80:
		w.cfg.Opacity = 204
		procSetLayeredWindowAttributes.Call(uintptr(w.hwnd), 0, 204, LWA_ALPHA)
	case ID_OPACITY_100:
		w.cfg.Opacity = 255
		procSetLayeredWindowAttributes.Call(uintptr(w.hwnd), 0, 255, LWA_ALPHA)
	case ID_PIN_DESKTOP:
		w.cfg.PinDesktop = !w.cfg.PinDesktop
		if w.cfg.PinDesktop {
			PinToDesktop(w.hwnd)
		} else {
			UnpinFromDesktop(w.hwnd)
		}
	case ID_AUTOSTART:
		w.cfg.AutoStart = !w.cfg.AutoStart
		setAutoStart(w.cfg.AutoStart)
	case ID_NOTIFY:
		w.cfg.NotifyEnabled = !w.cfg.NotifyEnabled
	case ID_EXIT:
		procPostMessage.Call(uintptr(w.hwnd), WM_CLOSE, 0, 0)
	}

	SaveConfig(w.cfg)
}

func appendMenuItem(hMenu uintptr, id uintptr, text string) {
	procAppendMenu.Call(hMenu, MF_STRING, id, uintptr(unsafe.Pointer(utf16Ptr(text))))
}

func appendSeparator(hMenu uintptr) {
	procAppendMenu.Call(hMenu, MF_SEPARATOR, 0, 0)
}
