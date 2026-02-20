package data

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"claude-hud/internal/model"
	"claude-hud/internal/platform"
)

// httpClient is a package-level client with a short timeout to avoid blocking the UI.
var httpClient = &http.Client{Timeout: 3 * time.Second}

// authBackoffSeconds holds the backoff duration in seconds after consecutive auth failures.
// Sequence: 0, 30, 60, 120, 300, 600 (capped at 10 minutes).
var authBackoffSeconds = []int{0, 30, 60, 120, 300, 600}

// lastAuthFailTime tracks when the last auth failure occurred.
var lastAuthFailTime time.Time

// consecutiveAuthFails tracks the number of consecutive 401/403 failures.
var consecutiveAuthFails int

// shouldSkipAPIDueToAuthBackoff returns true if we should skip the API call
// because we are still within the exponential backoff window after auth failures.
func shouldSkipAPIDueToAuthBackoff() bool {
	if consecutiveAuthFails == 0 {
		return false
	}
	idx := consecutiveAuthFails - 1
	if idx >= len(authBackoffSeconds) {
		idx = len(authBackoffSeconds) - 1
	}
	backoff := time.Duration(authBackoffSeconds[idx]) * time.Second
	return time.Since(lastAuthFailTime) < backoff
}

// APIFetchResult holds the result of a FetchUsageFromAPI call including auth error info.
type APIFetchResult struct {
	Response     *model.APIUsageResponse
	TokenExpired bool
	AuthError    string
}

// FetchUsageFromAPI calls the Anthropic OAuth usage endpoint and returns the parsed
// response, or nil on any error (network, auth, parse, missing token).
func FetchUsageFromAPI(homeDir string) *model.APIUsageResponse {
	result := FetchUsageFromAPIWithAuthInfo(homeDir)
	return result.Response
}

// FetchUsageFromAPIWithAuthInfo calls the Anthropic OAuth usage endpoint and returns
// full result including auth error information.
func FetchUsageFromAPIWithAuthInfo(homeDir string) APIFetchResult {
	if shouldSkipAPIDueToAuthBackoff() {
		platform.Log("  [usage] Skipping API call (auth backoff: %ds, fails=%d)",
			authBackoffSeconds[min(consecutiveAuthFails-1, len(authBackoffSeconds)-1)],
			consecutiveAuthFails)
		return APIFetchResult{TokenExpired: true, AuthError: "인증 만료 (백오프 중)"}
	}

	creds := ReadCredentials(homeDir)
	if creds == nil || creds.ClaudeAiOauth.AccessToken == "" {
		platform.Log("  [usage] No credentials or access token")
		return APIFetchResult{}
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		platform.Log("  [usage] Failed to create request: %v", err)
		return APIFetchResult{}
	}
	req.Header.Set("Authorization", "Bearer "+creds.ClaudeAiOauth.AccessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	resp, err := httpClient.Do(req)
	if err != nil {
		platform.Log("  [usage] API request failed: %v", err)
		return APIFetchResult{}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		consecutiveAuthFails++
		lastAuthFailTime = time.Now()
		errMsg := fmt.Sprintf("인증 만료 (HTTP %d)", resp.StatusCode)
		platform.Log("  [usage] Auth failure %d: %s", consecutiveAuthFails, errMsg)
		return APIFetchResult{TokenExpired: true, AuthError: errMsg}
	}

	if resp.StatusCode != http.StatusOK {
		platform.Log("  [usage] API returned status %d", resp.StatusCode)
		return APIFetchResult{}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		platform.Log("  [usage] Failed to read response body: %v", err)
		return APIFetchResult{}
	}

	var usage model.APIUsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		platform.Log("  [usage] Failed to parse JSON: %v", err)
		return APIFetchResult{}
	}

	// Successful auth: reset backoff counters
	consecutiveAuthFails = 0
	lastAuthFailTime = time.Time{}

	platform.Log("  [usage] API OK: 5h=%.1f%% reset=%s, 7d=%.1f%% reset=%s",
		usage.FiveHour.Utilization, usage.FiveHour.ResetsAt,
		usage.SevenDay.Utilization, usage.SevenDay.ResetsAt)
	return APIFetchResult{Response: &usage}
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BuildModelBreakdown aggregates per-model token counts from an API response.
// It uses the 5-hour window's model breakdown (most granular / recent).
func BuildModelBreakdown(resp *model.APIUsageResponse) map[string]int64 {
	if resp == nil || len(resp.FiveHour.Models) == 0 {
		return nil
	}
	breakdown := make(map[string]int64, len(resp.FiveHour.Models))
	for _, m := range resp.FiveHour.Models {
		if m.ModelID != "" {
			breakdown[m.ModelID] += m.TokensUsed
		}
	}
	return breakdown
}

// BuildWindowsFromAPI converts an APIUsageResponse to RateLimitWindow entries.
// plan is used to derive absolute token counts from the utilization percentage.
// NOTE: The API returns utilization as a percentage (0-100), not a decimal (0-1).
func BuildWindowsFromAPI(resp *model.APIUsageResponse, plan model.PlanType) []model.RateLimitWindow {
	max5h := model.PlanTokenLimit5h(plan)
	maxWeekly := model.PlanTokenLimitWeekly(plan)

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

	return []model.RateLimitWindow{
		{
			Label:      "5시간 사용량",
			UsagePct:   pct5h,
			TokensUsed: int(pct5h * float64(max5h)),
			TokensMax:  max5h,
			ResetAt:    model.ParseISO(resp.FiveHour.ResetsAt),
		},
		{
			Label:      "주간 사용량",
			UsagePct:   pctWeekly,
			TokensUsed: int(pctWeekly * float64(maxWeekly)),
			TokensMax:  maxWeekly,
			ResetAt:    model.ParseISO(resp.SevenDay.ResetsAt),
		},
	}
}

// ReadUsageCache reads and parses the OMC usage cache file.
// Returns nil if the file is missing, malformed, or stale (reset times in the past).
func ReadUsageCache(homeDir string) *model.UsageCacheData {
	path := filepath.Join(homeDir, ".claude", "plugins", "oh-my-claudecode", ".usage-cache.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cache model.UsageCacheData
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil
	}
	if cache.Error {
		return nil
	}
	// Check staleness: if either reset time is in the past, the cache is outdated
	now := time.Now()
	fiveHourReset := model.ParseISO(cache.Data.FiveHourResetsAt)
	weeklyReset := model.ParseISO(cache.Data.WeeklyResetsAt)
	if (!fiveHourReset.IsZero() && fiveHourReset.Before(now)) ||
		(!weeklyReset.IsZero() && weeklyReset.Before(now)) {
		platform.Log("  [usage] Cache stale: 5h reset=%s, weekly reset=%s (both in past)",
			cache.Data.FiveHourResetsAt, cache.Data.WeeklyResetsAt)
		return nil
	}
	platform.Log("  [usage] Cache OK: 5h=%d%%, weekly=%d%%", cache.Data.FiveHourPercent, cache.Data.WeeklyPercent)
	return &cache
}

// ReadCredentials reads and parses ~/.claude/.credentials.json.
// Returns nil if missing or malformed.
func ReadCredentials(homeDir string) *model.CredentialsData {
	path := filepath.Join(homeDir, ".claude", ".credentials.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var creds model.CredentialsData
	if err := json.Unmarshal(raw, &creds); err != nil {
		return nil
	}
	return &creds
}

// BuildWindows constructs RateLimitWindow entries from cache data.
// plan is used to derive absolute token counts from the percentage.
func BuildWindows(cache *model.UsageCacheData, plan model.PlanType) []model.RateLimitWindow {
	var windows []model.RateLimitWindow

	fiveHourReset := model.ParseISO(cache.Data.FiveHourResetsAt)
	weeklyReset := model.ParseISO(cache.Data.WeeklyResetsAt)

	max5h := model.PlanTokenLimit5h(plan)
	maxWeekly := model.PlanTokenLimitWeekly(plan)

	pct5h := float64(cache.Data.FiveHourPercent) / 100.0
	pctWeekly := float64(cache.Data.WeeklyPercent) / 100.0

	windows = append(windows, model.RateLimitWindow{
		Label:      "5시간 사용량",
		UsagePct:   pct5h,
		TokensUsed: int(pct5h * float64(max5h)),
		TokensMax:  max5h,
		ResetAt:    fiveHourReset,
	})

	windows = append(windows, model.RateLimitWindow{
		Label:      "주간 사용량",
		UsagePct:   pctWeekly,
		TokensUsed: int(pctWeekly * float64(maxWeekly)),
		TokensMax:  maxWeekly,
		ResetAt:    weeklyReset,
	})

	return windows
}

// ExtractProjectName derives a short human-readable project name from a
// ~/.claude/projects directory name such as "C--Users-openerd-go-src-foo-bar".
func ExtractProjectName(dirName string) string {
	// Split on "--" to remove drive prefix like "C--"
	parts := strings.SplitN(dirName, "--", 2)
	remainder := dirName
	if len(parts) == 2 {
		remainder = parts[1]
	}

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

// LoadRealData reads live data from Claude config files and returns a populated HUDData.
func LoadRealData() *model.HUDData {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	hud := &model.HUDData{
		Account: "Claude User",
		Usage: model.UsageData{
			Plan:  model.PlanFree,
			Model: "Opus 4",
		},
	}

	// Read credentials for plan type
	if homeDir != "" {
		if creds := ReadCredentials(homeDir); creds != nil {
			tier := creds.ClaudeAiOauth.RateLimitTier
			hud.Usage.Plan = model.RateLimitTierToPlan(tier)
		}
	}

	// Try direct API first for near-real-time accuracy, fall back to cache
	if homeDir != "" {
		apiResult := FetchUsageFromAPIWithAuthInfo(homeDir)
		if apiResult.TokenExpired {
			hud.TokenExpired = true
			hud.LastAuthError = apiResult.AuthError
			hud.APIFailCount++
			// Fall back to cache
			if cache := ReadUsageCache(homeDir); cache != nil {
				hud.Usage.Windows = BuildWindows(cache, hud.Usage.Plan)
				hud.DataSource = model.DataSourceCache
				platform.Log("  [usage] Source: OMC cache (after auth failure)")
			} else {
				hud.DataSource = model.DataSourceNone
			}
		} else if apiResult.Response != nil {
			hud.Usage.Windows = BuildWindowsFromAPI(apiResult.Response, hud.Usage.Plan)
			hud.Usage.ModelBreakdown = BuildModelBreakdown(apiResult.Response)
			hud.DataSource = model.DataSourceAPI
			hud.LastAPISuccess = time.Now()
			hud.APIFailCount = 0
			hud.TokenExpired = false
			hud.LastAuthError = ""
			platform.Log("  [usage] Source: API direct")
		} else if cache := ReadUsageCache(homeDir); cache != nil {
			hud.Usage.Windows = BuildWindows(cache, hud.Usage.Plan)
			hud.DataSource = model.DataSourceCache
			hud.APIFailCount++
			platform.Log("  [usage] Source: OMC cache")
		} else {
			hud.DataSource = model.DataSourceNone
			hud.APIFailCount++
			platform.Log("  [usage] Source: none (using defaults)")
		}
	}

	// Default windows if cache unavailable
	if len(hud.Usage.Windows) == 0 {
		now := time.Now()
		max5h := model.PlanTokenLimit5h(hud.Usage.Plan)
		maxWeekly := model.PlanTokenLimitWeekly(hud.Usage.Plan)
		hud.Usage.Windows = []model.RateLimitWindow{
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
		hud.Sessions = ScanSessions(homeDir)
	}

	return hud
}

// FetchRefreshData performs all I/O for data refresh and returns the results.
// Safe to call from any goroutine (does not modify shared state).
func FetchRefreshData(plan model.PlanType) model.RefreshResult {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return model.RefreshResult{}
	}

	var result model.RefreshResult

	// Try direct API first for near-real-time accuracy, fall back to cache
	apiResult := FetchUsageFromAPIWithAuthInfo(homeDir)
	if apiResult.TokenExpired {
		result.TokenExpired = true
		result.LastAuthError = apiResult.AuthError
		// Fall back to cache
		if cache := ReadUsageCache(homeDir); cache != nil {
			result.Windows = BuildWindows(cache, plan)
			result.DataSource = model.DataSourceCache
		} else {
			result.DataSource = model.DataSourceNone
		}
	} else if apiResult.Response != nil {
		result.Windows = BuildWindowsFromAPI(apiResult.Response, plan)
		result.ModelBreakdown = BuildModelBreakdown(apiResult.Response)
		result.DataSource = model.DataSourceAPI
		result.APISuccess = true
	} else if cache := ReadUsageCache(homeDir); cache != nil {
		result.Windows = BuildWindows(cache, plan)
		result.DataSource = model.DataSourceCache
	} else {
		result.DataSource = model.DataSourceNone
	}

	// Re-scan sessions to detect new or closed ones
	scanResult := ScanSessionsFull(homeDir, 5)
	result.Sessions = scanResult.Active
	result.ClosedSessions = scanResult.Closed
	return result
}

// RefreshData re-reads live data sources and updates the existing HUDData in place.
// Used for initial load. For periodic refresh, use FetchRefreshData + async pattern.
func RefreshData(data *model.HUDData) {
	result := FetchRefreshData(data.Usage.Plan)
	if result.Windows != nil {
		data.Usage.Windows = result.Windows
		UpdateTrend(&data.Usage)
	}
	data.Sessions = result.Sessions
	data.ClosedSessions = result.ClosedSessions
	data.DataSource = result.DataSource
	if result.TokenExpired {
		data.TokenExpired = true
		data.LastAuthError = result.LastAuthError
		data.APIFailCount++
	} else if result.APISuccess {
		data.LastAPISuccess = time.Now()
		data.APIFailCount = 0
		data.TokenExpired = false
		data.LastAuthError = ""
		if result.ModelBreakdown != nil {
			data.Usage.ModelBreakdown = result.ModelBreakdown
		}
	} else {
		data.APIFailCount++
	}
}

// UpdateTrend appends the current 5h usage percentage to TrendHistory (max 10 entries)
// and derives a TrendArrow from the velocity over the last N samples.
func UpdateTrend(usage *model.UsageData) {
	if len(usage.Windows) == 0 {
		return
	}
	// Use first window (5h) usage percentage
	current := usage.Windows[0].UsagePct

	const maxHistory = 10
	usage.TrendHistory = append(usage.TrendHistory, current)
	if len(usage.TrendHistory) > maxHistory {
		usage.TrendHistory = usage.TrendHistory[len(usage.TrendHistory)-maxHistory:]
	}

	usage.TrendArrow = calcTrendArrow(usage.TrendHistory)
}

// calcTrendArrow computes a trend arrow from a history of usage percentages.
func calcTrendArrow(history []float64) string {
	if len(history) < 2 {
		return "→" // not enough data
	}
	// Compare last value to value N steps ago (up to 5 steps)
	n := len(history)
	look := 3
	if n-1 < look {
		look = n - 1
	}
	delta := history[n-1] - history[n-1-look]

	switch {
	case delta > 0.10:
		return "↑" // fast rise
	case delta > 0.04:
		return "↗" // moderate rise
	case delta < -0.10:
		return "↓" // fast drop
	case delta < -0.04:
		return "↘" // moderate drop
	default:
		return "→" // stable
	}
}
