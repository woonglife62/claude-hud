package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"claude-hud/internal/model"
)

// writeJSONL writes JSONL lines to a temp file and returns its path.
func writeJSONL(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp jsonl: %v", err)
	}
	defer f.Close()
	for _, l := range lines {
		f.WriteString(l + "\n")
	}
	return path
}

func TestScanSessionAgents_Empty(t *testing.T) {
	path := writeJSONL(t, nil)
	result := ScanSessionAgents(path)
	if len(result.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(result.Agents))
	}
	if result.MessageCount != 0 {
		t.Errorf("expected 0 messages, got %d", result.MessageCount)
	}
}

func TestScanSessionAgents_BasicMessage(t *testing.T) {
	line := buildAssistantLine("claude-opus-4-6", nil)
	path := writeJSONL(t, []string{line})
	result := ScanSessionAgents(path)
	if result.SessionModel != "claude-opus-4-6" {
		t.Errorf("expected model claude-opus-4-6, got %q", result.SessionModel)
	}
	if result.MessageCount != 1 {
		t.Errorf("expected 1 message, got %d", result.MessageCount)
	}
}

func TestScanSessionAgents_SkillDetection(t *testing.T) {
	skillBlock := buildSkillToolUse("toolu_001", "ralph", "")
	line := buildAssistantLine("claude-sonnet-4-6", []json.RawMessage{skillBlock})
	path := writeJSONL(t, []string{line})
	result := ScanSessionAgents(path)
	if result.ActiveSkill == nil {
		t.Fatal("expected ActiveSkill, got nil")
	}
	if result.ActiveSkill.Name != "ralph" {
		t.Errorf("expected skill ralph, got %q", result.ActiveSkill.Name)
	}
}

func TestScanSessionAgents_AgentLifecycle(t *testing.T) {
	// Tool use starts agent
	taskBlock := buildTaskToolUse("toolu_002", "oh-my-claudecode:executor", "sonnet", "fix bug")
	startLine := buildAssistantLine("claude-opus-4-6", []json.RawMessage{taskBlock})

	// Tool result completes it
	resultBlock := buildToolResult("toolu_002", "done")
	resultLine := buildUserLine([]json.RawMessage{resultBlock})

	path := writeJSONL(t, []string{startLine, resultLine})
	result := ScanSessionAgents(path)

	if len(result.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(result.Agents))
	}
	if result.Agents[0].Status != model.StatusIdle {
		t.Errorf("expected StatusIdle (completed), got %v", result.Agents[0].Status)
	}
}

func TestScanSessionAgentsWithCache_CacheHit(t *testing.T) {
	path := writeJSONL(t, []string{buildAssistantLine("claude-opus-4-6", nil)})
	info, _ := os.Stat(path)

	cachedResult := &model.SessionScanResult{
		Model:    "claude-opus-4-6",
		Messages: 5,
	}
	cached := &model.FileCache{
		ModTime:  info.ModTime(),
		FileSize: info.Size(),
		Result:   cachedResult,
	}

	// Same mtime and size -> should use full scan (cache hit returns cached data)
	result := ScanSessionAgentsWithCache(path, info.Size(), cached)
	// When modtime matches, ScanSessionAgentsWithCache falls through to ScanSessionAgents
	// which reads the file; the cache is used only for the incremental path (newSize > cached.FileSize)
	if result.SessionModel == "" {
		t.Error("expected non-empty model from scan")
	}
}

func TestScanSessionAgentsWithCache_IncrementalScan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	firstLine := buildAssistantLine("claude-opus-4-6", nil) + "\n"
	if err := os.WriteFile(path, []byte(firstLine), 0644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)

	cached := &model.FileCache{
		ModTime:  info.ModTime(),
		FileSize: info.Size(),
		Result: &model.SessionScanResult{
			Model:    "claude-opus-4-6",
			Messages: 1,
		},
	}

	// Append a new line to simulate growth
	secondLine := buildAssistantLine("claude-sonnet-4-6", nil) + "\n"
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString(secondLine)
	f.Close()

	newInfo, _ := os.Stat(path)
	result := ScanSessionAgentsWithCache(path, newInfo.Size(), cached)
	// Incremental scan merges new messages
	if result.MessageCount < 1 {
		t.Errorf("expected at least 1 message from incremental scan, got %d", result.MessageCount)
	}
}

func TestExtractProjectName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"C--Users-alice-go-src-myproject", "myproject"},
		{"C--Users-bob-projects-foo-bar", "foo-bar"},
		{"simple", "simple"},
	}
	for _, tc := range tests {
		got := ExtractProjectName(tc.input)
		if got != tc.want {
			t.Errorf("ExtractProjectName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- helpers ---

func buildAssistantLine(modelName string, content []json.RawMessage) string {
	if content == nil {
		content = []json.RawMessage{}
	}
	entry := map[string]interface{}{
		"type":      "assistant",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"message": map[string]interface{}{
			"model":   modelName,
			"role":    "assistant",
			"content": content,
		},
	}
	b, _ := json.Marshal(entry)
	return string(b)
}

func buildUserLine(content []json.RawMessage) string {
	if content == nil {
		content = []json.RawMessage{}
	}
	entry := map[string]interface{}{
		"type":      "user",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"message": map[string]interface{}{
			"role":    "user",
			"content": content,
		},
	}
	b, _ := json.Marshal(entry)
	return string(b)
}

func buildTaskToolUse(id, subagentType, mdl, desc string) json.RawMessage {
	block := map[string]interface{}{
		"type": "tool_use",
		"id":   id,
		"name": "Task",
		"input": map[string]interface{}{
			"subagent_type": subagentType,
			"model":         mdl,
			"description":   desc,
		},
	}
	b, _ := json.Marshal(block)
	return json.RawMessage(b)
}

func buildSkillToolUse(id, skill, args string) json.RawMessage {
	block := map[string]interface{}{
		"type": "tool_use",
		"id":   id,
		"name": "Skill",
		"input": map[string]interface{}{
			"skill": skill,
			"args":  args,
		},
	}
	b, _ := json.Marshal(block)
	return json.RawMessage(b)
}

func buildToolResult(toolUseID, text string) json.RawMessage {
	block := map[string]interface{}{
		"type":        "tool_result",
		"tool_use_id": toolUseID,
		"text":        text,
	}
	b, _ := json.Marshal(block)
	return json.RawMessage(b)
}
