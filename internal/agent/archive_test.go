package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveActiveTask(t *testing.T) {
	t.Run("copies ACTIVE_TASK.md to correct archive path", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		logsDir := filepath.Join(dougDir, "logs")
		if err := os.MkdirAll(dougDir, 0o755); err != nil {
			t.Fatalf("mkdir dougDir: %v", err)
		}

		content := "# Active Task\n\n**Task ID**: EPIC-5-001\n\n---\n\n## Agent Result\n\n---\noutcome: \"SUCCESS\"\n---\n"
		if err := os.WriteFile(filepath.Join(dougDir, "ACTIVE_TASK.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write ACTIVE_TASK.md: %v", err)
		}

		if err := ArchiveActiveTask(dougDir, logsDir, "EPIC-5", "EPIC-5-001", 1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := filepath.Join(logsDir, "sessions", "EPIC-5", "session-EPIC-5-001_attempt-1.md")
		data, err := os.ReadFile(want)
		if err != nil {
			t.Fatalf("archive file not found at %s: %v", want, err)
		}
		if string(data) != content {
			t.Errorf("archive content mismatch:\ngot:  %q\nwant: %q", string(data), content)
		}
	})

	t.Run("creates parent directories if missing", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		logsDir := filepath.Join(dir, "nested", "logs")
		if err := os.MkdirAll(dougDir, 0o755); err != nil {
			t.Fatalf("mkdir dougDir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dougDir, "ACTIVE_TASK.md"), []byte("content"), 0o644); err != nil {
			t.Fatalf("write ACTIVE_TASK.md: %v", err)
		}

		if err := ArchiveActiveTask(dougDir, logsDir, "EPIC-1", "EPIC-1-001", 3); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := filepath.Join(logsDir, "sessions", "EPIC-1", "session-EPIC-1-001_attempt-3.md")
		if _, err := os.Stat(want); err != nil {
			t.Errorf("archive file not found at %s: %v", want, err)
		}
	})

	t.Run("attempt number is reflected in filename", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		logsDir := filepath.Join(dougDir, "logs")
		if err := os.MkdirAll(dougDir, 0o755); err != nil {
			t.Fatalf("mkdir dougDir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dougDir, "ACTIVE_TASK.md"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write ACTIVE_TASK.md: %v", err)
		}

		for _, attempt := range []int{1, 2, 5} {
			if err := ArchiveActiveTask(dougDir, logsDir, "EPIC-2", "EPIC-2-003", attempt); err != nil {
				t.Fatalf("attempt %d: unexpected error: %v", attempt, err)
			}
			wantFilename := fmt.Sprintf("session-EPIC-2-003_attempt-%d.md", attempt)
			path := filepath.Join(logsDir, "sessions", "EPIC-2", wantFilename)
			if _, err := os.Stat(path); err != nil {
				t.Errorf("attempt %d: file not found at %s: %v", attempt, path, err)
			}
		}
	})

	t.Run("returns error when ACTIVE_TASK.md is missing", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		logsDir := filepath.Join(dougDir, "logs")
		if err := os.MkdirAll(dougDir, 0o755); err != nil {
			t.Fatalf("mkdir dougDir: %v", err)
		}
		// No ACTIVE_TASK.md written.

		err := ArchiveActiveTask(dougDir, logsDir, "EPIC-3", "EPIC-3-001", 1)
		if err == nil {
			t.Error("expected error when ACTIVE_TASK.md is missing, got nil")
		}
		if !strings.Contains(err.Error(), "ACTIVE_TASK.md") {
			t.Errorf("error should mention ACTIVE_TASK.md, got: %v", err)
		}
	})

	t.Run("archive content matches ACTIVE_TASK.md including result block", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		logsDir := filepath.Join(dougDir, "logs")
		if err := os.MkdirAll(dougDir, 0o755); err != nil {
			t.Fatalf("mkdir dougDir: %v", err)
		}

		briefing := "# Active Task\n\n**Task ID**: EPIC-6-001\n\n---\n\n## Agent Result\n\n---\noutcome: \"FAILURE\"\nchangelog_entry: \"\"\ndependencies_added: []\n---\n\n## Implementation Summary\n\nFailed to implement.\n"
		if err := os.WriteFile(filepath.Join(dougDir, "ACTIVE_TASK.md"), []byte(briefing), 0o644); err != nil {
			t.Fatalf("write ACTIVE_TASK.md: %v", err)
		}

		if err := ArchiveActiveTask(dougDir, logsDir, "EPIC-6", "EPIC-6-001", 2); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		dst := filepath.Join(logsDir, "sessions", "EPIC-6", "session-EPIC-6-001_attempt-2.md")
		data, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("read archive: %v", err)
		}

		for _, want := range []string{
			"EPIC-6-001",
			"outcome: \"FAILURE\"",
			"Failed to implement.",
		} {
			if !strings.Contains(string(data), want) {
				t.Errorf("archive missing %q:\n%s", want, string(data))
			}
		}
	})
}

func TestCleanupActiveTask(t *testing.T) {
	t.Run("removes ACTIVE_TASK.md when present", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		if err := os.MkdirAll(dougDir, 0o755); err != nil {
			t.Fatalf("mkdir dougDir: %v", err)
		}
		path := filepath.Join(dougDir, "ACTIVE_TASK.md")
		if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
			t.Fatalf("write ACTIVE_TASK.md: %v", err)
		}

		if err := CleanupActiveTask(dougDir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("ACTIVE_TASK.md still present after cleanup: %v", err)
		}
	})

	t.Run("missing ACTIVE_TASK.md is non-fatal", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		if err := os.MkdirAll(dougDir, 0o755); err != nil {
			t.Fatalf("mkdir dougDir: %v", err)
		}

		if err := CleanupActiveTask(dougDir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
