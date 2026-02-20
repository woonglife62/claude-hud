//go:build windows

package ui

import (
	"fmt"
	"sort"
	"syscall"
	"time"
	"unsafe"

	"claude-hud/internal/config"
	"claude-hud/internal/i18n"
	"claude-hud/internal/model"
)

// GDICache holds pre-created GDI brushes and pens that are reused across render frames
type GDICache struct {
	// Brushes
	BrushBg           syscall.Handle
	BrushCard         syscall.Handle
	BrushProgressBg   syscall.Handle
	BrushProgressFill syscall.Handle
	BrushAccentOrange syscall.Handle
	BrushAccentRed    syscall.Handle
	BrushAccentGreen  syscall.Handle
	BrushAccentPurple syscall.Handle
	// Pens
	PenCardBorder syscall.Handle
	PenDivider    syscall.Handle
}

// Renderer handles all GDI drawing operations
type Renderer struct {
	Hwnd      syscall.Handle
	Width     int
	Height    int
	DpiScale  float64
	FontTitle syscall.Handle
	FontBody  syscall.Handle
	FontSmall syscall.Handle
	FontBold  syscall.Handle
	FontMono  syscall.Handle
	FontIcon  syscall.Handle
	Cache     GDICache
	scheme    ColorScheme
}

// RenderResult holds all hit-test rectangles produced by a single Render call.
type RenderResult struct {
	Sessions   []SessionHitRect
	RefreshBtn RECT
}

// NewRenderer creates a new renderer for the given window at 96 DPI (1x scale)
func NewRenderer(hwnd syscall.Handle, width, height int) *Renderer {
	return NewRendererWithDPI(hwnd, width, height, 1.0)
}

// NewRendererWithDPI creates a new renderer with explicit DPI scaling
func NewRendererWithDPI(hwnd syscall.Handle, width, height int, dpiScale float64) *Renderer {
	if dpiScale <= 0 {
		dpiScale = 1.0
	}
	r := &Renderer{
		Hwnd:     hwnd,
		Width:    width,
		Height:   height,
		DpiScale: dpiScale,
		scheme:   DarkScheme,
	}

	// Create fonts scaled by DPI (negative height = point size via GDI convention)
	r.FontTitle = CreateFont("Segoe UI", int(float64(-18)*dpiScale), 700)
	r.FontBody = CreateFont("Segoe UI", int(float64(-13)*dpiScale), 400)
	r.FontSmall = CreateFont("Segoe UI", int(float64(-11)*dpiScale), 400)
	r.FontBold = CreateFont("Segoe UI Semibold", int(float64(-13)*dpiScale), 600)
	r.FontMono = CreateFont("Cascadia Code", int(float64(-12)*dpiScale), 400)
	r.FontIcon = CreateFont("Segoe UI", int(float64(-12)*dpiScale), 400)

	r.buildCache()
	return r
}

// buildCache creates GDI brushes and pens from the current scheme.
func (r *Renderer) buildCache() {
	h, _, _ := ProcCreateSolidBrush.Call(uintptr(r.scheme.BgColor))
	r.Cache.BrushBg = syscall.Handle(h)
	h, _, _ = ProcCreateSolidBrush.Call(uintptr(r.scheme.CardColor))
	r.Cache.BrushCard = syscall.Handle(h)
	h, _, _ = ProcCreateSolidBrush.Call(uintptr(r.scheme.ProgressBg))
	r.Cache.BrushProgressBg = syscall.Handle(h)
	h, _, _ = ProcCreateSolidBrush.Call(uintptr(r.scheme.ProgressFill))
	r.Cache.BrushProgressFill = syscall.Handle(h)
	h, _, _ = ProcCreateSolidBrush.Call(uintptr(r.scheme.AccentOrange))
	r.Cache.BrushAccentOrange = syscall.Handle(h)
	h, _, _ = ProcCreateSolidBrush.Call(uintptr(r.scheme.AccentRed))
	r.Cache.BrushAccentRed = syscall.Handle(h)
	h, _, _ = ProcCreateSolidBrush.Call(uintptr(r.scheme.AccentGreen))
	r.Cache.BrushAccentGreen = syscall.Handle(h)
	h, _, _ = ProcCreateSolidBrush.Call(uintptr(r.scheme.AccentPurple))
	r.Cache.BrushAccentPurple = syscall.Handle(h)

	ph, _, _ := ProcCreatePen.Call(PS_SOLID, 1, uintptr(r.scheme.CardBorder))
	r.Cache.PenCardBorder = syscall.Handle(ph)
	ph, _, _ = ProcCreatePen.Call(PS_SOLID, 1, uintptr(r.scheme.Divider))
	r.Cache.PenDivider = syscall.Handle(ph)
}

// RebuildCache destroys existing GDI cache objects and recreates them from the new scheme.
func (r *Renderer) RebuildCache(scheme ColorScheme) {
	brushes := []syscall.Handle{
		r.Cache.BrushBg, r.Cache.BrushCard, r.Cache.BrushProgressBg,
		r.Cache.BrushProgressFill, r.Cache.BrushAccentOrange, r.Cache.BrushAccentRed,
		r.Cache.BrushAccentGreen, r.Cache.BrushAccentPurple,
	}
	for _, b := range brushes {
		if b != 0 {
			ProcDeleteObject.Call(uintptr(b))
		}
	}
	pens := []syscall.Handle{r.Cache.PenCardBorder, r.Cache.PenDivider}
	for _, p := range pens {
		if p != 0 {
			ProcDeleteObject.Call(uintptr(p))
		}
	}
	r.scheme = scheme
	r.buildCache()
}

// CreateFont creates a GDI font with the given properties
func CreateFont(name string, height int, weight int) syscall.Handle {
	var lf LOGFONT
	lf.LfHeight = int32(height)
	lf.LfWeight = int32(weight)
	lf.LfCharSet = 1 // DEFAULT_CHARSET
	lf.LfQuality = 5 // CLEARTYPE_QUALITY

	nameRunes := []rune(name)
	for i := 0; i < len(nameRunes) && i < 31; i++ {
		lf.LfFaceName[i] = uint16(nameRunes[i])
	}

	h, _, _ := ProcCreateFontIndirect.Call(uintptr(unsafe.Pointer(&lf)))
	return syscall.Handle(h)
}

// Render draws the entire HUD content and returns hit rects for click detection
func (r *Renderer) Render(hdc syscall.Handle, data *model.HUDData, cfg *config.Config, scrollY int, expandedSessions map[string]bool, refreshPressed bool) RenderResult {
	w := int32(cfg.Width)
	h := int32(cfg.Height)

	// Create memory DC for double buffering (prevents flicker)
	memDC, _, _ := ProcCreateCompatibleDC.Call(uintptr(hdc))
	memBmp, _, _ := ProcCreateCompatibleBitmap.Call(uintptr(hdc), uintptr(w), uintptr(h))
	oldBmp, _, _ := ProcSelectObject.Call(memDC, memBmp)

	// Draw background
	r.drawBackground(syscall.Handle(memDC), w, h)

	// Set transparent text background mode
	ProcSetBkMode.Call(memDC, TRANSPARENT_BK)

	// Layout: vertical stack with padding
	y := r.scaled(12)

	// Title bar with account info + refresh button
	var refreshBtn RECT
	y, refreshBtn = r.drawTitleBar(syscall.Handle(memDC), w, y, data, refreshPressed)
	y += r.scaled(5)

	// Divider
	r.drawDivider(syscall.Handle(memDC), r.scaled(16), y, w-r.scaled(16))
	y += r.scaled(10)

	// Usage windows section (5h, daily, weekly)
	y = r.drawUsageWindows(syscall.Handle(memDC), w, y, data)
	y += r.scaled(6)

	// Divider
	r.drawDivider(syscall.Handle(memDC), r.scaled(16), y, w-r.scaled(16))
	y += r.scaled(10)

	// Sessions section (scrollable) - skipped in compact mode
	var hitRects []SessionHitRect
	if !cfg.CompactMode {
		hitRects = r.drawSessionsSection(syscall.Handle(memDC), w, y, h, data, scrollY, expandedSessions)
	}

	// Copy buffer to screen
	ProcBitBlt.Call(
		uintptr(hdc), 0, 0, uintptr(w), uintptr(h),
		memDC, 0, 0, SRCCOPY,
	)

	// Cleanup GDI objects
	ProcSelectObject.Call(memDC, oldBmp)
	ProcDeleteObject.Call(memBmp)
	ProcDeleteDC.Call(memDC)

	return RenderResult{Sessions: hitRects, RefreshBtn: refreshBtn}
}

// scaled returns v multiplied by the DPI scale factor
func (r *Renderer) scaled(v int32) int32 {
	return int32(float64(v) * r.DpiScale)
}

// drawBackground fills the window background and draws a border
func (r *Renderer) drawBackground(hdc syscall.Handle, w, h int32) {
	rect := RECT{0, 0, w, h}
	ProcFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rect)), uintptr(r.Cache.BrushBg))

	oldPen, _, _ := ProcSelectObject.Call(uintptr(hdc), uintptr(r.Cache.PenCardBorder))
	nullBrush, _, _ := ProcGetStockObject.Call(5) // HOLLOW_BRUSH
	oldBrush, _, _ := ProcSelectObject.Call(uintptr(hdc), nullBrush)
	ProcRoundRect.Call(uintptr(hdc), 0, 0, uintptr(w), uintptr(h), 12, 12)
	ProcSelectObject.Call(uintptr(hdc), oldPen)
	ProcSelectObject.Call(uintptr(hdc), oldBrush)
}

// drawTitleBar draws the "Claude HUD" title, plan badge, refresh button, and account.
func (r *Renderer) drawTitleBar(hdc syscall.Handle, w, y int32, data *model.HUDData, refreshPressed bool) (int32, RECT) {
	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontTitle))
	ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.AccentPurple))
	r.drawTextStr(hdc, "Claude HUD", r.scaled(16), y, w-r.scaled(120), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX)

	// Refresh button
	refreshBtnX := w - r.scaled(94)
	refreshBtnY := y - r.scaled(1)
	refreshBtnW := r.scaled(26)
	refreshBtnH := r.scaled(24)
	refreshRect := RECT{refreshBtnX - r.scaled(3), refreshBtnY - r.scaled(2), refreshBtnX + refreshBtnW + r.scaled(3), refreshBtnY + refreshBtnH + r.scaled(2)}

	if refreshPressed {
		oldBr, _, _ := ProcSelectObject.Call(uintptr(hdc), uintptr(r.Cache.BrushAccentPurple))
		oldPn, _, _ := ProcSelectObject.Call(uintptr(hdc), uintptr(r.Cache.PenCardBorder))
		ProcRoundRect.Call(uintptr(hdc),
			uintptr(refreshRect.Left), uintptr(refreshRect.Top),
			uintptr(refreshRect.Right), uintptr(refreshRect.Bottom), uintptr(r.scaled(6)), uintptr(r.scaled(6)))
		ProcSelectObject.Call(uintptr(hdc), oldBr)
		ProcSelectObject.Call(uintptr(hdc), oldPn)
		ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontTitle))
		ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextPrimary))
	} else {
		ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontTitle))
		ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.AccentCyan))
	}
	btnRect := RECT{refreshBtnX, refreshBtnY, refreshBtnX + refreshBtnW, refreshBtnY + refreshBtnH}
	p := Utf16Ptr("\u21BB")
	n := Utf16Len("\u21BB")
	ProcDrawText.Call(uintptr(hdc), uintptr(unsafe.Pointer(p)), uintptr(n),
		uintptr(unsafe.Pointer(&btnRect)), uintptr(DT_CENTER|DT_SINGLELINE|DT_NOPREFIX))

	// Plan badge on right side
	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
	planColor := r.scheme.AccentPurple
	switch data.Usage.Plan {
	case model.PlanMax5x, model.PlanMax20x:
		planColor = r.scheme.AccentOrange
	case model.PlanTeam, model.PlanEnterprise:
		planColor = r.scheme.AccentBlue
	}
	ProcSetTextColor.Call(uintptr(hdc), uintptr(planColor))
	r.drawTextStr(hdc, string(data.Usage.Plan), w-r.scaled(62), y+r.scaled(2), r.scaled(46), DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	y += r.scaled(24)

	// Account email
	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
	ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
	r.drawTextStr(hdc, data.Account, r.scaled(16), y, w-r.scaled(32), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	y += r.scaled(16)

	return y, refreshRect
}

// drawDivider draws a horizontal separator line
func (r *Renderer) drawDivider(hdc syscall.Handle, x, y, x2 int32) {
	oldPen, _, _ := ProcSelectObject.Call(uintptr(hdc), uintptr(r.Cache.PenDivider))
	ProcMoveToEx.Call(uintptr(hdc), uintptr(x), uintptr(y), 0)
	ProcLineTo.Call(uintptr(hdc), uintptr(x2), uintptr(y))
	ProcSelectObject.Call(uintptr(hdc), oldPen)
}

// drawUsageWindows draws each rate limit window (5h, daily, weekly)
func (r *Renderer) drawUsageWindows(hdc syscall.Handle, w, y int32, data *model.HUDData) int32 {
	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontBold))
	ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.AccentOrange))
	r.drawString(hdc, i18n.T.Usage, r.scaled(16), y)

	// Draw data source indicator dot
	var dotBrush syscall.Handle
	var dotLabel string
	if data.TokenExpired {
		dotBrush = r.Cache.BrushAccentRed
		dotLabel = i18n.T.AuthExpired
	} else {
		switch data.DataSource {
		case model.DataSourceAPI:
			dotBrush = r.Cache.BrushAccentGreen
		case model.DataSourceCache:
			dotBrush = r.Cache.BrushAccentOrange
			dotLabel = i18n.T.Cache
		default:
			dotBrush = r.Cache.BrushAccentRed
			dotLabel = i18n.T.Offline
		}
	}

	nullPen, _, _ := ProcGetStockObject.Call(8) // NULL_PEN
	oldDB, _, _ := ProcSelectObject.Call(uintptr(hdc), uintptr(dotBrush))
	oldDP, _, _ := ProcSelectObject.Call(uintptr(hdc), nullPen)
	dotX := r.scaled(90)
	dotY := y + r.scaled(4)
	dotSize := r.scaled(8)
	ProcEllipse.Call(uintptr(hdc), uintptr(dotX), uintptr(dotY), uintptr(dotX+dotSize), uintptr(dotY+dotSize))
	ProcSelectObject.Call(uintptr(hdc), oldDB)
	ProcSelectObject.Call(uintptr(hdc), oldDP)

	if dotLabel != "" {
		ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
		ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
		r.drawTextStr(hdc, dotLabel, dotX+r.scaled(12), dotY-r.scaled(1), r.scaled(60), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX)
	}

	// Model name on the right
	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
	ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
	r.drawTextStr(hdc, data.Usage.Model, w-r.scaled(100), y+r.scaled(2), r.scaled(84), DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX)
	y += r.scaled(24)

	for _, win := range data.Usage.Windows {
		y = r.drawUsageWindowCard(hdc, r.scaled(16), y, w-r.scaled(32), win)
		y += r.scaled(6)
	}

	// Model breakdown (Issue #7): show per-model token counts if available
	if len(data.Usage.ModelBreakdown) > 0 {
		models := make([]string, 0, len(data.Usage.ModelBreakdown))
		for m := range data.Usage.ModelBreakdown {
			models = append(models, m)
		}
		sort.Strings(models)

		ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
		for _, m := range models {
			tokens := data.Usage.ModelBreakdown[m]
			label := fmt.Sprintf("%s: %s", m, model.FormatTokens(int(tokens)))
			ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
			r.drawTextStr(hdc, label, r.scaled(20), y, w-r.scaled(40), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			y += r.scaled(14)
		}
		y += r.scaled(2)
	}

	return y
}

// drawUsageWindowCard draws a single rate limit window with progress bar and reset timer
func (r *Renderer) drawUsageWindowCard(hdc syscall.Handle, x, y, w int32, win model.RateLimitWindow) int32 {
	cardH := r.scaled(52)
	oldBrush, _, _ := ProcSelectObject.Call(uintptr(hdc), uintptr(r.Cache.BrushCard))
	oldPen, _, _ := ProcSelectObject.Call(uintptr(hdc), uintptr(r.Cache.PenCardBorder))
	ProcRoundRect.Call(uintptr(hdc), uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+cardH), uintptr(r.scaled(6)), uintptr(r.scaled(6)))
	ProcSelectObject.Call(uintptr(hdc), oldBrush)
	ProcSelectObject.Call(uintptr(hdc), oldPen)

	px := x + r.scaled(10)
	py := y + r.scaled(6)

	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontBold))
	ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextPrimary))
	r.drawString(hdc, win.Label, px, py)

	// Reset timer on the right
	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
	resetStr := fmt.Sprintf("%s %s", i18n.T.Reset, model.FormatTimeRemaining(win.ResetAt))
	resetColor := r.scheme.AccentCyan
	if win.UsagePct > 0.85 {
		resetColor = r.scheme.AccentRed
	}
	ProcSetTextColor.Call(uintptr(hdc), uintptr(resetColor))
	r.drawTextStr(hdc, resetStr, px+w/2, py+r.scaled(2), w/2-r.scaled(20), DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX)
	py += r.scaled(18)

	// Usage percentage + token count
	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
	pctColor := r.scheme.TextSecondary
	if win.UsagePct > 0.9 {
		pctColor = r.scheme.AccentRed
	} else if win.UsagePct > 0.7 {
		pctColor = r.scheme.AccentOrange
	}
	ProcSetTextColor.Call(uintptr(hdc), uintptr(pctColor))
	pctStr := fmt.Sprintf(i18n.T.UsagePct, win.UsagePct*100)
	r.drawString(hdc, pctStr, px, py)
	ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
	var tokenStr string
	if win.TokensMax > 0 {
		tokenStr = fmt.Sprintf("%s / %s", model.FormatTokens(win.TokensUsed), model.FormatTokens(win.TokensMax))
	} else {
		tokenStr = model.FormatTokens(win.TokensUsed)
	}
	r.drawTextStr(hdc, tokenStr, px+w/2, py, w/2-r.scaled(20), DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX)
	py += r.scaled(16)

	// Progress bar
	r.drawProgressBar(hdc, px, py, w-r.scaled(20), r.scaled(5), win.UsagePct)

	return y + cardH
}

// drawSessionsSection draws the scrollable sessions list and returns hit rects for click detection
func (r *Renderer) drawSessionsSection(hdc syscall.Handle, w, y, maxH int32, data *model.HUDData, scrollY int, expandedSessions map[string]bool) []SessionHitRect {
	var hitRects []SessionHitRect

	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontBold))
	ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.AccentBlue))
	r.drawString(hdc, fmt.Sprintf("%s %s (%d)", i18n.T.Active, i18n.T.Sessions, len(data.Sessions)), r.scaled(16), y)
	y += r.scaled(24)

	clipTop := y
	savedDC, _, _ := ProcSaveDC.Call(uintptr(hdc))
	ProcIntersectClipRect.Call(uintptr(hdc), 0, uintptr(clipTop), uintptr(w), uintptr(maxH))

	y -= int32(scrollY)

	for _, session := range data.Sessions {
		if y > maxH {
			break
		}
		cardTop := y
		expanded := expandedSessions[session.DirKey]
		if y+r.scaled(20) > clipTop-r.scaled(40) {
			y = r.drawSessionCard(hdc, r.scaled(12), y, w-r.scaled(24), session, expanded, false)
			hitRects = append(hitRects, SessionHitRect{
				SessionKey: session.DirKey,
				Top:        cardTop,
				Bottom:     cardTop + r.scaled(36),
			})
		} else {
			if expanded {
				y += r.CalcCardHeight(session, true)
			} else {
				y += int32(CardMinHeight)
			}
		}
		y += r.scaled(8)
	}

	// Draw recently closed sessions section
	if len(data.ClosedSessions) > 0 && y < maxH {
		ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontBold))
		ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
		closedLabel := fmt.Sprintf("\uC885\uB8CC\uB428 (%d)", len(data.ClosedSessions)) // "종료됨 (N)"
		r.drawString(hdc, closedLabel, r.scaled(16), y)
		y += r.scaled(20)

		for _, session := range data.ClosedSessions {
			if y > maxH {
				break
			}
			if y+r.scaled(20) > clipTop-r.scaled(40) {
				y = r.drawSessionCard(hdc, r.scaled(12), y, w-r.scaled(24), session, false, true)
			} else {
				y += int32(CardMinHeight)
			}
			y += r.scaled(8)
		}
	}

	ProcRestoreDC.Call(uintptr(hdc), savedDC)

	return hitRects
}

// drawSessionCard draws a single session card with its agents.
// isClosed=true renders with muted styling for recently closed sessions.
func (r *Renderer) drawSessionCard(hdc syscall.Handle, x, y, w int32, session model.Session, expanded bool, isClosed bool) int32 {
	cardH := r.CalcCardHeight(session, expanded)
	oldBrush, _, _ := ProcSelectObject.Call(uintptr(hdc), uintptr(r.Cache.BrushCard))
	oldPen, _, _ := ProcSelectObject.Call(uintptr(hdc), uintptr(r.Cache.PenCardBorder))
	ProcRoundRect.Call(uintptr(hdc), uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+cardH), uintptr(r.scaled(8)), uintptr(r.scaled(8)))
	ProcSelectObject.Call(uintptr(hdc), oldBrush)
	ProcSelectObject.Call(uintptr(hdc), oldPen)

	px := x + r.scaled(12)
	y += r.scaled(8)

	if isClosed {
		// Closed session: muted header + "종료 Xm ago" label
		ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontBold))
		ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
		var header string
		if session.ProjectName != "" {
			header = fmt.Sprintf("#%d - %s", session.ID, session.ProjectName)
		} else {
			header = fmt.Sprintf("#%d - %s", session.ID, session.Type)
		}
		headerMaxW := w/2 - r.scaled(30)
		if headerMaxW < r.scaled(60) {
			headerMaxW = r.scaled(60)
		}
		r.drawTextStr(hdc, header, px+r.scaled(14), y, headerMaxW, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)

		ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
		ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
		agoStr := fmt.Sprintf("\uC885\uB8CC %s ago", formatAgo(session.ClosedAt)) // "종료 Xm ago"
		r.drawTextStr(hdc, agoStr, x+w/2, y+r.scaled(2), w/2-r.scaled(12), DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		y += r.scaled(20)

		// Detail: model + message count
		ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
		ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
		detailStr := session.Model
		if session.Messages > 0 {
			detailStr += fmt.Sprintf("  %d msgs", session.Messages)
		}
		r.drawTextStr(hdc, detailStr, px+r.scaled(14), y, w-r.scaled(28), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		y += r.scaled(14)

		y += r.scaled(4)
		return y
	}

	// Active session card

	// Expand/collapse indicator
	indicator := "\u25B6" // "▶"
	if expanded {
		indicator = "\u25BC" // "▼"
	}
	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
	ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
	r.drawString(hdc, indicator, px, y+r.scaled(1))

	// Status indicator dot + session header
	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontBold))

	statusColor := r.scheme.AccentGreen
	statusChar := "\u25CF" // "●"
	if session.Status == model.StatusIdle {
		statusColor = r.scheme.TextMuted
		statusChar = "\u25CB" // "○"
	} else if session.Status == model.StatusPaused {
		statusColor = r.scheme.AccentOrange
		statusChar = "\u25D0" // "◐"
	}
	ProcSetTextColor.Call(uintptr(hdc), uintptr(statusColor))
	r.drawString(hdc, statusChar, px+r.scaled(14), y)

	// Session name
	ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextPrimary))
	var header string
	if session.ProjectName != "" {
		header = fmt.Sprintf("#%d - %s", session.ID, session.ProjectName)
	} else {
		header = fmt.Sprintf("#%d - %s", session.ID, session.Type)
	}
	headerMaxW := w/2 - r.scaled(42)
	if headerMaxW < r.scaled(60) {
		headerMaxW = r.scaled(60)
	}
	r.drawTextStr(hdc, header, px+r.scaled(30), y, headerMaxW, DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)

	// Model + elapsed time (right-aligned)
	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
	ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
	timeStr := fmt.Sprintf("%s  %s", session.Model, model.FormatDuration(session.StartTime))
	r.drawTextStr(hdc, timeStr, x+w/2, y+r.scaled(2), w/2-r.scaled(12), DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)

	y += r.scaled(20)

	// Issue #14: Enhanced detail lines (always visible below header)
	ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
	ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
	if session.ProjectName != "" {
		r.drawTextStr(hdc, session.ProjectName, px+r.scaled(14), y, w/2-r.scaled(20), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
	}
	if session.Messages > 0 {
		r.drawTextStr(hdc, fmt.Sprintf("%d msgs", session.Messages), x+w/2, y, w/2-r.scaled(12), DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX)
	}
	y += r.scaled(14)

	// Active skill compact line (always visible)
	if session.ActiveSkill != nil {
		ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
		ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.AccentOrange))
		skillStr := fmt.Sprintf("\u26A1 %s", session.ActiveSkill.Name)
		if session.ActiveSkill.Args != "" {
			skillStr += fmt.Sprintf(" %s", session.ActiveSkill.Args)
		}
		r.drawTextStr(hdc, skillStr, px+r.scaled(14), y, w-r.scaled(28), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
		y += r.scaled(14)
	}

	if expanded {
		if len(session.Agents) == 0 {
			ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
			ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
			r.drawTextStr(hdc, i18n.T.DirectWork, px+r.scaled(4), y, w-r.scaled(28), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
			y += r.scaled(16)
		} else {
			for _, agent := range session.Agents {
				ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontBody))
				if agent.Status == model.StatusRunning {
					ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.AccentGreen))
					r.drawString(hdc, "\u25CF", px+r.scaled(2), y)
				} else {
					ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
					r.drawString(hdc, "\u25CB", px+r.scaled(2), y)
				}

				if agent.Status == model.StatusRunning {
					ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextPrimary))
				} else {
					ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextSecondary))
				}
				agentLabel := agent.Name
				if agent.Model != "" {
					agentLabel += fmt.Sprintf(" (%s)", agent.Model)
				}
				r.drawTextStr(hdc, agentLabel, px+r.scaled(16), y, w-r.scaled(80), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)

				ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
				if agent.Status == model.StatusRunning {
					ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.AccentGreen))
					r.drawTextStr(hdc, i18n.T.AgentRunning, px+w-r.scaled(72), y+r.scaled(1), r.scaled(60), DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX)
				} else {
					ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
					r.drawTextStr(hdc, i18n.T.AgentDone, px+w-r.scaled(72), y+r.scaled(1), r.scaled(60), DT_RIGHT|DT_SINGLELINE|DT_NOPREFIX)
				}
				y += r.scaled(16)

				if agent.Task != "" {
					ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
					ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
					r.drawTextStr(hdc, agent.Task, px+r.scaled(16), y, w-r.scaled(40), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
					y += r.scaled(15)
				}

				for _, sub := range agent.SubAgents {
					subColor := r.scheme.AccentGreen
					if sub.Status == model.StatusIdle {
						subColor = r.scheme.TextMuted
					}
					ProcSetTextColor.Call(uintptr(hdc), uintptr(subColor))
					r.drawString(hdc, "\u25B8", px+r.scaled(14), y)

					ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontBody))
					ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextSecondary))
					r.drawTextStr(hdc, sub.Name, px+r.scaled(26), y, w-r.scaled(60), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)

					if sub.Task != "" {
						ProcSetTextColor.Call(uintptr(hdc), uintptr(r.scheme.TextMuted))
						ProcSelectObject.Call(uintptr(hdc), uintptr(r.FontSmall))
						r.drawTextStr(hdc, sub.Task, px+r.scaled(26), y+r.scaled(14), w-r.scaled(60), DT_LEFT|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)
					}
					y += r.scaled(28)
				}
			}
		}
	}

	y += r.scaled(4)
	return y
}

// formatAgo formats duration since t as a short human-readable string.
func formatAgo(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

// CalcCardHeight calculates the required height for a session card.
func (r *Renderer) CalcCardHeight(session model.Session, expanded bool) int32 {
	// Always shown: header row + detail line
	detailLines := int32(14) // project + msgs line
	if session.ActiveSkill != nil {
		detailLines += 14 // skill line
	}

	if !expanded {
		return int32(CardMinHeight) + detailLines
	}
	h := int32(CardBaseHeight) + detailLines
	if len(session.Agents) == 0 {
		h += int32(CardNoAgentsHeight)
	}
	for _, agent := range session.Agents {
		h += 16
		if agent.Task != "" {
			h += 15
		}
		h += int32(len(agent.SubAgents)) * int32(CardSubAgentHeight)
	}
	if h < int32(CardMinHeight)+detailLines {
		h = int32(CardMinHeight) + detailLines
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

	bgRect := RECT{x, y, x + w, y + h}
	ProcFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&bgRect)), uintptr(r.Cache.BrushProgressBg))

	fillW := int32(float64(w) * pct)
	if fillW > 0 {
		var fillBrush syscall.Handle
		if pct > 0.9 {
			fillBrush = r.Cache.BrushAccentRed
		} else if pct > 0.7 {
			fillBrush = r.Cache.BrushAccentOrange
		} else {
			fillBrush = r.Cache.BrushProgressFill
		}
		fillRect := RECT{x, y, x + fillW, y + h}
		ProcFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&fillRect)), uintptr(fillBrush))
	}
}

// drawString draws a simple text string at the given position using TextOutW.
func (r *Renderer) drawString(hdc syscall.Handle, s string, x, y int32) {
	p := Utf16Ptr(s)
	n := Utf16Len(s)
	ProcTextOut.Call(uintptr(hdc), uintptr(x), uintptr(y),
		uintptr(unsafe.Pointer(p)), uintptr(n))
}

// drawTextStr draws text within a bounding rectangle using DrawTextW.
func (r *Renderer) drawTextStr(hdc syscall.Handle, s string, x, y, maxW int32, flags uint32) {
	p := Utf16Ptr(s)
	n := Utf16Len(s)
	rect := RECT{x, y, x + maxW, y + 20}
	ProcDrawText.Call(uintptr(hdc), uintptr(unsafe.Pointer(p)), uintptr(n),
		uintptr(unsafe.Pointer(&rect)), uintptr(flags))
}

// Cleanup releases all GDI font and cached brush/pen resources
func (r *Renderer) Cleanup() {
	fonts := []syscall.Handle{r.FontTitle, r.FontBody, r.FontSmall, r.FontBold, r.FontMono, r.FontIcon}
	for _, f := range fonts {
		if f != 0 {
			ProcDeleteObject.Call(uintptr(f))
		}
	}
	brushes := []syscall.Handle{
		r.Cache.BrushBg,
		r.Cache.BrushCard,
		r.Cache.BrushProgressBg,
		r.Cache.BrushProgressFill,
		r.Cache.BrushAccentOrange,
		r.Cache.BrushAccentRed,
		r.Cache.BrushAccentGreen,
		r.Cache.BrushAccentPurple,
	}
	for _, b := range brushes {
		if b != 0 {
			ProcDeleteObject.Call(uintptr(b))
		}
	}
	pens := []syscall.Handle{r.Cache.PenCardBorder, r.Cache.PenDivider}
	for _, p := range pens {
		if p != 0 {
			ProcDeleteObject.Call(uintptr(p))
		}
	}
}
