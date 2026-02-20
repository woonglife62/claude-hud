package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// SessionType represents the type of Claude session
type SessionType string

const (
	SessionCowork  SessionType = "Cowork"
	SessionCode    SessionType = "Claude Code"
	SessionAPI     SessionType = "API"
	SessionDesktop SessionType = "Desktop"
	SessionWeb     SessionType = "Web"
)

// SessionStatus represents the current status
type SessionStatus string

const (
	StatusRunning SessionStatus = "Running"
	StatusIdle    SessionStatus = "Idle"
	StatusPaused  SessionStatus = "Paused"
)

// PlanType represents the Claude subscription plan
type PlanType string

const (
	PlanFree       PlanType = "Free"
	PlanPro        PlanType = "Pro"
	PlanTeam       PlanType = "Team"
	PlanEnterprise PlanType = "Enterprise"
	PlanMax5x      PlanType = "Max (5x)"
	PlanMax20x     PlanType = "Max (20x)"
)

// SubAgent represents a sub-agent within an agent
type SubAgent struct {
	Name   string
	Task   string
	Status SessionStatus
}

// SkillInfo represents the last activated OMC skill/strategy
type SkillInfo struct {
	Name string
	Args string
}

// Agent represents an agent within a session
type Agent struct {
	Name      string
	Model     string // model used by the agent (e.g. "sonnet", "haiku", "opus")
	Task      string
	Status    SessionStatus
	SubAgents []SubAgent
}

// Session represents a Claude session
type Session struct {
	ID          int
	DirKey      string // stable key from directory name (survives rescans)
	Type        SessionType
	Model       string
	Agents      []Agent
	ActiveSkill *SkillInfo // last activated OMC strategy (e.g. ralph, ultrawork)
	Status      SessionStatus
	StartTime   time.Time
	Messages    int
	ProjectName string    // short project name extracted from path
	ClosedAt    time.Time // non-zero when session recently closed
}

// RateLimitWindow represents a usage window with reset timer
type RateLimitWindow struct {
	Label        string  // "5시간" or "일간" or "주간"
	MessagesUsed int     // messages used in this window
	MessagesMax  int     // max allowed (0 = unlimited/unknown)
	TokensUsed   int     // tokens consumed in this window (derived from utilization * max)
	TokensMax    int     // max tokens allowed in this window (0 = unknown)
	UsagePct     float64 // 0.0 ~ 1.0 estimated usage percentage
	ResetAt      time.Time
}

// UsageData represents overall usage statistics
type UsageData struct {
	Plan           PlanType
	Model          string // current primary model
	Windows        []RateLimitWindow
	TotalMsgs      int              // total messages today
	TotalCost      float64          // API cost (if applicable)
	ModelBreakdown map[string]int64 // per-model token counts (model name -> tokens used)
	TrendHistory   []float64        // recent 5h usage percentages (oldest first)
	TrendArrow     string           // trend direction: ↑ ↗ → ↘ ↓
}

// DataSource indicates where usage data was obtained
type DataSource int

const (
	DataSourceNone  DataSource = iota // no data available
	DataSourceAPI                     // live from Anthropic API
	DataSourceCache                   // from OMC usage cache fallback
)

// HUDData holds all data displayed in the HUD
type HUDData struct {
	Account        string
	Usage          UsageData
	Sessions       []Session
	ClosedSessions []Session  // recently closed sessions (pruned by retention policy)
	DataSource     DataSource // where usage data came from
	LastAPISuccess time.Time  // last successful API fetch timestamp
	APIFailCount   int        // consecutive API failures
	TokenExpired   bool       // true when last API call returned 401/403
	LastAuthError  string     // last authentication error message
}

// RefreshResult holds results from a background data refresh.
type RefreshResult struct {
	Windows        []RateLimitWindow
	Sessions       []Session
	ClosedSessions []Session        // recently closed sessions
	DataSource     DataSource
	APISuccess     bool             // true if API call succeeded
	TokenExpired   bool             // true when API returned 401/403
	LastAuthError  string           // auth error message if TokenExpired
	ModelBreakdown map[string]int64 // per-model token counts
}

// FormatTokens formats token count as human-readable string
func FormatTokens(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// FormatDuration formats duration since start time
func FormatDuration(start time.Time) string {
	d := time.Since(start)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

// FormatTimeRemaining formats remaining time until reset
func FormatTimeRemaining(target time.Time) string {
	d := time.Until(target)
	if d <= 0 {
		return "리셋됨"
	}
	totalMinutes := int(d.Minutes())
	days := totalMinutes / (60 * 24)
	hours := (totalMinutes % (60 * 24)) / 60
	minutes := totalMinutes % 60
	if days > 0 {
		return fmt.Sprintf("%d일 %d시간 %d분", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d시간 %d분", hours, minutes)
	}
	return fmt.Sprintf("%d분", minutes)
}

// PlanTokenLimit5h returns the 5-hour token limit for the given plan.
// Values from Claude Code Usage Monitor (github.com/Maciek-roboblog/Claude-Code-Usage-Monitor).
func PlanTokenLimit5h(plan PlanType) int {
	switch plan {
	case PlanMax20x:
		return 220000
	case PlanMax5x:
		return 88000
	case PlanPro:
		return 19000
	case PlanTeam:
		return 44000
	case PlanEnterprise:
		return 44000
	default:
		return 19000 // Free/unknown default to Pro limits
	}
}

// PlanTokenLimitWeekly returns 0 because authoritative 7-day token caps
// are not published. The API utilization percentage is displayed directly;
// absolute token counts are only shown for the 5-hour window where
// the per-plan limits are well-documented.
func PlanTokenLimitWeekly(_ PlanType) int {
	return 0
}

// ScanResult holds all extracted data from a session JSONL scan.
type ScanResult struct {
	Agents       []Agent
	ActiveSkill  *SkillInfo
	SessionModel string
	MessageCount int
}

// SessionScanResult holds the cached result of scanning a JSONL file.
type SessionScanResult struct {
	Agents   []Agent
	Skill    *SkillInfo
	Messages int
	Model    string
}

// FileCache holds the cached scan result for a JSONL file keyed by path.
type FileCache struct {
	ModTime  time.Time
	FileSize int64
	Result   *SessionScanResult
}

// UsageCacheData mirrors the JSON structure of .usage-cache.json
type UsageCacheData struct {
	Timestamp int64 `json:"timestamp"`
	Data      struct {
		FiveHourPercent      int    `json:"fiveHourPercent"`
		WeeklyPercent        int    `json:"weeklyPercent"`
		FiveHourResetsAt     string `json:"fiveHourResetsAt"`
		WeeklyResetsAt       string `json:"weeklyResetsAt"`
		SonnetWeeklyPercent  int    `json:"sonnetWeeklyPercent"`
		SonnetWeeklyResetsAt string `json:"sonnetWeeklyResetsAt"`
	} `json:"data"`
	Error  bool   `json:"error"`
	Source string `json:"source"`
}

// CredentialsData mirrors ~/.claude/.credentials.json
type CredentialsData struct {
	ClaudeAiOauth struct {
		AccessToken      string   `json:"accessToken"`
		RefreshToken     string   `json:"refreshToken"`
		ExpiresAt        int64    `json:"expiresAt"`
		Scopes           []string `json:"scopes"`
		SubscriptionType string   `json:"subscriptionType"`
		RateLimitTier    string   `json:"rateLimitTier"`
	} `json:"claudeAiOauth"`
}

// APIModelUsage holds per-model token breakdown within a usage window.
type APIModelUsage struct {
	ModelID    string `json:"model_id"`
	TokensUsed int64  `json:"tokens_used"`
}

// APIUsageResponse mirrors the JSON response from GET /api/oauth/usage
type APIUsageResponse struct {
	FiveHour struct {
		Utilization float64         `json:"utilization"`
		ResetsAt    string          `json:"resets_at"`
		Models      []APIModelUsage `json:"models"`
	} `json:"five_hour"`
	SevenDay struct {
		Utilization float64         `json:"utilization"`
		ResetsAt    string          `json:"resets_at"`
		Models      []APIModelUsage `json:"models"`
	} `json:"seven_day"`
}

// JsonlLine represents a single line from the JSONL transcript.
type JsonlLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		Model   string            `json:"model"`
		Role    string            `json:"role"`
		Content []json.RawMessage `json:"content"`
	} `json:"message"`
}

// ContentBlock represents a content block within a JSONL message.
type ContentBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	ToolUseID string          `json:"tool_use_id"`
	Input     json.RawMessage `json:"input"`
	Content   json.RawMessage `json:"content"`
	Text      string          `json:"text"`
}

// TaskToolInput represents the input for a Task tool_use.
type TaskToolInput struct {
	SubagentType string `json:"subagent_type"`
	Model        string `json:"model"`
	Description  string `json:"description"`
}

// SkillToolInput represents the input for a Skill tool_use.
type SkillToolInput struct {
	Skill string `json:"skill"`
	Args  string `json:"args"`
}

// ParseISO parses an ISO 8601 timestamp string, returning zero time on error.
// Handles both "2006-01-02T15:04:05Z" and "2006-01-02T15:04:05.000Z" formats.
func ParseISO(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// RFC3339Nano handles both with and without fractional seconds
	t, err := time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return t
	}
	// Fallback to standard RFC3339
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		return t
	}
	return time.Time{}
}

// RateLimitTierToPlan maps the rateLimitTier string to a PlanType.
func RateLimitTierToPlan(tier string) PlanType {
	switch {
	case contains(tier, "max_20x") || contains(tier, "20x"):
		return PlanMax20x
	case contains(tier, "max_5x") || contains(tier, "5x"):
		return PlanMax5x
	case contains(tier, "enterprise"):
		return PlanEnterprise
	case contains(tier, "team"):
		return PlanTeam
	case contains(tier, "pro"):
		return PlanPro
	default:
		return PlanFree
	}
}

// contains checks if s contains substr (avoids importing strings in model package)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
