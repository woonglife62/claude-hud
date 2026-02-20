package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	ProjectName string // short project name extracted from path
}

// RateLimitWindow represents a usage window with reset timer
type RateLimitWindow struct {
	Label        string  // "5시간" or "일간" or "주간"
	MessagesUsed int     // messages used in this window
	MessagesMax  int     // max allowed (0 = unlimited/unknown)
	TokensUsed   int     // tokens consumed in this window (derived from utilization × max)
	TokensMax    int     // max tokens allowed in this window (0 = unknown)
	UsagePct     float64 // 0.0 ~ 1.0 estimated usage percentage
	ResetAt      time.Time
}

// planTokenLimit5h returns the 5-hour token limit for the given plan.
// Values from Claude Code Usage Monitor (github.com/Maciek-roboblog/Claude-Code-Usage-Monitor).
func planTokenLimit5h(plan PlanType) int {
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

// planTokenLimitWeekly returns 0 because authoritative 7-day token caps
// are not published. The API utilization percentage is displayed directly;
// absolute token counts are only shown for the 5-hour window where
// the per-plan limits are well-documented.
func planTokenLimitWeekly(_ PlanType) int {
	return 0
}

// UsageData represents overall usage statistics
type UsageData struct {
	Plan      PlanType
	Model     string // current primary model
	Windows   []RateLimitWindow
	TotalMsgs int     // total messages today
	TotalCost float64 // API cost (if applicable)
}

// DataSource indicates where usage data was obtained
type DataSource int

const (
	DataSourceNone  DataSource = iota // no data available
	DataSourceAPI                      // live from Anthropic API
	DataSourceCache                    // from OMC usage cache fallback
)

// HUDData holds all data displayed in the HUD
type HUDData struct {
	Account        string
	Usage          UsageData
	Sessions       []Session
	DataSource     DataSource // where usage data came from
	LastAPISuccess time.Time  // last successful API fetch timestamp
	APIFailCount   int        // consecutive API failures
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

// usageCacheData mirrors the JSON structure of .usage-cache.json
type usageCacheData struct {
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

// credentialsData mirrors ~/.claude/.credentials.json
type credentialsData struct {
	ClaudeAiOauth struct {
		AccessToken      string   `json:"accessToken"`
		RefreshToken     string   `json:"refreshToken"`
		ExpiresAt        int64    `json:"expiresAt"`
		Scopes           []string `json:"scopes"`
		SubscriptionType string   `json:"subscriptionType"`
		RateLimitTier    string   `json:"rateLimitTier"`
	} `json:"claudeAiOauth"`
}

// apiUsageResponse mirrors the JSON response from GET /api/oauth/usage
type apiUsageResponse struct {
	FiveHour struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"five_hour"`
	SevenDay struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"seven_day"`
}

// httpClient is a package-level client with a short timeout to avoid blocking the UI.
var httpClient = &http.Client{Timeout: 3 * time.Second}

// sessionScanResult holds the cached result of scanning a JSONL file.
type sessionScanResult struct {
	agents   []Agent
	skill    *SkillInfo
	messages int
	model    string
}

// fileCache holds the cached scan result for a JSONL file keyed by path.
type fileCache struct {
	modTime  time.Time
	fileSize int64
	result   *sessionScanResult
}

var scanCacheMu sync.Mutex
var scanCache = make(map[string]*fileCache)

// fetchUsageFromAPI calls the Anthropic OAuth usage endpoint and returns the parsed
// response, or nil on any error (network, auth, parse, missing token).
func fetchUsageFromAPI(homeDir string) *apiUsageResponse {
	creds := readCredentials(homeDir)
	if creds == nil || creds.ClaudeAiOauth.AccessToken == "" {
		Log("  [usage] No credentials or access token")
		return nil
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		Log("  [usage] Failed to create request: %v", err)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+creds.ClaudeAiOauth.AccessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	resp, err := httpClient.Do(req)
	if err != nil {
		Log("  [usage] API request failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		Log("  [usage] API returned status %d", resp.StatusCode)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		Log("  [usage] Failed to read response body: %v", err)
		return nil
	}

	var usage apiUsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		Log("  [usage] Failed to parse JSON: %v", err)
		return nil
	}
	Log("  [usage] API OK: 5h=%.1f%% reset=%s, 7d=%.1f%% reset=%s",
		usage.FiveHour.Utilization, usage.FiveHour.ResetsAt,
		usage.SevenDay.Utilization, usage.SevenDay.ResetsAt)
	return &usage
}

// buildWindowsFromAPI converts an apiUsageResponse to RateLimitWindow entries.
// plan is used to derive absolute token counts from the utilization percentage.
// NOTE: The API returns utilization as a percentage (0-100), not a decimal (0-1).
func buildWindowsFromAPI(resp *apiUsageResponse, plan PlanType) []RateLimitWindow {
	max5h := planTokenLimit5h(plan)
	maxWeekly := planTokenLimitWeekly(plan)

	// Convert from 0-100 percentage to 0-1 decimal
	pct5h := resp.FiveHour.Utilization / 100.0
	pctWeekly := resp.SevenDay.Utilization / 100.0

	// Clamp to [0, 1] range (API may return > 100 if over-limit)
	if pct5h > 1.0 {
		pct5h = 1.0
	}
	if pctWeekly > 1.0 {
		pctWeekly = 1.0
	}

	return []RateLimitWindow{
		{
			Label:      "5시간 사용량",
			UsagePct:   pct5h,
			TokensUsed: int(pct5h * float64(max5h)),
			TokensMax:  max5h,
			ResetAt:    parseISO(resp.FiveHour.ResetsAt),
		},
		{
			Label:      "주간 사용량",
			UsagePct:   pctWeekly,
			TokensUsed: int(pctWeekly * float64(maxWeekly)),
			TokensMax:  maxWeekly,
			ResetAt:    parseISO(resp.SevenDay.ResetsAt),
		},
	}
}

// parseISO parses an ISO 8601 timestamp string, returning zero time on error.
// Handles both "2006-01-02T15:04:05Z" and "2006-01-02T15:04:05.000Z" formats.
func parseISO(s string) time.Time {
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

// rateLimitTierToPlan maps the rateLimitTier string to a PlanType.
func rateLimitTierToPlan(tier string) PlanType {
	switch {
	case strings.Contains(tier, "max_20x") || strings.Contains(tier, "20x"):
		return PlanMax20x
	case strings.Contains(tier, "max_5x") || strings.Contains(tier, "5x"):
		return PlanMax5x
	case strings.Contains(tier, "enterprise"):
		return PlanEnterprise
	case strings.Contains(tier, "team"):
		return PlanTeam
	case strings.Contains(tier, "pro"):
		return PlanPro
	default:
		return PlanFree
	}
}

// readUsageCache reads and parses the OMC usage cache file.
// Returns nil if the file is missing, malformed, or stale (reset times in the past).
func readUsageCache(homeDir string) *usageCacheData {
	path := filepath.Join(homeDir, ".claude", "plugins", "oh-my-claudecode", ".usage-cache.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cache usageCacheData
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil
	}
	if cache.Error {
		return nil
	}
	// Check staleness: if either reset time is in the past, the cache is outdated
	now := time.Now()
	fiveHourReset := parseISO(cache.Data.FiveHourResetsAt)
	weeklyReset := parseISO(cache.Data.WeeklyResetsAt)
	if (!fiveHourReset.IsZero() && fiveHourReset.Before(now)) ||
		(!weeklyReset.IsZero() && weeklyReset.Before(now)) {
		Log("  [usage] Cache stale: 5h reset=%s, weekly reset=%s (both in past)",
			cache.Data.FiveHourResetsAt, cache.Data.WeeklyResetsAt)
		return nil
	}
	Log("  [usage] Cache OK: 5h=%d%%, weekly=%d%%", cache.Data.FiveHourPercent, cache.Data.WeeklyPercent)
	return &cache
}

// readCredentials reads and parses ~/.claude/.credentials.json.
// Returns nil if missing or malformed.
func readCredentials(homeDir string) *credentialsData {
	path := filepath.Join(homeDir, ".claude", ".credentials.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var creds credentialsData
	if err := json.Unmarshal(raw, &creds); err != nil {
		return nil
	}
	return &creds
}

// jsonlLine represents a single line from the JSONL transcript.
type jsonlLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		Model   string            `json:"model"`
		Role    string            `json:"role"`
		Content []json.RawMessage `json:"content"`
	} `json:"message"`
}

// contentBlock represents a content block within a JSONL message.
type contentBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	ToolUseID string          `json:"tool_use_id"`
	Input     json.RawMessage `json:"input"`
	Content   json.RawMessage `json:"content"`
	Text      string          `json:"text"`
}

// taskToolInput represents the input for a Task tool_use.
type taskToolInput struct {
	SubagentType string `json:"subagent_type"`
	Model        string `json:"model"`
	Description  string `json:"description"`
}

// skillToolInput represents the input for a Skill tool_use.
type skillToolInput struct {
	Skill string `json:"skill"`
	Args  string `json:"args"`
}

// scanResult holds all extracted data from a session JSONL scan.
type scanResult struct {
	Agents       []Agent
	ActiveSkill  *SkillInfo
	SessionModel string
	MessageCount int
}

// scanSessionAgents reads the tail of a JSONL file to extract agent lifecycle,
// active skills/strategies, and session metadata. It tracks both tool_use (agent start)
// and tool_result (agent completion) blocks to determine running vs completed status.
func scanSessionAgents(jsonlPath string) scanResult {
	const tailSize = 256 * 1024 // 256KB for richer history

	f, err := os.Open(jsonlPath)
	if err != nil {
		return scanResult{}
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return scanResult{}
	}
	offset := int64(0)
	if info.Size() > tailSize {
		offset = info.Size() - tailSize
	}
	f.Seek(offset, 0)

	// Skip partial first line at seek boundary
	if offset > 0 {
		buf := make([]byte, 1)
		for {
			n, readErr := f.Read(buf)
			if readErr != nil || (n == 1 && buf[0] == '\n') {
				break
			}
		}
	}

	raw, err := io.ReadAll(f)
	if err != nil {
		return scanResult{}
	}

	// Agent lifecycle tracking
	type agentEntry struct {
		agentType   string
		model       string
		description string
		status      string // "running" or "completed"
		startTime   time.Time
	}

	agentMap := make(map[string]*agentEntry) // key: tool_use block id
	bgAgentMap := make(map[string]string)    // key: background agent id -> tool_use block id
	var lastSkill *SkillInfo
	sessionModel := ""
	messageCount := 0

	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry jsonlLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Skip non-message entries (progress, hooks, etc.)
		if entry.Type != "assistant" && entry.Type != "user" {
			continue
		}

		if entry.Type == "assistant" {
			messageCount++
			if entry.Message.Model != "" {
				sessionModel = entry.Message.Model
			}
		}

		ts := parseISO(entry.Timestamp)
		if ts.IsZero() {
			ts = time.Now()
		}

		// Process each content block
		for _, rawBlock := range entry.Message.Content {
			var block contentBlock
			if json.Unmarshal(rawBlock, &block) != nil {
				continue
			}

			switch block.Type {
			case "tool_use":
				if block.Name == "Task" || block.Name == "proxy_Task" {
					var input taskToolInput
					if json.Unmarshal(block.Input, &input) == nil && input.SubagentType != "" {
						agentMap[block.ID] = &agentEntry{
							agentType:   input.SubagentType,
							model:       input.Model,
							description: input.Description,
							status:      "running",
							startTime:   ts,
						}
						// Cap at 100 entries, evict oldest completed
						if len(agentMap) > 100 {
							var oldestKey string
							var oldestTime time.Time
							for k, v := range agentMap {
								if v.status == "completed" {
									if oldestKey == "" || v.startTime.Before(oldestTime) {
										oldestKey = k
										oldestTime = v.startTime
									}
								}
							}
							if oldestKey != "" {
								delete(agentMap, oldestKey)
							}
						}
					}
				} else if block.Name == "Skill" || block.Name == "proxy_Skill" {
					var input skillToolInput
					if json.Unmarshal(block.Input, &input) == nil && input.Skill != "" {
						skillName := strings.TrimPrefix(input.Skill, "oh-my-claudecode:")
						lastSkill = &SkillInfo{
							Name: skillName,
							Args: input.Args,
						}
					}
				}

			case "tool_result":
				if block.ToolUseID == "" {
					continue
				}
				contentStr := extractToolResultText(block.Content, block.Text)

				// Check if this completes an agent
				if agent, ok := agentMap[block.ToolUseID]; ok {
					if strings.Contains(contentStr, "Async agent launched") {
						// Background agent: keep as running, extract bgAgentId
						if idx := strings.Index(contentStr, "agentId:"); idx >= 0 {
							rest := strings.TrimSpace(contentStr[idx+8:])
							var bgID string
							for _, ch := range rest {
								if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
									bgID += string(ch)
								} else {
									break
								}
							}
							if bgID != "" {
								bgAgentMap[bgID] = block.ToolUseID
							}
						}
					} else {
						agent.status = "completed"
					}
				}

				// Check for TaskOutput completion (background agents)
				if strings.Contains(contentStr, "<task_id>") && strings.Contains(contentStr, "<status>completed</status>") {
					if start := strings.Index(contentStr, "<task_id>"); start >= 0 {
						rest := contentStr[start+9:]
						if end := strings.Index(rest, "</task_id>"); end >= 0 {
							taskID := rest[:end]
							if toolUseID, ok := bgAgentMap[taskID]; ok {
								if agent, ok := agentMap[toolUseID]; ok {
									agent.status = "completed"
								}
							}
						}
					}
				}
			}
		}
	}

	// Stale agent detection: running > 30 min → mark completed
	now := time.Now()
	staleThreshold := 30 * time.Minute
	for _, agent := range agentMap {
		if agent.status == "running" && now.Sub(agent.startTime) > staleThreshold {
			agent.status = "completed"
		}
	}

	// Build result: running first, then most recent completed, cap at 10
	var running, completed []*agentEntry
	for _, agent := range agentMap {
		if agent.status == "running" {
			running = append(running, agent)
		} else {
			completed = append(completed, agent)
		}
	}

	sort.Slice(running, func(i, j int) bool {
		return running[i].startTime.After(running[j].startTime)
	})
	sort.Slice(completed, func(i, j int) bool {
		return completed[i].startTime.After(completed[j].startTime)
	})

	maxAgents := 10
	var resultAgents []Agent

	for _, a := range running {
		if len(resultAgents) >= maxAgents {
			break
		}
		displayName := strings.TrimPrefix(a.agentType, "oh-my-claudecode:")
		resultAgents = append(resultAgents, Agent{
			Name:   displayName,
			Model:  a.model,
			Task:   a.description,
			Status: StatusRunning,
		})
	}

	remaining := maxAgents - len(resultAgents)
	for i := 0; i < remaining && i < len(completed); i++ {
		displayName := strings.TrimPrefix(completed[i].agentType, "oh-my-claudecode:")
		resultAgents = append(resultAgents, Agent{
			Name:   displayName,
			Model:  completed[i].model,
			Task:   completed[i].description,
			Status: StatusIdle,
		})
	}

	return scanResult{
		Agents:       resultAgents,
		ActiveSkill:  lastSkill,
		SessionModel: sessionModel,
		MessageCount: messageCount,
	}
}

// extractToolResultText extracts text content from a tool_result content field.
// Content can be a string, or an array of text blocks.
func extractToolResultText(content json.RawMessage, text string) string {
	if text != "" {
		return text
	}
	if len(content) == 0 {
		return ""
	}
	// Try as string
	var s string
	if json.Unmarshal(content, &s) == nil {
		return s
	}
	// Try as array of text blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) == nil {
		var sb strings.Builder
		for _, b := range blocks {
			if b.Type == "text" {
				sb.WriteString(b.Text)
			}
		}
		return sb.String()
	}
	return ""
}

// scanSessionAgentsWithCache performs an incremental scan when possible.
// If the file grew since last scan (cached != nil, newSize > cached.fileSize),
// only the new bytes are read and merged with cached agents.
// Otherwise falls back to a full scanSessionAgents call.
func scanSessionAgentsWithCache(jsonlPath string, newSize int64, cached *fileCache) scanResult {
	if cached == nil || newSize <= cached.fileSize {
		return scanSessionAgents(jsonlPath)
	}

	f, err := os.Open(jsonlPath)
	if err != nil {
		return scanSessionAgents(jsonlPath)
	}
	defer f.Close()

	if _, err := f.Seek(cached.fileSize, 0); err != nil {
		return scanSessionAgents(jsonlPath)
	}

	// Skip partial first line at seek boundary
	buf := make([]byte, 1)
	for {
		n, readErr := f.Read(buf)
		if readErr != nil || (n == 1 && buf[0] == '\n') {
			break
		}
	}

	raw, err := io.ReadAll(f)
	if err != nil || len(raw) == 0 {
		return scanResult{
			Agents:       cached.result.agents,
			ActiveSkill:  cached.result.skill,
			SessionModel: cached.result.model,
			MessageCount: cached.result.messages,
		}
	}

	type agentEntry struct {
		agentType   string
		model       string
		description string
		status      string
		startTime   time.Time
	}

	agentMap := make(map[string]*agentEntry)
	for _, a := range cached.result.agents {
		status := "completed"
		if a.Status == StatusRunning {
			status = "running"
		}
		agentMap["cached-"+a.Name] = &agentEntry{
			agentType:   a.Name,
			model:       a.Model,
			description: a.Task,
			status:      status,
		}
	}

	bgAgentMap := make(map[string]string)
	lastSkill := cached.result.skill
	sessionModel := cached.result.model
	messageCount := cached.result.messages

	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry jsonlLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Type != "assistant" && entry.Type != "user" {
			continue
		}
		if entry.Type == "assistant" {
			messageCount++
			if entry.Message.Model != "" {
				sessionModel = entry.Message.Model
			}
		}
		ts := parseISO(entry.Timestamp)
		if ts.IsZero() {
			ts = time.Now()
		}
		for _, rawBlock := range entry.Message.Content {
			var block contentBlock
			if json.Unmarshal(rawBlock, &block) != nil {
				continue
			}
			switch block.Type {
			case "tool_use":
				if block.Name == "Task" || block.Name == "proxy_Task" {
					var input taskToolInput
					if json.Unmarshal(block.Input, &input) == nil && input.SubagentType != "" {
						agentMap[block.ID] = &agentEntry{
							agentType:   input.SubagentType,
							model:       input.Model,
							description: input.Description,
							status:      "running",
							startTime:   ts,
						}
					}
				} else if block.Name == "Skill" || block.Name == "proxy_Skill" {
					var input skillToolInput
					if json.Unmarshal(block.Input, &input) == nil && input.Skill != "" {
						skillName := strings.TrimPrefix(input.Skill, "oh-my-claudecode:")
						lastSkill = &SkillInfo{Name: skillName, Args: input.Args}
					}
				}
			case "tool_result":
				if block.ToolUseID == "" {
					continue
				}
				contentStr := extractToolResultText(block.Content, block.Text)
				if agent, ok := agentMap[block.ToolUseID]; ok {
					if strings.Contains(contentStr, "Async agent launched") {
						if idx := strings.Index(contentStr, "agentId:"); idx >= 0 {
							rest := strings.TrimSpace(contentStr[idx+8:])
							var bgID string
							for _, ch := range rest {
								if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
									bgID += string(ch)
								} else {
									break
								}
							}
							if bgID != "" {
								bgAgentMap[bgID] = block.ToolUseID
							}
						}
					} else {
						agent.status = "completed"
					}
				}
				if strings.Contains(contentStr, "<task_id>") && strings.Contains(contentStr, "<status>completed</status>") {
					if start := strings.Index(contentStr, "<task_id>"); start >= 0 {
						rest := contentStr[start+9:]
						if end := strings.Index(rest, "</task_id>"); end >= 0 {
							taskID := rest[:end]
							if toolUseID, ok := bgAgentMap[taskID]; ok {
								if agent, ok := agentMap[toolUseID]; ok {
									agent.status = "completed"
								}
							}
						}
					}
				}
			}
		}
	}

	now := time.Now()
	staleThreshold := 30 * time.Minute
	for _, agent := range agentMap {
		if agent.status == "running" && !agent.startTime.IsZero() && now.Sub(agent.startTime) > staleThreshold {
			agent.status = "completed"
		}
	}

	var running, completed []*agentEntry
	for _, agent := range agentMap {
		if agent.status == "running" {
			running = append(running, agent)
		} else {
			completed = append(completed, agent)
		}
	}
	sort.Slice(running, func(i, j int) bool {
		return running[i].startTime.After(running[j].startTime)
	})
	sort.Slice(completed, func(i, j int) bool {
		return completed[i].startTime.After(completed[j].startTime)
	})

	maxAgents := 10
	var resultAgents []Agent
	for _, a := range running {
		if len(resultAgents) >= maxAgents {
			break
		}
		displayName := strings.TrimPrefix(a.agentType, "oh-my-claudecode:")
		resultAgents = append(resultAgents, Agent{
			Name:   displayName,
			Model:  a.model,
			Task:   a.description,
			Status: StatusRunning,
		})
	}
	remaining := maxAgents - len(resultAgents)
	for i := 0; i < remaining && i < len(completed); i++ {
		displayName := strings.TrimPrefix(completed[i].agentType, "oh-my-claudecode:")
		resultAgents = append(resultAgents, Agent{
			Name:   displayName,
			Model:  completed[i].model,
			Task:   completed[i].description,
			Status: StatusIdle,
		})
	}

	return scanResult{
		Agents:       resultAgents,
		ActiveSkill:  lastSkill,
		SessionModel: sessionModel,
		MessageCount: messageCount,
	}
}

// scanSessions scans ~/.claude/projects/*/ for active JSONL transcript files.
// Running = modified within last 5 min. Idle = modified within last 30 min.
// Uses mtime caching to skip re-scanning unchanged files.
func scanSessions(homeDir string) []Session {
	scanStart := time.Now()

	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil
	}

	now := time.Now()
	runningThreshold := 5 * time.Minute
	idleThreshold := 30 * time.Minute

	var sessions []Session
	sessionID := 1

	// Track which paths are still active so we can evict stale cache entries.
	seenPaths := make(map[string]bool)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		projectDir := filepath.Join(projectsDir, dirName)

		// Find .jsonl files in this project directory
		jsonlFiles, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		if err != nil || len(jsonlFiles) == 0 {
			continue
		}

		// Find the most recently modified .jsonl file
		var newestTime time.Time
		for _, f := range jsonlFiles {
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			if info.ModTime().After(newestTime) {
				newestTime = info.ModTime()
			}
		}

		age := now.Sub(newestTime)
		if age > idleThreshold {
			// Too old, skip
			continue
		}

		var status SessionStatus
		if age <= runningThreshold {
			status = StatusRunning
		} else {
			status = StatusIdle
		}

		// Extract a short project name from the directory name.
		// Directory format: C--Users-openerd-go-src-project-name
		// Use the last non-empty segment split by '-'.
		projectName := extractProjectName(dirName)

		// Extract agents and model from the newest JSONL file
		var newestFile string
		var newestFileMod time.Time
		var newestFileSize int64
		for _, f := range jsonlFiles {
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			if info.ModTime().After(newestFileMod) {
				newestFileMod = info.ModTime()
				newestFileSize = info.Size()
				newestFile = f
			}
		}

		seenPaths[newestFile] = true

		// Check mtime cache before scanning
		scanCacheMu.Lock()
		cached, hasCached := scanCache[newestFile]
		scanCacheMu.Unlock()

		var sr scanResult
		if hasCached && cached.modTime.Equal(newestFileMod) {
			// Cache hit: file unchanged, reuse result
			sr = scanResult{
				Agents:       cached.result.agents,
				ActiveSkill:  cached.result.skill,
				SessionModel: cached.result.model,
				MessageCount: cached.result.messages,
			}
		} else {
			// Cache miss or file changed: scan the file
			sr = scanSessionAgentsWithCache(newestFile, newestFileSize, cached)

			// Store result in cache
			scanCacheMu.Lock()
			scanCache[newestFile] = &fileCache{
				modTime:  newestFileMod,
				fileSize: newestFileSize,
				result: &sessionScanResult{
					agents:   sr.Agents,
					skill:    sr.ActiveSkill,
					model:    sr.SessionModel,
					messages: sr.MessageCount,
				},
			}
			scanCacheMu.Unlock()
		}

		model := sr.SessionModel
		if model == "" {
			model = "Claude"
		}

		sessions = append(sessions, Session{
			ID:          sessionID,
			DirKey:      dirName,
			Type:        SessionCode,
			Model:       model,
			Agents:      sr.Agents,
			ActiveSkill: sr.ActiveSkill,
			Status:      status,
			StartTime:   newestTime,
			Messages:    sr.MessageCount,
			ProjectName: projectName,
		})
		sessionID++
	}

	// Evict cache entries for paths no longer active
	scanCacheMu.Lock()
	for path := range scanCache {
		if !seenPaths[path] {
			delete(scanCache, path)
		}
	}
	scanCacheMu.Unlock()

	Log("scanSessions: %d sessions scanned in %v", len(sessions), time.Since(scanStart))
	return sessions
}

// extractProjectName derives a short human-readable project name from a
// ~/.claude/projects directory name such as "C--Users-openerd-go-src-foo-bar".
// The directory name encodes the original file path with "-" as the separator.
// We reconstruct the original path and take the last 1-2 meaningful segments.
func extractProjectName(dirName string) string {
	// Split on "--" to remove drive prefix like "C--"
	parts := strings.SplitN(dirName, "--", 2)
	remainder := dirName
	if len(parts) == 2 {
		remainder = parts[1]
	}

	// The remainder encodes path segments with "-".
	// e.g. "Users-openerd-go-src-claude-hud-claude-hud"
	// We need to reconstruct: try to match against actual directory structure.
	// Heuristic: skip known prefix segments (Users, username, common dirs)
	segments := strings.Split(remainder, "-")

	// Skip common path prefixes
	skipWords := map[string]bool{
		"Users": true, "users": true, "home": true,
		"src": true, "go": true, "projects": true,
		"Documents": true, "Desktop": true, "workspace": true,
		"Workspace": true, "repos": true, "code": true,
	}

	// Find the first meaningful segment (skip username too - typically 2nd segment)
	startIdx := 0
	skippedUser := false
	for i, seg := range segments {
		if skipWords[seg] {
			startIdx = i + 1
			continue
		}
		// Skip what looks like a username (first non-skip segment after Users)
		if !skippedUser && i > 0 {
			skippedUser = true
			startIdx = i + 1
			continue
		}
		break
	}

	if startIdx >= len(segments) {
		startIdx = len(segments) - 1
	}

	// Take remaining segments as the project name, joined with "-"
	projectParts := segments[startIdx:]
	if len(projectParts) == 0 {
		return dirName
	}
	result := strings.Join(projectParts, "-")
	if result == "" {
		return dirName
	}
	return result
}

// buildWindows constructs RateLimitWindow entries from cache data.
// plan is used to derive absolute token counts from the percentage.
func buildWindows(cache *usageCacheData, plan PlanType) []RateLimitWindow {
	var windows []RateLimitWindow

	fiveHourReset := parseISO(cache.Data.FiveHourResetsAt)
	weeklyReset := parseISO(cache.Data.WeeklyResetsAt)

	max5h := planTokenLimit5h(plan)
	maxWeekly := planTokenLimitWeekly(plan)

	pct5h := float64(cache.Data.FiveHourPercent) / 100.0
	pctWeekly := float64(cache.Data.WeeklyPercent) / 100.0

	windows = append(windows, RateLimitWindow{
		Label:      "5시간 사용량",
		UsagePct:   pct5h,
		TokensUsed: int(pct5h * float64(max5h)),
		TokensMax:  max5h,
		ResetAt:    fiveHourReset,
	})

	windows = append(windows, RateLimitWindow{
		Label:      "주간 사용량",
		UsagePct:   pctWeekly,
		TokensUsed: int(pctWeekly * float64(maxWeekly)),
		TokensMax:  maxWeekly,
		ResetAt:    weeklyReset,
	})

	return windows
}

// LoadRealData reads live data from Claude config files and returns a populated HUDData.
func LoadRealData() *HUDData {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	hud := &HUDData{
		Account: "Claude User",
		Usage: UsageData{
			Plan:  PlanFree,
			Model: "Opus 4",
		},
	}

	// Read credentials for plan type
	if homeDir != "" {
		if creds := readCredentials(homeDir); creds != nil {
			tier := creds.ClaudeAiOauth.RateLimitTier
			hud.Usage.Plan = rateLimitTierToPlan(tier)
		}
	}

	// Try direct API first for near-real-time accuracy, fall back to cache
	if homeDir != "" {
		if apiResp := fetchUsageFromAPI(homeDir); apiResp != nil {
			hud.Usage.Windows = buildWindowsFromAPI(apiResp, hud.Usage.Plan)
			hud.DataSource = DataSourceAPI
			hud.LastAPISuccess = time.Now()
			hud.APIFailCount = 0
			Log("  [usage] Source: API direct")
		} else if cache := readUsageCache(homeDir); cache != nil {
			hud.Usage.Windows = buildWindows(cache, hud.Usage.Plan)
			hud.DataSource = DataSourceCache
			hud.APIFailCount++
			Log("  [usage] Source: OMC cache")
		} else {
			hud.DataSource = DataSourceNone
			hud.APIFailCount++
			Log("  [usage] Source: none (using defaults)")
		}
	}

	// Default windows if cache unavailable
	if len(hud.Usage.Windows) == 0 {
		now := time.Now()
		max5h := planTokenLimit5h(hud.Usage.Plan)
		maxWeekly := planTokenLimitWeekly(hud.Usage.Plan)
		hud.Usage.Windows = []RateLimitWindow{
			{
				Label:     "5시간 사용량",
				UsagePct:  0,
				TokensMax: max5h,
				ResetAt:   now.Add(5 * time.Hour),
			},
			{
				Label:     "주간 사용량",
				UsagePct:  0,
				TokensMax: maxWeekly,
				ResetAt:   now.Add(7 * 24 * time.Hour),
			},
		}
	}

	// Scan active sessions
	if homeDir != "" {
		hud.Sessions = scanSessions(homeDir)
	}

	return hud
}

// RefreshResult holds results from a background data refresh.
type RefreshResult struct {
	Windows    []RateLimitWindow
	Sessions   []Session
	DataSource DataSource
	APISuccess bool // true if API call succeeded
}

// FetchRefreshData performs all I/O for data refresh and returns the results.
// Safe to call from any goroutine (does not modify shared state).
func FetchRefreshData(plan PlanType) RefreshResult {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return RefreshResult{}
	}

	var result RefreshResult

	// Try direct API first for near-real-time accuracy, fall back to cache
	if apiResp := fetchUsageFromAPI(homeDir); apiResp != nil {
		result.Windows = buildWindowsFromAPI(apiResp, plan)
		result.DataSource = DataSourceAPI
		result.APISuccess = true
	} else if cache := readUsageCache(homeDir); cache != nil {
		result.Windows = buildWindows(cache, plan)
		result.DataSource = DataSourceCache
	} else {
		result.DataSource = DataSourceNone
	}

	// Re-scan sessions to detect new or closed ones
	result.Sessions = scanSessions(homeDir)
	return result
}

// RefreshData re-reads live data sources and updates the existing HUDData in place.
// Used for initial load. For periodic refresh, use FetchRefreshData + async pattern.
func RefreshData(data *HUDData) {
	result := FetchRefreshData(data.Usage.Plan)
	if result.Windows != nil {
		data.Usage.Windows = result.Windows
	}
	data.Sessions = result.Sessions
	data.DataSource = result.DataSource
	if result.APISuccess {
		data.LastAPISuccess = time.Now()
		data.APIFailCount = 0
	} else {
		data.APIFailCount++
	}
}
