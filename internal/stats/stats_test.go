package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
)

func TestFromRunResponseCopiesPiStatsAndRunObservability(t *testing.T) {
	completedAt := time.Date(2026, 6, 17, 10, 11, 12, 0, time.UTC)
	resp := agent.RunResponse{
		SessionID:        "pi-session-run",
		Duration:         1500 * time.Millisecond,
		FirstResponseMs:  250,
		ToolCallCount:    3,
		ProviderFailures: 2,
		SessionStats: &agent.PiSessionStats{
			Tokens: agent.PiSessionTokenStats{Input: 100, Output: 50, CacheRead: 20, CacheWrite: 5},
			Cost:   0.1234,
		},
	}

	got := FromRunResponse(agent.RunPhaseResearch, "EPIC-46-001", 2, completedAt, resp)

	if got.Phase != "research" || got.TaskID != "EPIC-46-001" || got.Attempt != 2 || got.SessionID != "pi-session-run" {
		t.Fatalf("identity fields = %+v", got)
	}
	if got.InputTokens != 100 || got.OutputTokens != 50 || got.CacheTokens != 25 || got.CostUSD != 0.1234 {
		t.Fatalf("token/cost fields = %+v", got)
	}
	if got.FirstResponseMs != 250 || got.ToolCallCount != 3 || got.ProviderFailureCount != 2 || got.DurationMs != 1500 {
		t.Fatalf("observability fields = %+v", got)
	}
	if got.CompletedAt != "2026-06-17T10:11:12Z" {
		t.Fatalf("CompletedAt = %q", got.CompletedAt)
	}
}

func TestWriteRunStatsPersistsDedicatedStatsFile(t *testing.T) {
	logsDir := filepath.Join(t.TempDir(), ".doug", "logs")
	record := RunStats{Phase: "runtime", TaskID: "EPIC-46-001", Attempt: 1, InputTokens: 10, CompletedAt: "2026-06-17T10:11:12Z"}

	path, err := WriteRunStats(logsDir, "EPIC-46", record)
	if err != nil {
		t.Fatalf("WriteRunStats: %v", err)
	}
	wantPath := filepath.Join(logsDir, "epics", "EPIC-46", "EPIC-46-001", "attempt-1", "stats.json")
	if path != wantPath {
		t.Fatalf("path = %q, want %q", path, wantPath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stats file: %v", err)
	}
	var got RunStats
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal stats file: %v", err)
	}
	if got.TaskID != record.TaskID || got.Attempt != record.Attempt || got.InputTokens != record.InputTokens {
		t.Fatalf("record = %+v, want %+v", got, record)
	}
}

func TestLoadSummaryAggregatesDougStatsAndFiltersByEpic(t *testing.T) {
	logsDir := filepath.Join(t.TempDir(), ".doug", "logs")
	records := []struct {
		epic string
		rec  RunStats
	}{
		{epic: "EPIC-45", rec: RunStats{Phase: "runtime", TaskID: "EPIC-45-001", Attempt: 1, InputTokens: 10, OutputTokens: 5, CacheTokens: 2, CostUSD: 0.01, DurationMs: 1000, FirstResponseMs: 100}},
		{epic: "EPIC-45", rec: RunStats{Phase: "runtime", TaskID: "EPIC-45-001", Attempt: 2, InputTokens: 7, OutputTokens: 3, CacheTokens: 1, CostUSD: 0.02, DurationMs: 2000, FirstResponseMs: 300}},
		{epic: "EPIC-46", rec: RunStats{Phase: "research", TaskID: "EPIC-46-001", Attempt: 1, InputTokens: 99, OutputTokens: 50, CostUSD: 0.50, DurationMs: 5000, FirstResponseMs: 500}},
	}
	for _, record := range records {
		if _, err := WriteRunStats(logsDir, record.epic, record.rec); err != nil {
			t.Fatalf("WriteRunStats: %v", err)
		}
	}

	summary, err := LoadSummary(logsDir, "EPIC-45")
	if err != nil {
		t.Fatalf("LoadSummary: %v", err)
	}
	if len(summary.Rows) != 1 {
		t.Fatalf("Rows len = %d, want 1", len(summary.Rows))
	}
	row := summary.Rows[0]
	if row.EpicID != "EPIC-45" || row.Phase != "runtime" || row.TaskID != "EPIC-45-001" || row.Runs != 2 {
		t.Fatalf("identity row = %+v", row)
	}
	if row.InputTokens != 17 || row.OutputTokens != 8 || row.CacheTokens != 3 || row.CostUSD != 0.03 || row.DurationMs != 3000 || row.FirstResponseMs != 200 {
		t.Fatalf("aggregated row = %+v", row)
	}
	if summary.Totals.InputTokens != 17 || summary.Totals.Runs != 2 || summary.Totals.FirstResponseMs != 200 {
		t.Fatalf("totals = %+v", summary.Totals)
	}
}
