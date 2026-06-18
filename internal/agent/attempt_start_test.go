package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAttemptStart(t *testing.T) {
	projectRoot := t.TempDir()
	task := TaskContext{
		ID:      "EPIC-44-004",
		Attempt: 2,
		EpicID:  "EPIC-44",
	}
	startedAt := time.Date(2026, 6, 17, 12, 30, 0, 0, time.FixedZone("offset", -4*60*60))

	if err := WriteAttemptStart(projectRoot, RunPhaseRuntime, task, startedAt); err != nil {
		t.Fatalf("WriteAttemptStart: %v", err)
	}

	path := AttemptStartPath(projectRoot, RunPhaseRuntime, task)
	wantPath := filepath.Join(projectRoot, ".doug", "logs", "pi-sessions", "EPIC-44", "EPIC-44-004", "attempt-2", "attempt-start.json")
	if path != wantPath {
		t.Fatalf("AttemptStartPath = %q, want %q", path, wantPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read attempt-start.json: %v", err)
	}
	var got struct {
		StartedAt string `json:"started_at"`
		Attempt   int    `json:"attempt"`
		TaskID    string `json:"task_id"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal attempt-start.json: %v", err)
	}
	if got.StartedAt != "2026-06-17T16:30:00Z" {
		t.Fatalf("started_at = %q, want UTC RFC3339 timestamp", got.StartedAt)
	}
	if got.Attempt != 2 {
		t.Fatalf("attempt = %d, want 2", got.Attempt)
	}
	if got.TaskID != "EPIC-44-004" {
		t.Fatalf("task_id = %q, want EPIC-44-004", got.TaskID)
	}
}
