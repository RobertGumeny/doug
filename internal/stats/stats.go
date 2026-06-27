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
	Phase                string  `json:"phase"`
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
	Phase           string
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
func FromRunResponse(phase agent.RunPhase, taskID string, attempt int, completedAt time.Time, resp agent.RunResponse) RunStats {
	record := RunStats{
		Phase:                string(phase),
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

// WriteRunStats persists a RunStats record under the attempt-scoped forensic
// layout: .doug/logs/epics/<epic>/<task>/attempt-N/stats.json.
func WriteRunStats(logsDir, epicID string, record RunStats) (string, error) {
	if logsDir == "" {
		return "", fmt.Errorf("logs directory is required")
	}
	if epicID == "" {
		epicID = "runtime"
	}
	taskID := record.TaskID
	if taskID == "" {
		taskID = "task"
	}
	attempt := record.Attempt
	if attempt < 0 {
		attempt = 0
	}
	statsDir := filepath.Join(logsDir, "epics", sanitizePathComponent(epicID), sanitizePathComponent(taskID), fmt.Sprintf("attempt-%d", attempt))
	if err := os.MkdirAll(statsDir, 0o755); err != nil {
		return "", fmt.Errorf("create stats directory: %w", err)
	}
	path := filepath.Join(statsDir, "stats.json")
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

// LoadSummary reads Doug-owned stats JSON files from the forensic epics tree,
// plus legacy .doug/logs/stats records for backward compatibility. If epicID is
// non-empty, only that epic/bucket is read. Missing stats directories produce an
// empty summary.
func LoadSummary(logsDir, epicID string) (Summary, error) {
	if logsDir == "" {
		return Summary{}, fmt.Errorf("logs directory is required")
	}

	files, err := statsFiles(logsDir, epicID)
	if err != nil {
		return Summary{}, err
	}
	return loadStatsFiles(files)
}

type statsFile struct {
	path   string
	epicID string
}

func statsFiles(logsDir, epicID string) ([]statsFile, error) {
	var files []statsFile
	forensicRoot := filepath.Join(logsDir, "epics")
	legacyRoot := filepath.Join(logsDir, "stats")

	addForensicEpic := func(epicPath string) error {
		entries, err := os.ReadDir(epicPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("read forensic epic stats directory %s: %w", epicPath, err)
		}
		bucket := filepath.Base(epicPath)
		for _, taskEntry := range entries {
			if !taskEntry.IsDir() {
				continue
			}
			attemptRoot := filepath.Join(epicPath, taskEntry.Name())
			attempts, err := os.ReadDir(attemptRoot)
			if err != nil {
				return fmt.Errorf("read forensic task stats directory %s: %w", attemptRoot, err)
			}
			for _, attemptEntry := range attempts {
				if !attemptEntry.IsDir() || !strings.HasPrefix(attemptEntry.Name(), "attempt-") {
					continue
				}
				path := filepath.Join(attemptRoot, attemptEntry.Name(), "stats.json")
				if _, err := os.Stat(path); err == nil {
					files = append(files, statsFile{path: path, epicID: bucket})
				} else if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("inspect stats file %s: %w", path, err)
				}
			}
		}
		return nil
	}

	addLegacyBucket := func(bucketPath string) error {
		entries, err := os.ReadDir(bucketPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("read stats directory %s: %w", bucketPath, err)
		}
		bucket := filepath.Base(bucketPath)
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			files = append(files, statsFile{path: filepath.Join(bucketPath, entry.Name()), epicID: bucket})
		}
		return nil
	}

	if epicID != "" {
		bucket := sanitizePathComponent(epicID)
		if err := addForensicEpic(filepath.Join(forensicRoot, bucket)); err != nil {
			return nil, err
		}
		if err := addLegacyBucket(filepath.Join(legacyRoot, bucket)); err != nil {
			return nil, err
		}
		sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
		return files, nil
	}

	for _, root := range []struct {
		path string
		add  func(string) error
	}{
		{path: forensicRoot, add: addForensicEpic},
		{path: legacyRoot, add: addLegacyBucket},
	} {
		entries, err := os.ReadDir(root.path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read stats root %s: %w", root.path, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				if err := root.add(filepath.Join(root.path, entry.Name())); err != nil {
					return nil, err
				}
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files, nil
}

func loadStatsFiles(files []statsFile) (Summary, error) {
	byTask := make(map[string]*TaskSummary)
	firstResponseCounts := make(map[string]int64)
	for _, file := range files {
		record, err := readRunStats(file.path)
		if err != nil {
			return Summary{}, err
		}
		phase := record.Phase
		if phase == "" {
			phase = string(agent.RunPhaseRuntime)
		}
		key := file.epicID + "\x00" + phase + "\x00" + record.TaskID
		row := byTask[key]
		if row == nil {
			row = &TaskSummary{EpicID: file.epicID, Phase: phase, TaskID: record.TaskID}
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
		if rows[i].Phase != rows[j].Phase {
			return rows[i].Phase < rows[j].Phase
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
