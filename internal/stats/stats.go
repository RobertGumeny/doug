package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// TaskSummary is the per-task aggregation displayed by doug stats.
type TaskSummary struct {
	EpicID          string
	TaskID          string
	Runs            int
	InputTokens     int64
	OutputTokens    int64
	CacheTokens     int64
	CostUSD         float64
	DurationMs      int64
	FirstResponseMs int64
}

// Summary contains all task rows and totals for a stats query.
type Summary struct {
	Rows   []TaskSummary
	Totals TaskSummary
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

// LoadSummary reads Doug-owned stats JSON files from .doug/logs/stats and
// returns per-task rows plus totals. If epicID is non-empty, only that epic's
// stats directory is read. Missing stats directories produce an empty summary.
func LoadSummary(logsDir, epicID string) (Summary, error) {
	if logsDir == "" {
		return Summary{}, fmt.Errorf("logs directory is required")
	}
	statsRoot := filepath.Join(logsDir, "stats")
	if epicID != "" {
		return loadStatsDirs([]string{filepath.Join(statsRoot, sanitizePathComponent(epicID))})
	}

	entries, err := os.ReadDir(statsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return Summary{}, nil
		}
		return Summary{}, fmt.Errorf("read stats directory: %w", err)
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(statsRoot, entry.Name()))
		}
	}
	sort.Strings(dirs)
	return loadStatsDirs(dirs)
}

func loadStatsDirs(dirs []string) (Summary, error) {
	byTask := make(map[string]*TaskSummary)
	firstResponseCounts := make(map[string]int64)
	for _, dir := range dirs {
		epicID := filepath.Base(dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Summary{}, fmt.Errorf("read stats directory %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			record, err := readRunStats(filepath.Join(dir, entry.Name()))
			if err != nil {
				return Summary{}, err
			}
			key := epicID + "\x00" + record.TaskID
			row := byTask[key]
			if row == nil {
				row = &TaskSummary{EpicID: epicID, TaskID: record.TaskID}
				byTask[key] = row
			}
			row.Runs++
			row.InputTokens += record.InputTokens
			row.OutputTokens += record.OutputTokens
			row.CacheTokens += record.CacheTokens
			row.CostUSD += record.CostUSD
			row.DurationMs += record.DurationMs
			if record.FirstResponseMs > 0 {
				row.FirstResponseMs += record.FirstResponseMs
				firstResponseCounts[key]++
			}
		}
	}

	rows := make([]TaskSummary, 0, len(byTask))
	for key, row := range byTask {
		if count := firstResponseCounts[key]; count > 0 {
			row.FirstResponseMs /= count
		}
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].EpicID != rows[j].EpicID {
			return rows[i].EpicID < rows[j].EpicID
		}
		return rows[i].TaskID < rows[j].TaskID
	})

	var summary Summary
	summary.Rows = rows
	var firstResponseTotal int64
	var firstResponseRows int64
	for _, row := range rows {
		summary.Totals.Runs += row.Runs
		summary.Totals.InputTokens += row.InputTokens
		summary.Totals.OutputTokens += row.OutputTokens
		summary.Totals.CacheTokens += row.CacheTokens
		summary.Totals.CostUSD += row.CostUSD
		summary.Totals.DurationMs += row.DurationMs
		if row.FirstResponseMs > 0 {
			firstResponseTotal += row.FirstResponseMs
			firstResponseRows++
		}
	}
	if firstResponseRows > 0 {
		summary.Totals.FirstResponseMs = firstResponseTotal / firstResponseRows
	}
	return summary, nil
}

func readRunStats(path string) (RunStats, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RunStats{}, fmt.Errorf("read stats file %s: %w", path, err)
	}
	var record RunStats
	if err := json.Unmarshal(data, &record); err != nil {
		return RunStats{}, fmt.Errorf("parse stats file %s: %w", path, err)
	}
	return record, nil
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
