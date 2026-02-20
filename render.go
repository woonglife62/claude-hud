package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// ColorScheme defines the HUD color palette
type ColorScheme struct {
	BgColor       uint32
	CardColor     uint32
	CardBorder    uint32
	TextPrimary   uint32
	TextSecondary uint32
	TextMuted     uint32
	AccentPurple  uint32
	AccentGreen   uint32
	AccentOrange  uint32
	AccentRed     uint32
	AccentBlue    uint32
	AccentCyan    uint32
	ProgressBg    uint32
	ProgressFill  uint32
	Divider       uint32
}

// GDICache holds pre-created GDI brushes and pens that are reused across render frames
type GDICache struct {
	// Brushes
	brushBg           syscall.Handle
	brushCard         syscall.Handle
	brushProgressBg   syscall.Handle
	brushProgressFill syscall.Handle
	brushAccentOrange syscall.Handle
	brushAccentRed    syscall.Handle
	brushAccentGreen  syscall.Handle
	brushAccentPurple syscall.Handle
	// Pens
	penCardBorder syscall.Handle
	penDivider    syscall.Handle
}

var darkScheme = ColorScheme{
	BgColor:       COLORREF(24, 24, 32),
	CardColor:     COLORREF(36, 36, 50),
	CardBorder:    COLORREF(55, 55, 75),
	TextPrimary:   COLORREF(240, 240, 245),
	TextSecondary: COLORREF(180, 180, 200),
	TextMuted:     COLORREF(120, 120, 150),
	AccentPurple:  COLORREF(160, 120, 255),
	AccentGreen:   COLORREF(80, 220, 120),
	AccentOrange:  COLORREF(255, 180, 60),
	AccentRed:     COLORREF(255, 90, 90),
	AccentBlue:    COLORREF(80, 160, 255),
	AccentCyan:    COLORREF(80, 220, 220),
	ProgressBg:    COLORREF(45, 45, 65),
	ProgressFill:  COLORREF(160, 120, 255),
	Divider:       COLORREF(50, 50, 70),
}

// Renderer handles all GDI drawing operations
type Renderer struct {
	hwnd      syscall.Handle
	width     int
	height    int
	fontTitle syscall.Handle
	fontBody  syscall.Handle
	fontSmall syscall.Handle
	fontBold  syscall.Handle
	fontMono  syscall.Handle
	fontIcon  syscall.Handle
	cache     GDICache
}

// NewRenderer creates a new renderer for the given window
func NewRenderer(hwnd syscall.Handle, width, height int) *Renderer {
	r := &Renderer{
		hwnd:   hwnd,
		width:  width,
		height: height,
	}

	// Create fonts (negative height = point size via GDI convention)
	r.fontTitle = createFont("Segoe UI", -18, 700)
	r.fontBody = createFont("Segoe UI", -13, 400)
	r.fontSmall = createFont("Segoe UI", -11, 400)
	r.fontBold = createFont("Segoe UI Semibold", -13, 600)
	r.fontMono = createFont("Cascadia Code", -12, 400)
	r.fontIcon = createFont("Segoe UI", -12, 400)

	// Create cached GDI brushes and pens from the color scheme
	h, _, _ := procCreateSolidBrush.Call(uintptr(darkScheme.BgColor))
	r.cache.brushBg = syscall.Handle(h)
	h, _, _ = procCreateSolidBrush.Call(uintptr(darkScheme.CardColor))
	r.cache.brushCard = syscall.Handle(h)
	h, _, _ = procCreateSolidBrush.Call(uintptr(darkScheme.ProgressBg))
	r.cache.brushProgressBg = syscall.Handle(h)
	h, _, _ = procCreateSolidBrush.Call(uintptr(darkScheme.ProgressFill))
	r.cache.brushProgressFill = syscall.Handle(h)
	h, _, _ = procCreateSolidBrush.Call(uintptr(darkScheme.AccentOrange))
	r.cache.brushAccentOrange = syscall.Handle(h)
	h, _, _ = procCreateSolidBrush.Call(uintptr(darkScheme.AccentRed))
	r.cache.brushAccentRed = syscall.Handle(h)
	h, _, _ = procCreateSolidBrush.Call(uintptr(darkScheme.AccentGreen))
	r.cache.brushAccentGreen = syscall.Handle(h)
	h, _, _ = procCreateSolidBrush.Call(uintptr(darkScheme.AccentPurple))
	r.cache.brushAccentPurple = syscall.Handle(h)

	ph, _, _ := procCreatePen.Call(PS_SOLID, 1, uintptr(darkScheme.CardBorder))
	r.cache.penCardBorder = syscall.Handle(ph)
	ph, _, _ = procCreatePen.Call(PS_SOLID, 1, uintptr(darkScheme.Divider))
	r.cache.penDivider = syscall.Handle(ph)

	return r
}

// createFont creates a GDI font with the given properties
func createFont(name string, height int, weight int) syscall.Handle {
	var lf LOGFONT
	lf.LfHeight = int32(height)
	lf.LfWeight = int32(weight)
	lf.LfCharSet = 1 // DEFAULT_CHARSET
	lf.LfQuality = 5 // CLEARTYPE_QUALITY

	nameRunes := []rune(name)
	for i := 0; i < len(nameRunes) && i < 31; i++ {
		lf.LfFaceName[i] = uint16(nameRunes[i])
	}

	h, _, _ := procCreateFontIndirect.Call(uintptr(unsafe.Pointer(&lf)))
	return syscall.Handle(h)
}

// renderResult holds all hit-test rectangles produced by a single Render call.
type renderResult struct {
	sessions   []sessionHitRect
	refreshBtn RECT // clickable area for the refresh button
}

// Render draws the entire HUD content and returns hit rects for click detection
func (r *Renderer) Render(hdc syscall.Handle, data *HUDData, cfg *Config, scrollY int, expandedSessions map[string]bool, refreshPressed bool) renderResult {
	w := int32(cfg.Width)
	h := int32(cfg.Height)

	// Create memory DC for double buffering (prevents flicker)
	memDC, _, _ := procCreateCompatibleDC.Call(uintptr(hdc))
	memBmp, _, _ := procCreateCompatibleBitmap.Call(uintptr(hdc), uintptr(w), uintptr(h))
	oldBmp, _, _ := procSelectObject.Call(memDC, memBmp)

	// Draw background
	r.drawBackground(syscall.Handle(memDC), w, h)

	// Set transparent text background mode
	procSetBkMode.Call(memDC, TRANSPARENT_BK)

	// Layout: vertical stack with padding
	y := int32(12)

	// Title bar with account info + refresh button
	var refreshBtn RECT
	y, refreshBtn = r.drawTitleBar(syscall.Handle(memDC), w, y, data, refreshPressed)
	y += 5

	// Divider
	r.drawDivider(syscall.Handle(memDC), 16, y, w-16)
	y += 10

	// Usage windows section (5h, daily, weekly)
	y = r.drawUsageWindows(syscall.Handle(memDC), w, y, data)
	y += 6

	// Divider
	r.drawDivider(syscall.Handle(memDC), 16, y, w-16)
	y += 10

	// Sessions section (scrollable)
	hitRects := r.drawSessionsSection(syscall.Handle(memDC), w, y, h, data, scrollY, expandedSessions)

	// Copy buffer to screen
	procBitBlt.Call(
		uintptr(hdc), 0, 0, uintptr(w), uintptr(h),
		memDC, 0, 0, SRCCOPY,
	)

	// Cleanup GDI objects
	procSelectObject.Call(memDC, oldBmp)
	procDeleteObject.Call(memBmp)
	procDeleteDC.Call(memDC)

	return renderResult{sessions: hitRects, refreshBtn: refreshBtn}
}

// drawBackground fills the window background and draws a border
func (r *Renderer) drawBackground(hdc syscall.Handle, w, h int32) {
	// Fill background using cached brush
	rect := RECT{0, 0, w, h}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rect)), uintptr(r.cache.brushBg))

	// Draw rounded border using cached pen
	oldPen, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(r.cache.penCardBorder))
	nullBrush, _, _ := procGetStockObject.Call(5) // HOLLOW_BRUSH
	oldBrush, _, _ := procSelectObject.Call(uintptr(hdc), nullBrush)
	procRoundRect.Call(uintptr(hdc), 0, 0, uintptr(w), uintptr(h), 12, 12)
	procSelectObject.Call(uintptr(hdc), oldPen)
	procSelectObject.Call(uintptr(hdc), oldBrush)
}

// drawTitleBar draws the "Claude HUD" title, plan badge, refresh button, and account.
// Returns the updated Y position and the refresh button hit rect.
func (r *Renderer) drawTitleBar(hdc syscall.Handle, w, y int32, data *HUDData, refreshPressed bool) (int32, RECT) {
	// Title text
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontTitle))
	procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.AccentPurple))
	r.drawTextStr(hdc, "Claude HUD", 16, y, w-120, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX)

	// Refresh button "↻" (between title and plan badge)
	refreshBtnX := w - 94
	refreshBtnY := y - 1
	refreshBtnW := int32(26)
	refreshBtnH := int32(24)
	refreshRect := RECT{refreshBtnX - 3, refreshBtnY - 2, refreshBtnX + refreshBtnW + 3, refreshBtnY + refreshBtnH + 2}

	if refreshPressed {
		// Pressed state: draw a rounded background + white icon using cached brush/pen
		oldBr, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(r.cache.brushAccentPurple))
		oldPn, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(r.cache.penCardBorder))
		procRoundRect.Call(uintptr(hdc),
			uintptr(refreshRect.Left), uintptr(refreshRect.Top),
			uintptr(refreshRect.Right), uintptr(refreshRect.Bottom), 6, 6)
		procSelectObject.Call(uintptr(hdc), oldBr)
		procSelectObject.Call(uintptr(hdc), oldPn)
		procSelectObject.Call(uintptr(hdc), uintptr(r.fontTitle))
		procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextPrimary))
	} else {
		// Normal state: cyan icon (larger font for clarity)
		procSelectObject.Call(uintptr(hdc), uintptr(r.fontTitle))
		procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.AccentCyan))
	}
	btnRect := RECT{refreshBtnX, refreshBtnY, refreshBtnX + refreshBtnW, refreshBtnY + refreshBtnH}
	p := utf16Ptr("↻")
	n := utf16Len("↻")
	procDrawText.Call(uintptr(hdc), uintptr(unsafe.Pointer(p)), uintptr(n),
		uintptr(unsafe.Pointer(&btnRect)), uintptr(DT_CENTER|DT_SINGLELINE|DT_NOPREFIX))

	// Plan badge on right side
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
	planColor := darkScheme.AccentPurple
	switch data.Usage.Plan {
	case PlanMax5x, PlanMax20x:
		planColor = darkScheme.AccentOrange
	case PlanTeam, PlanEnterprise:
		planColor = darkScheme.AccentBlue
	}
	procSetTextColor.Call(uintptr(hdc), uintptr(planColor))
	r.drawTextStr(hdc, string(data.Usage.Plan), w-62, y+2, 46, DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	y += 24

	// Account email (smaller, muted)
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
	procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
	r.drawTextStr(hdc, data.Account, 16, y, w-32, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	y += 16

	return y, refreshRect
}

// drawDivider draws a horizontal separator line
func (r *Renderer) drawDivider(hdc syscall.Handle, x, y, x2 int32) {
	oldPen, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(r.cache.penDivider))
	procMoveToEx.Call(uintptr(hdc), uintptr(x), uintptr(y), 0)
	procLineTo.Call(uintptr(hdc), uintptr(x2), uintptr(y))
	procSelectObject.Call(uintptr(hdc), oldPen)
}

// drawUsageWindows draws each rate limit window (5h, daily, weekly)
func (r *Renderer) drawUsageWindows(hdc syscall.Handle, w, y int32, data *HUDData) int32 {
	// Section title
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontBold))
	procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.AccentOrange))
	r.drawString(hdc, "사용량", 16, y)

	// Draw data source indicator dot after "사용량" header using cached brushes
	var dotBrush syscall.Handle
	var dotLabel string
	switch data.DataSource {
	case DataSourceAPI:
		dotBrush = r.cache.brushAccentGreen
	case DataSourceCache:
		dotBrush = r.cache.brushAccentOrange
		dotLabel = "캐시"
	default:
		dotBrush = r.cache.brushAccentRed
		dotLabel = "오프라인"
	}

	nullPen, _, _ := procGetStockObject.Call(8) // NULL_PEN
	oldDB, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(dotBrush))
	oldDP, _, _ := procSelectObject.Call(uintptr(hdc), nullPen)
	dotX := int32(90) // after header text
	dotY := y + 4
	procEllipse.Call(uintptr(hdc), uintptr(dotX), uintptr(dotY), uintptr(dotX+8), uintptr(dotY+8))
	procSelectObject.Call(uintptr(hdc), oldDB)
	procSelectObject.Call(uintptr(hdc), oldDP)

	if dotLabel != "" {
		procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
		procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
		r.drawTextStr(hdc, dotLabel, dotX+12, dotY-1, 60, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX)
	}

	// Model name on the right
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
	procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
	r.drawTextStr(hdc, data.Usage.Model, w-100, y+2, 84, DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX)
	y += 24

	// Draw each usage window
	for _, win := range data.Usage.Windows {
		y = r.drawUsageWindowCard(hdc, 16, y, w-32, win)
		y += 6
	}

	return y
}

// drawUsageWindowCard draws a single rate limit window with progress bar and reset timer
func (r *Renderer) drawUsageWindowCard(hdc syscall.Handle, x, y, w int32, win RateLimitWindow) int32 {
	// Card background using cached brush and pen
	cardH := int32(52)
	oldBrush, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(r.cache.brushCard))
	oldPen, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(r.cache.penCardBorder))
	procRoundRect.Call(uintptr(hdc), uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+cardH), 6, 6)
	procSelectObject.Call(uintptr(hdc), oldBrush)
	procSelectObject.Call(uintptr(hdc), oldPen)

	px := x + 10
	py := y + 6

	// Window label (left) + reset timer (right)
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontBold))
	procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextPrimary))
	r.drawString(hdc, win.Label, px, py)

	// Reset timer on the right
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
	resetStr := fmt.Sprintf("리셋: %s", FormatTimeRemaining(win.ResetAt))
	resetColor := darkScheme.AccentCyan
	if win.UsagePct > 0.85 {
		resetColor = darkScheme.AccentRed
	}
	procSetTextColor.Call(uintptr(hdc), uintptr(resetColor))
	r.drawTextStr(hdc, resetStr, px+w/2, py+2, w/2-20, DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX)
	py += 18

	// Usage percentage + token count (current / max)
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
	pctColor := darkScheme.TextSecondary
	if win.UsagePct > 0.9 {
		pctColor = darkScheme.AccentRed
	} else if win.UsagePct > 0.7 {
		pctColor = darkScheme.AccentOrange
	}
	procSetTextColor.Call(uintptr(hdc), uintptr(pctColor))
	pctStr := fmt.Sprintf("%.0f%% 사용", win.UsagePct*100)
	r.drawString(hdc, pctStr, px, py)
	procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
	var tokenStr string
	if win.TokensMax > 0 {
		tokenStr = fmt.Sprintf("%s / %s", FormatTokens(win.TokensUsed), FormatTokens(win.TokensMax))
	} else {
		tokenStr = FormatTokens(win.TokensUsed)
	}
	r.drawTextStr(hdc, tokenStr, px+w/2, py, w/2-20, DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX)
	py += 16

	// Progress bar
	r.drawProgressBar(hdc, px, py, w-20, 5, win.UsagePct)

	return y + cardH
}

// drawSessionsSection draws the scrollable sessions list and returns hit rects for click detection
func (r *Renderer) drawSessionsSection(hdc syscall.Handle, w, y, maxH int32, data *HUDData, scrollY int, expandedSessions map[string]bool) []sessionHitRect {
	var hitRects []sessionHitRect

	// Section title (fixed, not scrolled)
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontBold))
	procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.AccentBlue))
	r.drawString(hdc, fmt.Sprintf("활성 세션 (%d)", len(data.Sessions)), 16, y)
	y += 24

	// Clip region: prevent session cards from drawing above the title
	clipTop := y
	savedDC, _, _ := procSaveDC.Call(uintptr(hdc))
	procIntersectClipRect.Call(uintptr(hdc), 0, uintptr(clipTop), uintptr(w), uintptr(maxH))

	// Apply scroll offset to session cards
	y -= int32(scrollY)

	// Draw each session card
	for _, session := range data.Sessions {
		if y > maxH {
			break // Below visible area
		}
		cardTop := y
		expanded := expandedSessions[session.DirKey]
		if y+20 > clipTop-40 { // Within or near visible area
			y = r.drawSessionCard(hdc, 12, y, w-24, session, expanded)
			// Only record hit rect for visible (rendered) cards
			hitRects = append(hitRects, sessionHitRect{
				sessionKey: session.DirKey,
				top:        cardTop,
				bottom:     cardTop + 36, // header area height
			})
		} else {
			// Skip offscreen cards using appropriate height estimate
			if expanded {
				y += r.calcCardHeight(session, true)
			} else {
				y += int32(cardMinHeight)
			}
		}
		y += 8
	}

	// Restore DC (removes clip region)
	procRestoreDC.Call(uintptr(hdc), savedDC)

	return hitRects
}

// drawSessionCard draws a single session card with its agents
func (r *Renderer) drawSessionCard(hdc syscall.Handle, x, y, w int32, session Session, expanded bool) int32 {
	// Card background with rounded corners using cached brush and pen
	cardH := r.calcCardHeight(session, expanded)
	oldBrush, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(r.cache.brushCard))
	oldPen, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(r.cache.penCardBorder))
	procRoundRect.Call(uintptr(hdc), uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+cardH), 8, 8)
	procSelectObject.Call(uintptr(hdc), oldBrush)
	procSelectObject.Call(uintptr(hdc), oldPen)

	// Inner padding
	px := x + 12
	y += 8

	// Expand/collapse indicator
	indicator := "▶"
	if expanded {
		indicator = "▼"
	}
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
	procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
	r.drawString(hdc, indicator, px, y+1)

	// Status indicator dot + session header
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontBold))

	statusColor := darkScheme.AccentGreen
	statusChar := "●" // unicode filled circle
	if session.Status == StatusIdle {
		statusColor = darkScheme.TextMuted
		statusChar = "○" // unicode empty circle
	} else if session.Status == StatusPaused {
		statusColor = darkScheme.AccentOrange
		statusChar = "◐" // half-filled circle
	}
	procSetTextColor.Call(uintptr(hdc), uintptr(statusColor))
	r.drawString(hdc, statusChar, px+14, y)

	// Session name (show project name if available) - with ellipsis for overflow
	procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextPrimary))
	var header string
	if session.ProjectName != "" {
		header = fmt.Sprintf("#%d - %s", session.ID, session.ProjectName)
	} else {
		header = fmt.Sprintf("#%d - %s", session.ID, session.Type)
	}
	headerMaxW := w/2 - 42 // leave room for right-aligned time
	if headerMaxW < 60 {
		headerMaxW = 60
	}
	r.drawTextStr(hdc, header, px+30, y, headerMaxW, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)

	// Model + elapsed time (right-aligned)
	procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
	procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
	timeStr := fmt.Sprintf("%s  %s", session.Model, FormatDuration(session.StartTime))
	r.drawTextStr(hdc, timeStr, x+w/2, y+2, w/2-12, DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)

	y += 20

	// Only show details when expanded
	if expanded {
		// Active strategy/skill display
		if session.ActiveSkill != nil {
			procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
			procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.AccentOrange))
			strategyLabel := fmt.Sprintf("⚡ %s", session.ActiveSkill.Name)
			if session.ActiveSkill.Args != "" {
				strategyLabel += fmt.Sprintf(" %s", session.ActiveSkill.Args)
			}
			r.drawTextStr(hdc, strategyLabel, px+4, y, w-28, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			y += 18
		}

		if len(session.Agents) == 0 {
			// No agent delegations - show a status message
			procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
			procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
			r.drawTextStr(hdc, "직접 작업 중 (에이전트 위임 없음)", px+4, y, w-28, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			y += 16
		} else {
			// Agents with running/completed status
			for _, agent := range session.Agents {
				// Status indicator: green ● for running, gray ○ for completed
				procSelectObject.Call(uintptr(hdc), uintptr(r.fontBody))
				if agent.Status == StatusRunning {
					procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.AccentGreen))
					r.drawString(hdc, "●", px+2, y)
				} else {
					procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
					r.drawString(hdc, "○", px+2, y)
				}

				// Agent name with model
				if agent.Status == StatusRunning {
					procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextPrimary))
				} else {
					procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextSecondary))
				}
				agentLabel := agent.Name
				if agent.Model != "" {
					agentLabel += fmt.Sprintf(" (%s)", agent.Model)
				}
				r.drawTextStr(hdc, agentLabel, px+16, y, w-80, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)

				// Status label on the right
				procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
				if agent.Status == StatusRunning {
					procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.AccentGreen))
					r.drawTextStr(hdc, "실행 중", px+w-72, y+1, 60, DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX)
				} else {
					procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
					r.drawTextStr(hdc, "완료", px+w-72, y+1, 60, DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX)
				}
				y += 16

				// Agent task description (with ellipsis for overflow)
				if agent.Task != "" {
					procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
					procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
					r.drawTextStr(hdc, agent.Task, px+16, y, w-40, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
					y += 15
				}

				// Sub-agents
				for _, sub := range agent.SubAgents {
					subColor := darkScheme.AccentGreen
					if sub.Status == StatusIdle {
						subColor = darkScheme.TextMuted
					}
					procSetTextColor.Call(uintptr(hdc), uintptr(subColor))
					r.drawString(hdc, "▸", px+14, y)

					procSelectObject.Call(uintptr(hdc), uintptr(r.fontBody))
					procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextSecondary))
					r.drawTextStr(hdc, sub.Name, px+26, y, w-60, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)

					if sub.Task != "" {
						procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
						procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
						r.drawTextStr(hdc, sub.Task, px+26, y+14, w-60, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
					}
					y += 28
				}
			}
		}

		// Session message count
		if session.Messages > 0 {
			procSelectObject.Call(uintptr(hdc), uintptr(r.fontSmall))
			procSetTextColor.Call(uintptr(hdc), uintptr(darkScheme.TextMuted))
			r.drawTextStr(hdc, fmt.Sprintf("%d messages", session.Messages), px, y, w-24, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX)
			y += 14
		}
	}

	y += 4
	return y
}

// calcCardHeight calculates the required height for a session card.
// Uses shared constants from window.go to stay in sync with calcMaxScroll.
// When expanded is false, returns the compact header-only height.
func (r *Renderer) calcCardHeight(session Session, expanded bool) int32 {
	if !expanded {
		return int32(cardMinHeight)
	}
	h := int32(cardBaseHeight)
	// Strategy line
	if session.ActiveSkill != nil {
		h += int32(cardStrategyHeight)
	}
	if len(session.Agents) == 0 {
		h += int32(cardNoAgentsHeight)
	}
	for _, agent := range session.Agents {
		h += 16 // agent name line (always present)
		if agent.Task != "" {
			h += 15 // task description line
		}
		h += int32(len(agent.SubAgents)) * int32(cardSubAgentHeight)
	}
	if session.Messages > 0 {
		h += int32(cardMessagesHeight)
	}
	if h < int32(cardMinHeight) {
		h = int32(cardMinHeight)
	}
	return h
}

// drawProgressBar draws a colored progress bar
func (r *Renderer) drawProgressBar(hdc syscall.Handle, x, y, w, h int32, pct float64) {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	// Background track using cached brush
	bgRect := RECT{x, y, x + w, y + h}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&bgRect)), uintptr(r.cache.brushProgressBg))

	// Fill bar (color changes based on usage) using cached brushes
	fillW := int32(float64(w) * pct)
	if fillW > 0 {
		var fillBrush syscall.Handle
		if pct > 0.9 {
			fillBrush = r.cache.brushAccentRed
		} else if pct > 0.7 {
			fillBrush = r.cache.brushAccentOrange
		} else {
			fillBrush = r.cache.brushProgressFill
		}
		fillRect := RECT{x, y, x + fillW, y + h}
		procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&fillRect)), uintptr(fillBrush))
	}
}

// drawString draws a simple text string at the given position using TextOutW.
// Use this for short, non-clipped text.
func (r *Renderer) drawString(hdc syscall.Handle, s string, x, y int32) {
	p := utf16Ptr(s)
	n := utf16Len(s)
	procTextOut.Call(uintptr(hdc), uintptr(x), uintptr(y),
		uintptr(unsafe.Pointer(p)), uintptr(n))
}

// drawTextStr draws text within a bounding rectangle using DrawTextW.
// Supports alignment, ellipsis, and other formatting flags.
func (r *Renderer) drawTextStr(hdc syscall.Handle, s string, x, y, maxW int32, flags uint32) {
	p := utf16Ptr(s)
	n := utf16Len(s)
	rect := RECT{x, y, x + maxW, y + 20}
	procDrawText.Call(uintptr(hdc), uintptr(unsafe.Pointer(p)), uintptr(n),
		uintptr(unsafe.Pointer(&rect)), uintptr(flags))
}

// Cleanup releases all GDI font and cached brush/pen resources
func (r *Renderer) Cleanup() {
	fonts := []syscall.Handle{r.fontTitle, r.fontBody, r.fontSmall, r.fontBold, r.fontMono, r.fontIcon}
	for _, f := range fonts {
		if f != 0 {
			procDeleteObject.Call(uintptr(f))
		}
	}
	brushes := []syscall.Handle{
		r.cache.brushBg,
		r.cache.brushCard,
		r.cache.brushProgressBg,
		r.cache.brushProgressFill,
		r.cache.brushAccentOrange,
		r.cache.brushAccentRed,
		r.cache.brushAccentGreen,
		r.cache.brushAccentPurple,
	}
	for _, b := range brushes {
		if b != 0 {
			procDeleteObject.Call(uintptr(b))
		}
	}
	pens := []syscall.Handle{r.cache.penCardBorder, r.cache.penDivider}
	for _, p := range pens {
		if p != 0 {
			procDeleteObject.Call(uintptr(p))
		}
	}
}
