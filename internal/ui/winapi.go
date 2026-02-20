package ui

import (
	"syscall"
	"unsafe"
)

var (
	User32   = syscall.NewLazyDLL("user32.dll")
	Kernel32 = syscall.NewLazyDLL("kernel32.dll")
	Gdi32    = syscall.NewLazyDLL("gdi32.dll")
	Shell32  = syscall.NewLazyDLL("shell32.dll")

	// user32
	ProcRegisterClassEx            = User32.NewProc("RegisterClassExW")
	ProcCreateWindowEx             = User32.NewProc("CreateWindowExW")
	ProcDefWindowProc              = User32.NewProc("DefWindowProcW")
	ProcGetMessage                 = User32.NewProc("GetMessageW")
	ProcTranslateMessage           = User32.NewProc("TranslateMessage")
	ProcDispatchMessage            = User32.NewProc("DispatchMessageW")
	ProcPostQuitMessage            = User32.NewProc("PostQuitMessage")
	ProcShowWindow                 = User32.NewProc("ShowWindow")
	ProcUpdateWindow               = User32.NewProc("UpdateWindow")
	ProcIsWindowVisible            = User32.NewProc("IsWindowVisible")
	ProcSetLayeredWindowAttributes = User32.NewProc("SetLayeredWindowAttributes")
	ProcInvalidateRect             = User32.NewProc("InvalidateRect")
	ProcSetTimer                   = User32.NewProc("SetTimer")
	ProcKillTimer                  = User32.NewProc("KillTimer")
	ProcBeginPaint                 = User32.NewProc("BeginPaint")
	ProcEndPaint                   = User32.NewProc("EndPaint")
	ProcLoadCursor                 = User32.NewProc("LoadCursorW")
	ProcGetCursorPos               = User32.NewProc("GetCursorPos")
	ProcReleaseCapture             = User32.NewProc("ReleaseCapture")
	ProcSendMessage                = User32.NewProc("SendMessageW")
	ProcSetWindowPos               = User32.NewProc("SetWindowPos")
	ProcGetWindowRect              = User32.NewProc("GetWindowRect")
	ProcFindWindow                 = User32.NewProc("FindWindowW")
	ProcFindWindowEx               = User32.NewProc("FindWindowExW")
	ProcEnumWindows                = User32.NewProc("EnumWindows")
	ProcSetParent                  = User32.NewProc("SetParent")
	ProcTrackMouseEvent            = User32.NewProc("TrackMouseEvent")
	ProcPostMessage                = User32.NewProc("PostMessageW")
	ProcFillRect                   = User32.NewProc("FillRect")
	ProcDrawText                   = User32.NewProc("DrawTextW")
	ProcMonitorFromWindow          = User32.NewProc("MonitorFromWindow")
	ProcGetMonitorInfo             = User32.NewProc("GetMonitorInfoW")
	ProcCreatePopupMenu            = User32.NewProc("CreatePopupMenu")
	ProcAppendMenu                 = User32.NewProc("AppendMenuW")
	ProcTrackPopupMenu             = User32.NewProc("TrackPopupMenu")
	ProcDestroyMenu                = User32.NewProc("DestroyMenu")
	ProcSetForegroundWindow        = User32.NewProc("SetForegroundWindow")
	ProcGetDpiForWindow            = User32.NewProc("GetDpiForWindow")
	ProcSetProcessDpiAwarenessCtx  = User32.NewProc("SetProcessDpiAwarenessContext")
	ProcRegisterHotKey             = User32.NewProc("RegisterHotKey")
	ProcUnregisterHotKey           = User32.NewProc("UnregisterHotKey")

	// kernel32
	ProcGetModuleHandle = Kernel32.NewProc("GetModuleHandleW")

	// gdi32
	ProcCreateCompatibleDC     = Gdi32.NewProc("CreateCompatibleDC")
	ProcCreateCompatibleBitmap = Gdi32.NewProc("CreateCompatibleBitmap")
	ProcSelectObject           = Gdi32.NewProc("SelectObject")
	ProcDeleteObject           = Gdi32.NewProc("DeleteObject")
	ProcDeleteDC               = Gdi32.NewProc("DeleteDC")
	ProcBitBlt                 = Gdi32.NewProc("BitBlt")
	ProcCreateSolidBrush       = Gdi32.NewProc("CreateSolidBrush")
	ProcCreatePen              = Gdi32.NewProc("CreatePen")
	ProcRoundRect              = Gdi32.NewProc("RoundRect")
	ProcSetBkMode              = Gdi32.NewProc("SetBkMode")
	ProcSetTextColor           = Gdi32.NewProc("SetTextColor")
	ProcCreateFontIndirect     = Gdi32.NewProc("CreateFontIndirectW")
	ProcTextOut                = Gdi32.NewProc("TextOutW")
	ProcGetStockObject         = Gdi32.NewProc("GetStockObject")
	ProcMoveToEx               = Gdi32.NewProc("MoveToEx")
	ProcLineTo                 = Gdi32.NewProc("LineTo")
	ProcSaveDC                 = Gdi32.NewProc("SaveDC")
	ProcRestoreDC              = Gdi32.NewProc("RestoreDC")
	ProcIntersectClipRect      = Gdi32.NewProc("IntersectClipRect")
	ProcEllipse                = Gdi32.NewProc("Ellipse")

	// shell32
	ProcShellNotifyIcon = Shell32.NewProc("Shell_NotifyIconW")
)

// Window style constants
const (
	WS_POPUP         = 0x80000000
	WS_THICKFRAME    = 0x00040000
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

	WM_DPICHANGED = 0x02E0

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

	RESIZE_BORDER = 6

	SRCCOPY = 0x00CC0020

	TRANSPARENT_BK = 1

	IDC_ARROW = 32512

	LWA_ALPHA = 0x02

	TIMER_REPAINT       = 1
	TIMER_REFRESH_PRESS = 2
	TIMER_DATA_REFRESH  = 3

	WM_DATA_READY = WM_USER + 2

	WM_HOTKEY = 0x0312

	// Hotkey modifier keys
	MOD_CONTROL = 0x0002
	MOD_SHIFT   = 0x0004

	// Hotkey ID
	HOTKEY_TOGGLE = 1

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

	SNAP_DISTANCE = 4
	SNAP_MARGIN   = -2

	// Tray icon constants
	NIM_ADD        = 0x00000000
	NIM_MODIFY     = 0x00000001
	NIM_DELETE     = 0x00000002
	NIM_SETVERSION = 0x00000004

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004
	NIF_STATE   = 0x00000008
	NIF_INFO    = 0x00000010
	NIF_GUID    = 0x00000020
	NIF_SHOWTIP = 0x00000080

	NIS_HIDDEN     = 0x00000001
	NIS_SHAREDICON = 0x00000002

	NOTIFYICON_VERSION_4 = 4

	WM_LBUTTONUP     = 0x0202
	WM_RBUTTONUP     = 0x0205
	WM_LBUTTONDBLCLK = 0x0203

	TPM_RIGHTALIGN  = 0x0008
	TPM_BOTTOMALIGN = 0x0020
	TPM_RETURNCMD   = 0x0100

	MF_STRING    = 0x00000000
	MF_SEPARATOR = 0x00000800
	MF_CHECKED   = 0x00000008

	ID_SHOW        = 1001
	ID_OPACITY_60  = 1002
	ID_OPACITY_80  = 1003
	ID_OPACITY_100 = 1004
	ID_PIN_DESKTOP = 1005
	ID_EXIT        = 1006
	ID_AUTOSTART   = 1007
	ID_NOTIFY      = 1008
	ID_COMPACT     = 1009
	ID_LANGUAGE    = 1010
	ID_THEME       = 1011
)

// Card layout constants shared between render and scroll calculation
const (
	CardBaseHeight     = 36
	CardAgentHeight    = 31
	CardSubAgentHeight = 28
	CardMessagesHeight = 18
	CardNoAgentsHeight = 16
	CardStrategyHeight = 18
	CardMinHeight      = 40
	CardGap            = 8
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

// GUID structure for notification icon identification
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// NOTIFYICONDATA - Full structure for Windows Vista+
type NOTIFYICONDATA struct {
	CbSize           uint32
	HWnd             syscall.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            syscall.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         GUID
	HBalloonIcon     syscall.Handle
}

// Utf16Ptr converts a Go string to a UTF16 pointer
func Utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

// Utf16Len returns the number of UTF-16 code units (excluding null terminator)
func Utf16Len(s string) int {
	u, _ := syscall.UTF16FromString(s)
	if len(u) > 0 {
		return len(u) - 1
	}
	return 0
}

// ClampInt clamps a value between min and max
func ClampInt(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}

// Abs32 returns the absolute value of an int32
func Abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// SessionHitRect tracks the clickable area of a session card header
type SessionHitRect struct {
	SessionKey string
	Top        int32
	Bottom     int32
}

// LogStructSizes logs the sizes of critical structs for debugging
func LogStructSizes() {
	// Import platform for logging
	// This is called from main, so we just provide sizes via unsafe.Sizeof
}

// GetStructSizes returns a map of struct name to size for debug logging
func GetStructSizes() map[string]uintptr {
	return map[string]uintptr{
		"WNDCLASSEX":      unsafe.Sizeof(WNDCLASSEX{}),
		"MSG":             unsafe.Sizeof(MSG{}),
		"PAINTSTRUCT":     unsafe.Sizeof(PAINTSTRUCT{}),
		"NOTIFYICONDATA":  unsafe.Sizeof(NOTIFYICONDATA{}),
		"TRACKMOUSEEVENT": unsafe.Sizeof(TRACKMOUSEEVENT{}),
		"LOGFONT":         unsafe.Sizeof(LOGFONT{}),
		"POINT":           unsafe.Sizeof(POINT{}),
		"RECT":            unsafe.Sizeof(RECT{}),
		"GUID":            unsafe.Sizeof(GUID{}),
	}
}
