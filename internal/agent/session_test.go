package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateSessionFile(t *testing.T) {
	t.Run("creates file at correct path", func(t *testing.T) {
		dir := t.TempDir()
		path, err := CreateSessionFile(dir, "EPIC-4", "EPIC-4-001", 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(dir, "sessions", "EPIC-4", "session-EPIC-4-001_attempt-1.md")
		if path != want {
			t.Errorf("path = %q, want %q", path, want)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("file not found at %s: %v", path, err)
		}
	})

	t.Run("pre-fills task_id field", func(t *testing.T) {
		dir := t.TempDir()
		path, err := CreateSessionFile(dir, "EPIC-4", "EPIC-4-001", 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, `task_id: "EPIC-4-001"`) {
			t.Errorf("task_id not pre-filled; file content:\n%s", content)
		}
		if strings.Contains(content, `task_id: ""`) {
			t.Errorf("template placeholder still present; file content:\n%s", content)
		}
	})

	t.Run("preserves rest of template structure", func(t *testing.T) {
		dir := t.TempDir()
		path, err := CreateSessionFile(dir, "EPIC-4", "EPIC-4-002", 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		content := string(data)
		for _, want := range []string{
			`outcome: ""`,
			`timestamp: ""`,
			`changelog_entry: ""`,
			"files_modified: []",
			"tests_run: 0",
			"tests_passed: 0",
			"build_successful: false",
			"## Implementation Summary",
			"## Files Changed",
			"## Key Decisions",
			"## Test Coverage",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("template field %q not found in file content:\n%s", want, content)
			}
		}
	})

	t.Run("creates parent directory if it does not exist", func(t *testing.T) {
		dir := t.TempDir()
		// Use a deeply nested logsDir that doesn't exist yet.
		logsDir := filepath.Join(dir, "nested", "logs")
		path, err := CreateSessionFile(logsDir, "EPIC-1", "EPIC-1-001", 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(logsDir, "sessions", "EPIC-1", "session-EPIC-1-001_attempt-3.md")
		if path != want {
			t.Errorf("path = %q, want %q", path, want)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("file not found at %s: %v", path, err)
		}
	})

	t.Run("attempt number is reflected in filename", func(t *testing.T) {
		dir := t.TempDir()
		for _, attempt := range []int{1, 2, 5} {
			path, err := CreateSessionFile(dir, "EPIC-2", "EPIC-2-003", attempt)
			if err != nil {
				t.Fatalf("attempt %d: unexpected error: %v", attempt, err)
			}
			want := fmt.Sprintf("session-EPIC-2-003_attempt-%d.md", attempt)
			if !strings.HasSuffix(path, want) {
				t.Errorf("attempt %d: path %q does not end with %q", attempt, path, want)
			}
		}
	})
}
