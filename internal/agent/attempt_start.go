package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const attemptStartFilename = "attempt-start.json"

type attemptStartMarker struct {
	StartedAt string `json:"started_at"`
	Attempt   int    `json:"attempt"`
	TaskID    string `json:"task_id"`
}

// AttemptStartPath returns the retained Pi session archive marker path for a
// Doug task attempt. It intentionally shares the Pi session directory layout so
// an attempt-start.json file can prove that Doug began an attempt even if Pi or
// the agent died before ACTIVE_TASK.md contained a completed result.
func AttemptStartPath(projectRoot string, phase RunPhase, task TaskContext) string {
	return filepath.Join(piSessionDir(RunRequest{ProjectRoot: projectRoot, Phase: phase, Task: task}), attemptStartFilename)
}

// WriteAttemptStart writes attempt-start.json into the Pi session archive
// directory before the backend is invoked.
func WriteAttemptStart(projectRoot string, phase RunPhase, task TaskContext, startedAt time.Time) error {
	path := AttemptStartPath(projectRoot, phase, task)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create attempt-start directory: %w", err)
	}

	marker := attemptStartMarker{
		StartedAt: startedAt.UTC().Format(time.RFC3339),
		Attempt:   task.Attempt,
		TaskID:    task.ID,
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal attempt-start marker: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write attempt-start temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename attempt-start marker: %w", err)
	}
	return nil
}
