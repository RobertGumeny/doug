// Package agent provides the functions that bridge the orchestrator and the
// agent process: session file creation, ACTIVE_TASK.md writing, agent
// invocation, and session result parsing.
package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/robertgumeny/doug/internal/templates"
)

// CreateSessionFile creates the session file for the given task at:
//
//	{logsDir}/sessions/{epic}/session-{taskID}_attempt-{attempt}.md
//
// The parent directory is created if it does not exist. The embedded session
// results template is copied to the new file with empty fields for the agent to fill in.
// The returned string is the absolute (or caller-relative) path to the file.
func CreateSessionFile(logsDir, epic, taskID string, attempt int) (string, error) {
	dir := filepath.Join(logsDir, "sessions", epic)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create session directory %s: %w", dir, err)
	}

	filename := fmt.Sprintf("session-%s_attempt-%d.md", taskID, attempt)
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, []byte(templates.SessionResult), 0o644); err != nil {
		return "", fmt.Errorf("write session file %s: %w", path, err)
	}

	return path, nil
}
