//go:build !windows

package ui

import (
	"fmt"
	"image/color"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"claude-hud/internal/config"
	"claude-hud/internal/data"
	"claude-hud/internal/i18n"
	"claude-hud/internal/model"
	"claude-hud/internal/platform"
)

// HUDApp is the cross-platform Fyne-based HUD application.
type HUDApp struct {
	fyneApp  fyne.App
	window   fyne.Window
	cfg      *config.Config
	data     *model.HUDData
	colors   FyneColors
	expanded map[int]bool
	visible  bool
	mu       sync.Mutex
	notify   notifyStateFyne
}

// notifyStateFyne tracks which threshold notifications have fired.
type notifyStateFyne struct {
	fiveHourT1Fired   bool
	fiveHourT2Fired   bool
	weeklyT1Fired     bool
	weeklyT2Fired     bool
	lastFiveHourReset time.Time
	lastWeeklyReset   time.Time
}

// RunApp creates and runs the Fyne HUD application.
// This is the main entry point for macOS and Linux.
func RunApp(cfg *config.Config, hudData *model.HUDData) {
	a := app.NewWithID("com.claude-hud")
	w := a.NewWindow("Claude HUD")

	colors := FyneDarkColors
	if cfg.ThemeMode == "light" {
		colors = FyneLightColors
	}

	h := &HUDApp{
		fyneApp:  a,
		window:   w,
		cfg:      cfg,
		data:     hudData,
		colors:   colors,
		expanded: make(map[int]bool),
		visible:  true,
	}

	a.Settings().SetTheme(&hudTheme{
		dark:   cfg.ThemeMode != "light",
		colors: colors,
	})

	h.rebuildUI()
	w.Resize(fyne.NewSize(float32(cfg.Width), float32(cfg.Height)))
	w.SetCloseIntercept(func() {
		w.Hide()
		h.visible = false
	})

	SetupTray(h)
	go h.refreshLoop()

	platform.Log("Fyne app starting")
	w.ShowAndRun()
}

// rebuildUI reconstructs the entire widget tree from current data.
func (h *HUDApp) rebuildUI() {
	h.mu.Lock()
	content := h.buildContent()
	bgColor := h.colors.BgColor
	h.mu.Unlock()

	bg := canvas.NewRectangle(bgColor)
	scroll := container.NewVScroll(content)
	h.window.SetContent(container.NewStack(bg, scroll))
}

// buildContent creates the full VBox layout with all HUD sections.
func (h *HUDApp) buildContent() *fyne.Container {
	sections := []fyne.CanvasObject{
		h.buildTitleSection(),
		h.buildUsageSection(),
	}
	if !h.cfg.CompactMode {
		sections = append(sections, h.buildSessionsSection())
	}
	return container.NewVBox(sections...)
}

// buildTitleSection creates the header: app name, plan badge, refresh button, account.
func (h *HUDApp) buildTitleSection() fyne.CanvasObject {
	title := canvas.NewText("Claude HUD", h.colors.TextPrimary)
	title.TextSize = 18
	title.TextStyle = fyne.TextStyle{Bold: true}

	planBadge := h.makeBadge(string(h.data.Usage.Plan), h.colors.AccentPurple)

	refreshBtn := widget.NewButton("↻", func() {
		go h.triggerRefresh()
	})
	refreshBtn.Importance = widget.LowImportance

	titleRow := container.NewHBox(title, planBadge, layout.NewSpacer(), refreshBtn)

	account := canvas.NewText(h.data.Account, h.colors.TextMuted)
	account.TextSize = 11

	return container.NewPadded(container.NewVBox(titleRow, account))
}

// buildUsageSection creates usage cards with progress bars and model breakdown.
func (h *HUDApp) buildUsageSection() fyne.CanvasObject {
	header := canvas.NewText(i18n.T.Usage, h.colors.TextPrimary)
	header.TextSize = 14
	header.TextStyle = fyne.TextStyle{Bold: true}

	sourceText, sourceColor := h.dataSourceLabel()
	source := canvas.NewText(sourceText, sourceColor)
	source.TextSize = 11

	headerRow := container.NewHBox(header, layout.NewSpacer(), source)
	items := []fyne.CanvasObject{container.NewPadded(headerRow)}

	if h.data.Usage.TrendArrow != "" {
		trend := canvas.NewText("Trend: "+h.data.Usage.TrendArrow, h.colors.AccentCyan)
		trend.TextSize = 11
		items = append(items, container.NewPadded(trend))
	}

	for _, w := range h.data.Usage.Windows {
		items = append(items, h.buildUsageCard(w))
	}

	if len(h.data.Usage.ModelBreakdown) > 0 {
		items = append(items, h.buildModelBreakdown())
	}

	return container.NewVBox(items...)
}

// dataSourceLabel returns the data source indicator text and color.
func (h *HUDApp) dataSourceLabel() (string, color.NRGBA) {
	if h.data.TokenExpired {
		return "● " + i18n.T.AuthExpired, h.colors.AccentRed
	}
	switch h.data.DataSource {
	case model.DataSourceAPI:
		return "● API", h.colors.AccentGreen
	case model.DataSourceCache:
		return "● " + i18n.T.Cache, h.colors.AccentOrange
	default:
		return "● " + i18n.T.Offline, h.colors.TextMuted
	}
}

// buildUsageCard creates a single usage window card with progress bar.
func (h *HUDApp) buildUsageCard(w model.RateLimitWindow) fyne.CanvasObject {
	label := canvas.NewText(w.Label, h.colors.TextSecondary)
	label.TextSize = 12

	pctText := fmt.Sprintf(i18n.T.UsagePct, w.UsagePct*100)
	pctLabel := canvas.NewText(pctText, h.colors.TextPrimary)
	pctLabel.TextSize = 12
	pctLabel.TextStyle = fyne.TextStyle{Bold: true}

	fillColor := h.colors.ProgressFill
	if w.UsagePct >= 0.95 {
		fillColor = h.colors.AccentRed
	} else if w.UsagePct >= 0.80 {
		fillColor = h.colors.AccentOrange
	}

	bar := newColoredProgressBar(h.colors.ProgressBg, fillColor)
	bar.SetValue(w.UsagePct)

	cardItems := []fyne.CanvasObject{
		container.NewHBox(label, layout.NewSpacer(), pctLabel),
		bar,
	}

	if w.TokensMax > 0 {
		tokenText := fmt.Sprintf("%s / %s tokens",
			model.FormatTokens(w.TokensUsed), model.FormatTokens(w.TokensMax))
		tokenLabel := canvas.NewText(tokenText, h.colors.TextMuted)
		tokenLabel.TextSize = 10
		cardItems = append(cardItems, tokenLabel)
	}

	resetText := fmt.Sprintf("%s %s", i18n.T.Reset, model.FormatTimeRemaining(w.ResetAt))
	resetLabel := canvas.NewText(resetText, h.colors.TextMuted)
	resetLabel.TextSize = 10
	cardItems = append(cardItems, resetLabel)

	return container.NewPadded(h.makeCard(container.NewVBox(cardItems...)))
}

// buildModelBreakdown shows per-model token usage.
func (h *HUDApp) buildModelBreakdown() fyne.CanvasObject {
	var parts []string
	for name, tokens := range h.data.Usage.ModelBreakdown {
		short := name
		if idx := strings.LastIndex(name, "-"); idx > 0 && idx < len(name)-1 {
			short = name[:idx]
		}
		if len(short) > 15 {
			short = short[:15]
		}
		parts = append(parts, fmt.Sprintf("%s=%s", short, model.FormatTokens(int(tokens))))
	}
	text := canvas.NewText(strings.Join(parts, "  "), h.colors.TextMuted)
	text.TextSize = 10
	return container.NewPadded(text)
}

// buildSessionsSection creates the sessions list with expandable cards.
func (h *HUDApp) buildSessionsSection() fyne.CanvasObject {
	divider := canvas.NewRectangle(h.colors.Divider)
	divider.SetMinSize(fyne.NewSize(0, 1))

	activeCount := 0
	for _, s := range h.data.Sessions {
		if s.Status == model.StatusRunning {
			activeCount++
		}
	}

	headerText := fmt.Sprintf("%s (%d %s)", i18n.T.Sessions, activeCount, i18n.T.Active)
	header := canvas.NewText(headerText, h.colors.TextPrimary)
	header.TextSize = 14
	header.TextStyle = fyne.TextStyle{Bold: true}

	items := []fyne.CanvasObject{divider, container.NewPadded(header)}

	for _, s := range h.data.Sessions {
		items = append(items, h.buildSessionCard(s))
	}
	for _, s := range h.data.ClosedSessions {
		items = append(items, h.buildClosedSessionCard(s))
	}

	if len(h.data.Sessions) == 0 && len(h.data.ClosedSessions) == 0 {
		noSessions := canvas.NewText("No active sessions", h.colors.TextMuted)
		noSessions.TextSize = 12
		items = append(items, container.NewPadded(noSessions))
	}

	return container.NewVBox(items...)
}

// buildSessionCard creates an expandable session card.
func (h *HUDApp) buildSessionCard(s model.Session) fyne.CanvasObject {
	statusColor := h.colors.AccentGreen
	if s.Status == model.StatusIdle {
		statusColor = h.colors.AccentOrange
	}
	dot := canvas.NewRectangle(statusColor)
	dot.CornerRadius = 4
	dot.SetMinSize(fyne.NewSize(8, 8))

	name := s.ProjectName
	if name == "" {
		name = fmt.Sprintf("Session %d", s.ID)
	}
	nameLabel := canvas.NewText(name, h.colors.TextPrimary)
	nameLabel.TextSize = 13
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}

	topRow := container.NewHBox(dot, nameLabel, layout.NewSpacer())
	if s.ActiveSkill != nil {
		topRow.Add(h.makeBadge(s.ActiveSkill.Name, h.colors.AccentOrange))
	}

	typeText := fmt.Sprintf("%s  %s  %d msgs", s.Type, model.FormatDuration(s.StartTime), s.Messages)
	typeLine := canvas.NewText(typeText, h.colors.TextSecondary)
	typeLine.TextSize = 11

	modelLabel := canvas.NewText(s.Model, h.colors.AccentCyan)
	modelLabel.TextSize = 10

	infoRow := container.NewHBox(typeLine, layout.NewSpacer(), modelLabel)
	cardItems := []fyne.CanvasObject{topRow, infoRow}

	if len(s.Agents) > 0 {
		isExpanded := h.expanded[s.ID]
		arrow := "▸"
		if isExpanded {
			arrow = "▾"
		}
		expandBtn := widget.NewButton(fmt.Sprintf("%s Agents (%d)", arrow, len(s.Agents)), nil)
		expandBtn.Importance = widget.LowImportance
		sid := s.ID
		expandBtn.OnTapped = func() {
			h.mu.Lock()
			h.expanded[sid] = !h.expanded[sid]
			h.mu.Unlock()
			h.rebuildUI()
		}
		cardItems = append(cardItems, expandBtn)
		if isExpanded {
			for _, agent := range s.Agents {
				cardItems = append(cardItems, h.buildAgentRow(agent))
			}
		}
	} else {
		noAgents := canvas.NewText(i18n.T.DirectWork, h.colors.TextMuted)
		noAgents.TextSize = 10
		cardItems = append(cardItems, noAgents)
	}

	return container.NewPadded(h.makeCard(container.NewVBox(cardItems...)))
}

// buildClosedSessionCard creates a dimmed card for a recently closed session.
func (h *HUDApp) buildClosedSessionCard(s model.Session) fyne.CanvasObject {
	dot := canvas.NewRectangle(h.colors.TextMuted)
	dot.CornerRadius = 4
	dot.SetMinSize(fyne.NewSize(8, 8))

	name := s.ProjectName
	if name == "" {
		name = fmt.Sprintf("Session %d", s.ID)
	}
	nameLabel := canvas.NewText(name+" (closed)", h.colors.TextMuted)
	nameLabel.TextSize = 12

	elapsed := canvas.NewText(model.FormatDuration(s.StartTime), h.colors.TextMuted)
	elapsed.TextSize = 10

	row := container.NewHBox(dot, nameLabel, layout.NewSpacer(), elapsed)
	return container.NewPadded(h.makeCard(row))
}

// buildAgentRow creates a row showing agent info with optional sub-agents.
func (h *HUDApp) buildAgentRow(agent model.Agent) fyne.CanvasObject {
	statusColor := h.colors.AccentGreen
	statusText := i18n.T.AgentRunning
	if agent.Status != model.StatusRunning {
		statusColor = h.colors.TextMuted
		statusText = i18n.T.AgentDone
	}

	dot := canvas.NewRectangle(statusColor)
	dot.CornerRadius = 3
	dot.SetMinSize(fyne.NewSize(6, 6))

	nameText := agent.Name
	if agent.Model != "" {
		nameText += " (" + agent.Model + ")"
	}
	nameLabel := canvas.NewText("  "+nameText, h.colors.TextSecondary)
	nameLabel.TextSize = 11

	statusLabel := canvas.NewText(statusText, statusColor)
	statusLabel.TextSize = 10

	row := container.NewHBox(dot, nameLabel, layout.NewSpacer(), statusLabel)
	items := []fyne.CanvasObject{row}

	for _, sub := range agent.SubAgents {
		subColor := h.colors.AccentGreen
		if sub.Status != model.StatusRunning {
			subColor = h.colors.TextMuted
		}
		subDot := canvas.NewRectangle(subColor)
		subDot.CornerRadius = 2
		subDot.SetMinSize(fyne.NewSize(4, 4))
		subName := canvas.NewText("    \u2514 "+sub.Name, h.colors.TextMuted)
		subName.TextSize = 10
		items = append(items, container.NewHBox(subDot, subName))
	}

	return container.NewVBox(items...)
}

// makeCard wraps content in a card-like container with background and border.
func (h *HUDApp) makeCard(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(h.colors.CardColor)
	bg.CornerRadius = 6
	bg.StrokeColor = h.colors.CardBorder
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.NewPadded(content))
}

// makeBadge creates a small colored badge with text.
func (h *HUDApp) makeBadge(text string, c color.NRGBA) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: c.R, G: c.G, B: c.B, A: 40})
	bg.CornerRadius = 4
	label := canvas.NewText(text, c)
	label.TextSize = 10
	label.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewStack(bg, container.NewPadded(label))
}

// refreshLoop periodically refreshes data.
func (h *HUDApp) refreshLoop() {
	ticker := time.NewTicker(time.Duration(h.cfg.RefreshMs) * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		h.triggerRefresh()
	}
}

// triggerRefresh fetches new data and rebuilds the UI.
func (h *HUDApp) triggerRefresh() {
	result := data.FetchRefreshData(h.data.Usage.Plan)

	h.mu.Lock()
	if result.Windows != nil {
		h.data.Usage.Windows = result.Windows
		data.UpdateTrend(&h.data.Usage)
	}
	h.data.Sessions = result.Sessions
	h.data.ClosedSessions = result.ClosedSessions
	h.data.DataSource = result.DataSource
	if result.TokenExpired {
		h.data.TokenExpired = true
		h.data.LastAuthError = result.LastAuthError
		h.data.APIFailCount++
	} else if result.APISuccess {
		h.data.LastAPISuccess = time.Now()
		h.data.APIFailCount = 0
		h.data.TokenExpired = false
		h.data.LastAuthError = ""
		if result.ModelBreakdown != nil {
			h.data.Usage.ModelBreakdown = result.ModelBreakdown
		}
	} else {
		h.data.APIFailCount++
	}
	h.mu.Unlock()

	h.checkNotifications()
	h.rebuildUI()
}

// checkNotifications checks usage thresholds and sends desktop notifications.
func (h *HUDApp) checkNotifications() {
	if !h.cfg.NotifyEnabled {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, w := range h.data.Usage.Windows {
		isFiveHour := strings.Contains(w.Label, i18n.T.FiveHour)
		remaining := model.FormatTimeRemaining(w.ResetAt)

		if isFiveHour {
			if !w.ResetAt.Equal(h.notify.lastFiveHourReset) {
				h.notify.fiveHourT1Fired = false
				h.notify.fiveHourT2Fired = false
				h.notify.lastFiveHourReset = w.ResetAt
			}
			if w.UsagePct >= h.cfg.NotifyThreshold1 && !h.notify.fiveHourT1Fired {
				h.notify.fiveHourT1Fired = true
				h.fyneApp.SendNotification(fyne.NewNotification(
					i18n.T.NotifyInfo,
					fmt.Sprintf(i18n.T.FiveHourInfoMsg, w.UsagePct*100, remaining)))
			}
			if w.UsagePct >= h.cfg.NotifyThreshold2 && !h.notify.fiveHourT2Fired {
				h.notify.fiveHourT2Fired = true
				h.fyneApp.SendNotification(fyne.NewNotification(
					i18n.T.NotifyWarning,
					fmt.Sprintf(i18n.T.FiveHourWarnMsg, w.UsagePct*100, remaining)))
			}
		} else {
			if !w.ResetAt.Equal(h.notify.lastWeeklyReset) {
				h.notify.weeklyT1Fired = false
				h.notify.weeklyT2Fired = false
				h.notify.lastWeeklyReset = w.ResetAt
			}
			if w.UsagePct >= h.cfg.NotifyThreshold1 && !h.notify.weeklyT1Fired {
				h.notify.weeklyT1Fired = true
				h.fyneApp.SendNotification(fyne.NewNotification(
					i18n.T.NotifyInfo,
					fmt.Sprintf(i18n.T.WeeklyInfoMsg, w.UsagePct*100, remaining)))
			}
			if w.UsagePct >= h.cfg.NotifyThreshold2 && !h.notify.weeklyT2Fired {
				h.notify.weeklyT2Fired = true
				h.fyneApp.SendNotification(fyne.NewNotification(
					i18n.T.NotifyWarning,
					fmt.Sprintf(i18n.T.WeeklyWarnMsg, w.UsagePct*100, remaining)))
			}
		}
	}
}

// coloredProgressBar is a custom widget with configurable fill colors.
type coloredProgressBar struct {
	widget.BaseWidget
	value     float64
	bgColor   color.NRGBA
	fillColor color.NRGBA
}

func newColoredProgressBar(bg, fill color.NRGBA) *coloredProgressBar {
	p := &coloredProgressBar{bgColor: bg, fillColor: fill}
	p.ExtendBaseWidget(p)
	return p
}

func (p *coloredProgressBar) SetValue(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.value = v
	p.Refresh()
}

func (p *coloredProgressBar) CreateRenderer() fyne.WidgetRenderer {
	bgRect := canvas.NewRectangle(p.bgColor)
	bgRect.CornerRadius = 4
	fillRect := canvas.NewRectangle(p.fillColor)
	fillRect.CornerRadius = 4
	return &progressBarRenderer{bar: p, bg: bgRect, fill: fillRect}
}

type progressBarRenderer struct {
	bar  *coloredProgressBar
	bg   *canvas.Rectangle
	fill *canvas.Rectangle
}

func (r *progressBarRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))
	fillW := float32(r.bar.value) * size.Width
	r.fill.Resize(fyne.NewSize(fillW, size.Height))
	r.fill.Move(fyne.NewPos(0, 0))
}

func (r *progressBarRenderer) MinSize() fyne.Size {
	return fyne.NewSize(50, 8)
}

func (r *progressBarRenderer) Refresh() {
	r.bg.FillColor = r.bar.bgColor
	r.fill.FillColor = r.bar.fillColor
	r.Layout(r.bg.Size())
}

func (r *progressBarRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.fill}
}

func (r *progressBarRenderer) Destroy() {}

// hudTheme implements fyne.Theme with HUD-specific colors.
type hudTheme struct {
	dark   bool
	colors FyneColors
}

func (t *hudTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return t.colors.BgColor
	case theme.ColorNameForeground:
		return t.colors.TextPrimary
	case theme.ColorNameButton:
		return t.colors.CardColor
	case theme.ColorNameDisabled:
		return t.colors.TextMuted
	case theme.ColorNameSeparator:
		return t.colors.Divider
	default:
		v := theme.VariantDark
		if !t.dark {
			v = theme.VariantLight
		}
		return theme.DefaultTheme().Color(name, v)
	}
}

func (t *hudTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *hudTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *hudTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
