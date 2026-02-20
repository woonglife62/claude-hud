package ui

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"claude-hud/internal/config"
	"claude-hud/internal/i18n"
	"claude-hud/internal/model"
	"claude-hud/internal/platform"
)

// ClaudeHUD GUID - unique identifier for the notification icon.
var claudeHUDGUID = GUID{
	Data1: 0xC1A0DE01,
	Data2: 0x4855,
	Data3: 0x4400,
	Data4: [8]byte{0xC0, 0xDE, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
}

// TrayIcon manages the system tray icon
type TrayIcon struct {
	Nid NOTIFYICONDATA
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
		t.Nid.SzInfoTitle[i] = titleUTF16[i]
	}
	for i := 0; i < len(msgUTF16) && i < 255; i++ {
		t.Nid.SzInfo[i] = msgUTF16[i]
	}
	t.Nid.UFlags |= NIF_INFO
	t.Nid.DwInfoFlags = 0x00000001 // NIIF_INFO
	ProcShellNotifyIcon.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&t.Nid)))
	t.Nid.UFlags &^= NIF_INFO
}

// CheckAndNotify checks usage thresholds and fires balloon notifications as needed
func CheckAndNotify(tray *TrayIcon, data *model.HUDData, cfg *config.Config) {
	if !cfg.NotifyEnabled || tray == nil {
		return
	}
	for _, w := range data.Usage.Windows {
		isFiveHour := strings.Contains(w.Label, i18n.T.FiveHour)
		resetTime := w.ResetAt
		usage := w.UsagePct

		if isFiveHour {
			if !resetTime.Equal(notifyTracker.lastFiveHourReset) {
				notifyTracker.fiveHourT1Fired = false
				notifyTracker.fiveHourT2Fired = false
				notifyTracker.lastFiveHourReset = resetTime
			}
			remaining := model.FormatTimeRemaining(resetTime)
			if usage >= cfg.NotifyThreshold2 && !notifyTracker.fiveHourT2Fired {
				notifyTracker.fiveHourT2Fired = true
				tray.ShowBalloon(i18n.T.NotifyWarning,
					fmt.Sprintf(i18n.T.FiveHourWarnMsg, usage*100, remaining))
			} else if usage >= cfg.NotifyThreshold1 && !notifyTracker.fiveHourT1Fired {
				notifyTracker.fiveHourT1Fired = true
				tray.ShowBalloon(i18n.T.NotifyInfo,
					fmt.Sprintf(i18n.T.FiveHourInfoMsg, usage*100, remaining))
			}
		} else {
			if !resetTime.Equal(notifyTracker.lastWeeklyReset) {
				notifyTracker.weeklyT1Fired = false
				notifyTracker.weeklyT2Fired = false
				notifyTracker.lastWeeklyReset = resetTime
			}
			remaining := model.FormatTimeRemaining(resetTime)
			if usage >= cfg.NotifyThreshold2 && !notifyTracker.weeklyT2Fired {
				notifyTracker.weeklyT2Fired = true
				tray.ShowBalloon(i18n.T.NotifyWarning,
					fmt.Sprintf(i18n.T.WeeklyWarnMsg, usage*100, remaining))
			} else if usage >= cfg.NotifyThreshold1 && !notifyTracker.weeklyT1Fired {
				notifyTracker.weeklyT1Fired = true
				tray.ShowBalloon(i18n.T.NotifyInfo,
					fmt.Sprintf(i18n.T.WeeklyInfoMsg, usage*100, remaining))
			}
		}
	}
}

// CreateClaudeIcon creates a custom 16x16 icon with a purple "C" for the system tray.
func CreateClaudeIcon() syscall.Handle {
	screenDC, _, _ := User32.NewProc("GetDC").Call(0)
	memDC, _, _ := ProcCreateCompatibleDC.Call(screenDC)
	bmp, _, _ := ProcCreateCompatibleBitmap.Call(screenDC, 16, 16)
	oldBmp, _, _ := ProcSelectObject.Call(memDC, bmp)

	bgBrush, _, _ := ProcCreateSolidBrush.Call(uintptr(COLORREF(100, 60, 200)))
	bgRect := RECT{0, 0, 16, 16}
	ProcFillRect.Call(memDC, uintptr(unsafe.Pointer(&bgRect)), bgBrush)
	ProcDeleteObject.Call(bgBrush)

	circleBrush, _, _ := ProcCreateSolidBrush.Call(uintptr(COLORREF(160, 120, 255)))
	circlePen, _, _ := ProcCreatePen.Call(PS_SOLID, 1, uintptr(COLORREF(160, 120, 255)))
	oldBrush2, _, _ := ProcSelectObject.Call(memDC, circleBrush)
	oldPen2, _, _ := ProcSelectObject.Call(memDC, circlePen)
	Gdi32.NewProc("Ellipse").Call(memDC, 1, 1, 15, 15)
	ProcSelectObject.Call(memDC, oldBrush2)
	ProcSelectObject.Call(memDC, oldPen2)
	ProcDeleteObject.Call(circleBrush)
	ProcDeleteObject.Call(circlePen)

	font := CreateFont("Segoe UI", -11, 700)
	ProcSelectObject.Call(memDC, uintptr(font))
	ProcSetBkMode.Call(memDC, TRANSPARENT_BK)
	ProcSetTextColor.Call(memDC, uintptr(COLORREF(255, 255, 255)))
	cText, _ := syscall.UTF16PtrFromString("C")
	cRect := RECT{0, 1, 16, 16}
	ProcDrawText.Call(memDC, uintptr(unsafe.Pointer(cText)), 1,
		uintptr(unsafe.Pointer(&cRect)),
		uintptr(DT_SINGLELINE|DT_NOPREFIX|0x01|0x04))
	ProcDeleteObject.Call(uintptr(font))

	ProcSelectObject.Call(memDC, oldBmp)

	maskBmp, _, _ := Gdi32.NewProc("CreateBitmap").Call(16, 16, 1, 1, 0)

	type ICONINFO struct {
		FIcon    int32
		XHotspot uint32
		YHotspot uint32
		HbmMask  syscall.Handle
		HbmColor syscall.Handle
	}
	ii := ICONINFO{
		FIcon:    1,
		HbmMask:  syscall.Handle(maskBmp),
		HbmColor: syscall.Handle(bmp),
	}
	icon, _, _ := User32.NewProc("CreateIconIndirect").Call(uintptr(unsafe.Pointer(&ii)))

	ProcDeleteObject.Call(bmp)
	ProcDeleteObject.Call(maskBmp)
	ProcDeleteDC.Call(memDC)
	User32.NewProc("ReleaseDC").Call(0, screenDC)

	return syscall.Handle(icon)
}

// NewTrayIcon creates and shows a system tray icon
func NewTrayIcon(hwnd syscall.Handle, hInstance syscall.Handle) *TrayIcon {
	t := &TrayIcon{}

	icon := uintptr(CreateClaudeIcon())
	if icon == 0 {
		icon, _, _ = User32.NewProc("LoadIconW").Call(0, uintptr(32512))
	}

	t.Nid = NOTIFYICONDATA{
		CbSize:           uint32(unsafe.Sizeof(NOTIFYICONDATA{})),
		HWnd:             hwnd,
		UID:              1,
		UFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP | NIF_GUID,
		UCallbackMessage: WM_TRAYICON,
		HIcon:            syscall.Handle(icon),
		GuidItem:         claudeHUDGUID,
	}

	tip := "Claude HUD - Desktop Monitor"
	tipRunes := []rune(tip)
	for i := 0; i < len(tipRunes) && i < 127; i++ {
		t.Nid.SzTip[i] = uint16(tipRunes[i])
	}

	platform.Log("  Tray NID CbSize=%d, Icon=0x%X", t.Nid.CbSize, t.Nid.HIcon)
	ret, _, err := ProcShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&t.Nid)))
	platform.Log("  Shell_NotifyIcon(NIM_ADD with GUID): ret=%d, err=%v", ret, err)
	if ret == 0 {
		t.Nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP
		ret2, _, err2 := ProcShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&t.Nid)))
		platform.Log("  Shell_NotifyIcon(NIM_ADD no GUID): ret=%d, err=%v", ret2, err2)
	}

	t.Nid.UVersion = NOTIFYICON_VERSION_4
	ret3, _, _ := ProcShellNotifyIcon.Call(NIM_SETVERSION, uintptr(unsafe.Pointer(&t.Nid)))
	platform.Log("  Shell_NotifyIcon(NIM_SETVERSION): ret=%d", ret3)

	return t
}

// UpdateTooltip updates the tray icon tooltip text
func (t *TrayIcon) UpdateTooltip(text string) {
	for i := range t.Nid.SzTip {
		t.Nid.SzTip[i] = 0
	}
	tipRunes := []rune(text)
	for i := 0; i < len(tipRunes) && i < 127; i++ {
		t.Nid.SzTip[i] = uint16(tipRunes[i])
	}
	t.Nid.UFlags = NIF_TIP | NIF_SHOWTIP
	ProcShellNotifyIcon.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&t.Nid)))
}

// Remove removes the tray icon from the notification area
func (t *TrayIcon) Remove() {
	ProcShellNotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(&t.Nid)))
	if t.Nid.HIcon != 0 {
		User32.NewProc("DestroyIcon").Call(uintptr(t.Nid.HIcon))
		t.Nid.HIcon = 0
	}
}

// HandleTrayMessage processes tray icon messages
func HandleTrayMessage(w *HUDWindow, lParam uintptr) {
	msg := lParam & 0xFFFF

	switch msg {
	case WM_RBUTTONUP:
		ShowTrayMenu(w)
	case WM_LBUTTONUP:
		w.ToggleVisibility()
	case WM_LBUTTONDBLCLK:
		if !w.Visible {
			w.ToggleVisibility()
		}
	}
}

// ShowTrayMenu shows the right-click context menu
func ShowTrayMenu(w *HUDWindow) {
	hMenu, _, _ := ProcCreatePopupMenu.Call()

	if w.Visible {
		appendMenuItem(hMenu, ID_SHOW, "Hide HUD")
	} else {
		appendMenuItem(hMenu, ID_SHOW, "Show HUD")
	}
	appendSeparator(hMenu)

	flags60 := uint32(MF_STRING)
	flags80 := uint32(MF_STRING)
	flags100 := uint32(MF_STRING)
	switch {
	case w.Cfg.Opacity <= 160:
		flags60 |= MF_CHECKED
	case w.Cfg.Opacity <= 210:
		flags80 |= MF_CHECKED
	default:
		flags100 |= MF_CHECKED
	}

	ProcAppendMenu.Call(hMenu, uintptr(flags60), ID_OPACITY_60,
		uintptr(unsafe.Pointer(Utf16Ptr("Opacity: 60%"))))
	ProcAppendMenu.Call(hMenu, uintptr(flags80), ID_OPACITY_80,
		uintptr(unsafe.Pointer(Utf16Ptr("Opacity: 80%"))))
	ProcAppendMenu.Call(hMenu, uintptr(flags100), ID_OPACITY_100,
		uintptr(unsafe.Pointer(Utf16Ptr("Opacity: 100%"))))
	appendSeparator(hMenu)

	pinFlags := uint32(MF_STRING)
	if w.Cfg.PinDesktop {
		pinFlags |= MF_CHECKED
	}
	ProcAppendMenu.Call(hMenu, uintptr(pinFlags), ID_PIN_DESKTOP,
		uintptr(unsafe.Pointer(Utf16Ptr(i18n.T.PinToDesktop))))
	appendSeparator(hMenu)

	autoStartFlags := uint32(MF_STRING)
	if platform.IsAutoStartEnabled() {
		autoStartFlags |= MF_CHECKED
	}
	ProcAppendMenu.Call(hMenu, uintptr(autoStartFlags), ID_AUTOSTART,
		uintptr(unsafe.Pointer(Utf16Ptr(i18n.T.StartWithWindows))))

	notifyFlags := uint32(MF_STRING)
	if w.Cfg.NotifyEnabled {
		notifyFlags |= MF_CHECKED
	}
	ProcAppendMenu.Call(hMenu, uintptr(notifyFlags), ID_NOTIFY,
		uintptr(unsafe.Pointer(Utf16Ptr(i18n.T.Notifications))))
	appendSeparator(hMenu)

	compactFlags := uint32(MF_STRING)
	if w.Cfg.CompactMode {
		compactFlags |= MF_CHECKED
	}
	ProcAppendMenu.Call(hMenu, uintptr(compactFlags), ID_COMPACT,
		uintptr(unsafe.Pointer(Utf16Ptr("Compact Mode"))))
	appendSeparator(hMenu)

	themeLabel := "Light Theme"
	if w.Cfg.ThemeMode == "light" {
		themeLabel = "Dark Theme"
	}
	appendMenuItem(hMenu, ID_THEME, themeLabel)
	appendSeparator(hMenu)

	appendMenuItem(hMenu, ID_LANGUAGE, i18n.T.LanguageToggle)
	appendSeparator(hMenu)

	appendMenuItem(hMenu, ID_EXIT, i18n.T.Exit)

	var pt POINT
	ProcGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	ProcSetForegroundWindow.Call(uintptr(w.Hwnd))

	ret, _, _ := ProcTrackPopupMenu.Call(
		hMenu,
		TPM_RIGHTALIGN|TPM_BOTTOMALIGN|TPM_RETURNCMD,
		uintptr(pt.X), uintptr(pt.Y),
		0,
		uintptr(w.Hwnd),
		0,
	)

	ProcDestroyMenu.Call(hMenu)

	switch ret {
	case ID_SHOW:
		w.ToggleVisibility()
	case ID_OPACITY_60:
		w.Cfg.Opacity = 153
		ProcSetLayeredWindowAttributes.Call(uintptr(w.Hwnd), 0, 153, LWA_ALPHA)
	case ID_OPACITY_80:
		w.Cfg.Opacity = 204
		ProcSetLayeredWindowAttributes.Call(uintptr(w.Hwnd), 0, 204, LWA_ALPHA)
	case ID_OPACITY_100:
		w.Cfg.Opacity = 255
		ProcSetLayeredWindowAttributes.Call(uintptr(w.Hwnd), 0, 255, LWA_ALPHA)
	case ID_PIN_DESKTOP:
		w.Cfg.PinDesktop = !w.Cfg.PinDesktop
		if w.Cfg.PinDesktop {
			platform.PinToDesktop(w.Hwnd)
		} else {
			platform.UnpinFromDesktop(w.Hwnd)
		}
	case ID_AUTOSTART:
		w.Cfg.AutoStart = !w.Cfg.AutoStart
		platform.SetAutoStart(w.Cfg.AutoStart)
	case ID_NOTIFY:
		w.Cfg.NotifyEnabled = !w.Cfg.NotifyEnabled
	case ID_COMPACT:
		w.Cfg.CompactMode = !w.Cfg.CompactMode
		if w.Cfg.CompactMode {
			ProcSetWindowPos.Call(uintptr(w.Hwnd), 0, 0, 0,
				uintptr(w.Cfg.Width), 150,
				SWP_NOMOVE|SWP_NOACTIVATE)
			w.Cfg.Height = 150
		} else {
			ProcSetWindowPos.Call(uintptr(w.Hwnd), 0, 0, 0,
				uintptr(w.Cfg.Width), 640,
				SWP_NOMOVE|SWP_NOACTIVATE)
			w.Cfg.Height = 640
		}
		w.Redraw()
	case ID_THEME:
		if w.Cfg.ThemeMode == "light" {
			w.Cfg.ThemeMode = "dark"
			w.Renderer.RebuildCache(DarkScheme)
		} else {
			w.Cfg.ThemeMode = "light"
			w.Renderer.RebuildCache(LightScheme)
		}
		w.Redraw()
	case ID_LANGUAGE:
		if w.Cfg.Language == "en" {
			w.Cfg.Language = "ko"
		} else {
			w.Cfg.Language = "en"
		}
		i18n.SetLanguage(w.Cfg.Language)
	case ID_EXIT:
		ProcPostMessage.Call(uintptr(w.Hwnd), WM_CLOSE, 0, 0)
	}

	config.SaveConfig(w.Cfg)
}

func appendMenuItem(hMenu uintptr, id uintptr, text string) {
	ProcAppendMenu.Call(hMenu, MF_STRING, id, uintptr(unsafe.Pointer(Utf16Ptr(text))))
}

func appendSeparator(hMenu uintptr) {
	ProcAppendMenu.Call(hMenu, MF_SEPARATOR, 0, 0)
}
