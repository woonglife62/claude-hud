package data

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"claude-hud/internal/model"
	"claude-hud/internal/platform"
)

var scanCacheMu sync.Mutex
var scanCache = make(map[string]*model.FileCache)

// closedSessionsMu guards closedSessions tracking state
var closedSessionsMu sync.Mutex

// prevSessionKeys holds the DirKeys seen in the last scan
var prevSessionKeys = make(map[string]model.Session)

// closedSessions accumulates sessions that disappeared, keyed by DirKey
var closedSessions = make(map[string]model.Session)

// ScanSessionAgents reads the tail of a JSONL file to extract agent lifecycle,
// active skills/strategies, and session metadata. It tracks both tool_use (agent start)
// and tool_result (agent completion) blocks to determine running vs completed status.
func ScanSessionAgents(jsonlPath string) model.ScanResult {
	const tailSize = 256 * 1024 // 256KB for richer history

	f, err := os.Open(jsonlPath)
	if err != nil {
		return model.ScanResult{}
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return model.ScanResult{}
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
		return model.ScanResult{}
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
	var lastSkill *model.SkillInfo
	sessionModel := ""
	messageCount := 0

	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry model.JsonlLine
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

		ts := model.ParseISO(entry.Timestamp)
		if ts.IsZero() {
			ts = time.Now()
		}

		// Process each content block
		for _, rawBlock := range entry.Message.Content {
			var block model.ContentBlock
			if json.Unmarshal(rawBlock, &block) != nil {
				continue
			}

			switch block.Type {
			case "tool_use":
				if block.Name == "Task" || block.Name == "proxy_Task" {
					var input model.TaskToolInput
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
					var input model.SkillToolInput
					if json.Unmarshal(block.Input, &input) == nil && input.Skill != "" {
						skillName := strings.TrimPrefix(input.Skill, "oh-my-claudecode:")
						lastSkill = &model.SkillInfo{
							Name: skillName,
							Args: input.Args,
						}
					}
				}

			case "tool_result":
				if block.ToolUseID == "" {
					continue
				}
				contentStr := ExtractToolResultText(block.Content, block.Text)

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

	// Stale agent detection: running > 30 min -> mark completed
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
	var resultAgents []model.Agent

	for _, a := range running {
		if len(resultAgents) >= maxAgents {
			break
		}
		displayName := strings.TrimPrefix(a.agentType, "oh-my-claudecode:")
		resultAgents = append(resultAgents, model.Agent{
			Name:   displayName,
			Model:  a.model,
			Task:   a.description,
			Status: model.StatusRunning,
		})
	}

	remaining := maxAgents - len(resultAgents)
	for i := 0; i < remaining && i < len(completed); i++ {
		displayName := strings.TrimPrefix(completed[i].agentType, "oh-my-claudecode:")
		resultAgents = append(resultAgents, model.Agent{
			Name:   displayName,
			Model:  completed[i].model,
			Task:   completed[i].description,
			Status: model.StatusIdle,
		})
	}

	return model.ScanResult{
		Agents:       resultAgents,
		ActiveSkill:  lastSkill,
		SessionModel: sessionModel,
		MessageCount: messageCount,
	}
}

// ExtractToolResultText extracts text content from a tool_result content field.
// Content can be a string, or an array of text blocks.
func ExtractToolResultText(content json.RawMessage, text string) string {
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

// ScanSessionAgentsWithCache performs an incremental scan when possible.
// If the file grew since last scan (cached != nil, newSize > cached.FileSize),
// only the new bytes are read and merged with cached agents.
// Otherwise falls back to a full ScanSessionAgents call.
func ScanSessionAgentsWithCache(jsonlPath string, newSize int64, cached *model.FileCache) model.ScanResult {
	if cached == nil || newSize <= cached.FileSize {
		return ScanSessionAgents(jsonlPath)
	}

	f, err := os.Open(jsonlPath)
	if err != nil {
		return ScanSessionAgents(jsonlPath)
	}
	defer f.Close()

	if _, err := f.Seek(cached.FileSize, 0); err != nil {
		return ScanSessionAgents(jsonlPath)
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
		return model.ScanResult{
			Agents:       cached.Result.Agents,
			ActiveSkill:  cached.Result.Skill,
			SessionModel: cached.Result.Model,
			MessageCount: cached.Result.Messages,
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
	for _, a := range cached.Result.Agents {
		status := "completed"
		if a.Status == model.StatusRunning {
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
	lastSkill := cached.Result.Skill
	sessionModel := cached.Result.Model
	messageCount := cached.Result.Messages

	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry model.JsonlLine
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
		ts := model.ParseISO(entry.Timestamp)
		if ts.IsZero() {
			ts = time.Now()
		}
		for _, rawBlock := range entry.Message.Content {
			var block model.ContentBlock
			if json.Unmarshal(rawBlock, &block) != nil {
				continue
			}
			switch block.Type {
			case "tool_use":
				if block.Name == "Task" || block.Name == "proxy_Task" {
					var input model.TaskToolInput
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
					var input model.SkillToolInput
					if json.Unmarshal(block.Input, &input) == nil && input.Skill != "" {
						skillName := strings.TrimPrefix(input.Skill, "oh-my-claudecode:")
						lastSkill = &model.SkillInfo{Name: skillName, Args: input.Args}
					}
				}
			case "tool_result":
				if block.ToolUseID == "" {
					continue
				}
				contentStr := ExtractToolResultText(block.Content, block.Text)
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
	var resultAgents []model.Agent
	for _, a := range running {
		if len(resultAgents) >= maxAgents {
			break
		}
		displayName := strings.TrimPrefix(a.agentType, "oh-my-claudecode:")
		resultAgents = append(resultAgents, model.Agent{
			Name:   displayName,
			Model:  a.model,
			Task:   a.description,
			Status: model.StatusRunning,
		})
	}
	remaining := maxAgents - len(resultAgents)
	for i := 0; i < remaining && i < len(completed); i++ {
		displayName := strings.TrimPrefix(completed[i].agentType, "oh-my-claudecode:")
		resultAgents = append(resultAgents, model.Agent{
			Name:   displayName,
			Model:  completed[i].model,
			Task:   completed[i].description,
			Status: model.StatusIdle,
		})
	}

	return model.ScanResult{
		Agents:       resultAgents,
		ActiveSkill:  lastSkill,
		SessionModel: sessionModel,
		MessageCount: messageCount,
	}
}

// ScanSessionsResult holds active and recently-closed sessions from a scan.
type ScanSessionsResult struct {
	Active []model.Session
	Closed []model.Session
}

// ScanSessions scans ~/.claude/projects/*/ for active JSONL transcript files.
// Running = modified within last 5 min. Idle = modified within last 30 min.
// Uses mtime caching to skip re-scanning unchanged files.
// Also detects sessions that disappeared since the last scan (closed sessions).
func ScanSessions(homeDir string) []model.Session {
	r := ScanSessionsFull(homeDir, 5)
	return r.Active
}

// ScanSessionsFull is like ScanSessions but also returns recently-closed sessions.
// retentionMin controls how long (in minutes) closed sessions are kept.
func ScanSessionsFull(homeDir string, retentionMin int) ScanSessionsResult {
	scanStart := time.Now()

	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return ScanSessionsResult{}
	}

	now := time.Now()
	runningThreshold := 5 * time.Minute
	idleThreshold := 30 * time.Minute

	var sessions []model.Session
	sessionID := 1

	// Track which paths are still active so we can evict stale cache entries.
	seenPaths := make(map[string]bool)
	// Track which DirKeys are active this scan.
	activeDirKeys := make(map[string]bool)

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

		var status model.SessionStatus
		if age <= runningThreshold {
			status = model.StatusRunning
		} else {
			status = model.StatusIdle
		}

		// Extract a short project name from the directory name.
		projectName := ExtractProjectName(dirName)

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
		activeDirKeys[dirName] = true

		// Check mtime cache before scanning
		scanCacheMu.Lock()
		cached, hasCached := scanCache[newestFile]
		scanCacheMu.Unlock()

		var sr model.ScanResult
		if hasCached && cached.ModTime.Equal(newestFileMod) {
			// Cache hit: file unchanged, reuse result
			sr = model.ScanResult{
				Agents:       cached.Result.Agents,
				ActiveSkill:  cached.Result.Skill,
				SessionModel: cached.Result.Model,
				MessageCount: cached.Result.Messages,
			}
		} else {
			// Cache miss or file changed: scan the file
			sr = ScanSessionAgentsWithCache(newestFile, newestFileSize, cached)

			// Store result in cache
			scanCacheMu.Lock()
			scanCache[newestFile] = &model.FileCache{
				ModTime:  newestFileMod,
				FileSize: newestFileSize,
				Result: &model.SessionScanResult{
					Agents:   sr.Agents,
					Skill:    sr.ActiveSkill,
					Model:    sr.SessionModel,
					Messages: sr.MessageCount,
				},
			}
			scanCacheMu.Unlock()
		}

		mdl := sr.SessionModel
		if mdl == "" {
			mdl = "Claude"
		}

		sess := model.Session{
			ID:          sessionID,
			DirKey:      dirName,
			Type:        model.SessionCode,
			Model:       mdl,
			Agents:      sr.Agents,
			ActiveSkill: sr.ActiveSkill,
			Status:      status,
			StartTime:   newestTime,
			Messages:    sr.MessageCount,
			ProjectName: projectName,
		}
		sessions = append(sessions, sess)
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

	// Detect closed sessions: sessions seen previously but not in this scan.
	retention := time.Duration(retentionMin) * time.Minute
	closedSessionsMu.Lock()
	// Any prev session not in active list -> mark closed (if not already tracked)
	for dirKey, prev := range prevSessionKeys {
		if !activeDirKeys[dirKey] {
			if _, alreadyClosed := closedSessions[dirKey]; !alreadyClosed {
				prev.ClosedAt = now
				closedSessions[dirKey] = prev
			}
		}
	}
	// Prune closed sessions older than retention period
	for dirKey, cs := range closedSessions {
		if now.Sub(cs.ClosedAt) > retention {
			delete(closedSessions, dirKey)
		}
	}
	// Update prevSessionKeys for next scan
	prevSessionKeys = make(map[string]model.Session, len(sessions))
	for _, s := range sessions {
		prevSessionKeys[s.DirKey] = s
	}
	// Collect current closed sessions slice
	var closedList []model.Session
	for _, cs := range closedSessions {
		closedList = append(closedList, cs)
	}
	closedSessionsMu.Unlock()

	platform.Log("scanSessions: %d active, %d closed, scanned in %v", len(sessions), len(closedList), time.Since(scanStart))
	return ScanSessionsResult{Active: sessions, Closed: closedList}
}
