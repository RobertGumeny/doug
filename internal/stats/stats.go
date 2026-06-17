package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/state"
)

// RunStats is Doug's normalized, phase-agnostic per-run stats record.
// Runtime runs populate token and cost fields from Pi's get_session_stats RPC;
// runtime observability fields are copied from agent.RunResponse.
type RunStats struct {
	TaskID               string  `json:"task_id"`
	Attempt              int     `json:"attempt"`
	SessionID            string  `json:"session_id,omitempty"`
	InputTokens          int64   `json:"input_tokens"`
	OutputTokens         int64   `json:"output_tokens"`
	CacheTokens          int64   `json:"cache_tokens"`
	CostUSD              float64 `json:"cost_usd"`
	FirstResponseMs      int64   `json:"first_response_ms"`
	ToolCallCount        int     `json:"tool_call_count"`
	ProviderFailureCount int     `json:"provider_failure_count"`
	DurationMs           int64   `json:"duration_ms"`
	CompletedAt          string  `json:"completed_at"`
}

// FromRunResponse builds a persisted stats record using Pi session stats for
// tokens/cost and RunResponse observability for first-response/tool/provider data.
func FromRunResponse(taskID string, attempt int, completedAt time.Time, resp agent.RunResponse) RunStats {
	record := RunStats{
		TaskID:               taskID,
		Attempt:              attempt,
		SessionID:            resp.SessionID,
		FirstResponseMs:      resp.FirstResponseMs,
		ToolCallCount:        resp.ToolCallCount,
		ProviderFailureCount: resp.ProviderFailures,
		DurationMs:           resp.Duration.Milliseconds(),
		CompletedAt:          completedAt.UTC().Format(time.RFC3339),
	}
	if resp.SessionStats != nil {
		record.InputTokens = resp.SessionStats.Tokens.Input
		record.OutputTokens = resp.SessionStats.Tokens.Output
		record.CacheTokens = resp.SessionStats.Tokens.CacheRead + resp.SessionStats.Tokens.CacheWrite
		record.CostUSD = resp.SessionStats.Cost
		if record.SessionID == "" {
			record.SessionID = resp.SessionStats.SessionID
		}
	}
	return record
}

// WriteRunStats persists a RunStats record under .doug/logs/stats/<epic>/.
func WriteRunStats(logsDir, epicID string, record RunStats) (string, error) {
	if logsDir == "" {
		return "", fmt.Errorf("logs directory is required")
	}
	if epicID == "" {
		epicID = "runtime"
	}
	statsDir := filepath.Join(logsDir, "stats", sanitizePathComponent(epicID))
	if err := os.MkdirAll(statsDir, 0o755); err != nil {
		return "", fmt.Errorf("create stats directory: %w", err)
	}
	name := fmt.Sprintf("stats-%s_attempt-%d.json", sanitizePathComponent(record.TaskID), record.Attempt)
	path := filepath.Join(statsDir, name)
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal run stats: %w", err)
	}
	data = append(data, '\n')
	if err := state.AtomicWrite(path, data); err != nil {
		return "", fmt.Errorf("write run stats: %w", err)
	}
	return path, nil
}

func sanitizePathComponent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "._")
	if out == "" {
		return "unknown"
	}
	return out
}
