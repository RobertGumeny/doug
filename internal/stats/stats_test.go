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

	got := FromRunResponse("EPIC-46-001", 2, completedAt, resp)

	if got.TaskID != "EPIC-46-001" || got.Attempt != 2 || got.SessionID != "pi-session-run" {
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
	record := RunStats{TaskID: "EPIC-46-001", Attempt: 1, InputTokens: 10, CompletedAt: "2026-06-17T10:11:12Z"}

	path, err := WriteRunStats(logsDir, "EPIC-46", record)
	if err != nil {
		t.Fatalf("WriteRunStats: %v", err)
	}
	wantPath := filepath.Join(logsDir, "stats", "EPIC-46", "stats-EPIC-46-001_attempt-1.json")
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
