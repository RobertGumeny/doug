package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ArchiveActiveTask copies .doug/ACTIVE_TASK.md to the session archive path:
//
//	{logsDir}/sessions/{epic}/session-{taskID}_attempt-{N}.md
//
// This function is unconditional: it must be called before every state change
// (rollback, commit, or state update) regardless of outcome so that a complete
// snapshot — briefing header plus agent result block — is always preserved on
// disk. The returned error is non-fatal; callers should log it as a warning and
// continue.
func ArchiveActiveTask(dougDir, logsDir, epic, taskID string, attempt int) error {
	src := filepath.Join(dougDir, "ACTIVE_TASK.md")
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read ACTIVE_TASK.md: %w", err)
	}

	dir := filepath.Join(logsDir, "sessions", epic)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create session archive directory %s: %w", dir, err)
	}

	filename := fmt.Sprintf("session-%s_attempt-%d.md", taskID, attempt)
	dst := filepath.Join(dir, filename)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write session archive %s: %w", dst, err)
	}

	return nil
}

// CleanupActiveTask removes the live .doug/ACTIVE_TASK.md briefing once the
// handler has finished using it. Missing files are ignored so callers can treat
// cleanup as best-effort.
func CleanupActiveTask(dougDir string) error {
	path := filepath.Join(dougDir, "ACTIVE_TASK.md")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove ACTIVE_TASK.md: %w", err)
	}
	return nil
}
