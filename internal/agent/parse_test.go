package agent

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/robertgumeny/doug/internal/types"
)

func TestParseSessionResult(t *testing.T) {
	writeFile := func(t *testing.T, content string) string {
		t.Helper()
		f := filepath.Join(t.TempDir(), "session.md")
		if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		return f
	}

	tests := []struct {
		name           string
		content        string
		wantOutcome    types.Outcome
		wantErrIs      error
		wantInvalidOut bool // expect *ErrInvalidOutcome
		wantErr        bool
	}{
		{
			name: "valid SUCCESS outcome",
			content: "---\n" +
				"task_id: \"EPIC-4-004\"\n" +
				"outcome: \"SUCCESS\"\n" +
				"changelog_entry: \"Implemented ParseSessionResult\"\n" +
				"---\n\n## Implementation Summary\n",
			wantOutcome: types.OutcomeSuccess,
		},
		{
			name:        "valid BUG outcome",
			content:     "---\noutcome: \"BUG\"\n---\n",
			wantOutcome: types.OutcomeBug,
		},
		{
			name:        "valid FAILURE outcome",
			content:     "---\noutcome: \"FAILURE\"\n---\n",
			wantOutcome: types.OutcomeFailure,
		},
		{
			name:        "valid EPIC_COMPLETE outcome",
			content:     "---\noutcome: \"EPIC_COMPLETE\"\n---\n",
			wantOutcome: types.OutcomeEpicComplete,
		},
		{
			name:        "lowercase success outcome is normalized",
			content:     "---\noutcome: \"success\"\n---\n",
			wantOutcome: types.OutcomeSuccess,
		},
		{
			name:        "lowercase epic_complete outcome is normalized",
			content:     "---\noutcome: \"epic_complete\"\n---\n",
			wantOutcome: types.OutcomeEpicComplete,
		},
		{
			name: "extra fields are silently ignored",
			content: "---\n" +
				"outcome: \"SUCCESS\"\n" +
				"duration_seconds: 300\n" +
				"estimated_tokens: 50000\n" +
				"build_successful: true\n" +
				"tests_run: 12\n" +
				"tests_passed: 12\n" +
				"unknown_future_field: \"ignored\"\n" +
				"---\n",
			wantOutcome: types.OutcomeSuccess,
		},
		{
			name:        "CRLF line endings",
			content:     "---\r\noutcome: \"SUCCESS\"\r\n---\r\n",
			wantOutcome: types.OutcomeSuccess,
		},
		{
			name:      "missing --- delimiters entirely",
			content:   "outcome: SUCCESS\n## Notes\n",
			wantErrIs: ErrNoFrontmatter,
			wantErr:   true,
		},
		{
			name:      "only one --- delimiter (no closing ---)",
			content:   "---\noutcome: \"SUCCESS\"\n",
			wantErrIs: ErrNoFrontmatter,
			wantErr:   true,
		},
		{
			name:      "empty outcome field",
			content:   "---\ntask_id: \"EPIC-1-001\"\noutcome: \"\"\n---\n",
			wantErrIs: ErrMissingOutcome,
			wantErr:   true,
		},
		{
			name:           "unknown outcome value",
			content:        "---\noutcome: \"UNKNOWN_VALUE\"\n---\n",
			wantInvalidOut: true,
			wantErr:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, tc.content)

			result, err := ParseSessionResult(path)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
					t.Errorf("errors.Is(%v, %v) = false, got error: %v", err, tc.wantErrIs, err)
				}
				if tc.wantInvalidOut {
					var invErr *ErrInvalidOutcome
					if !errors.As(err, &invErr) {
						t.Errorf("expected *ErrInvalidOutcome, got %T: %v", err, err)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil, expected non-nil")
			}
			if result.Outcome != tc.wantOutcome {
				t.Errorf("outcome = %q, want %q", result.Outcome, tc.wantOutcome)
			}
		})
	}

	t.Run("file not found returns os.ErrNotExist", func(t *testing.T) {
		_, err := ParseSessionResult(filepath.Join(t.TempDir(), "nonexistent.md"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("expected os.ErrNotExist, got: %v", err)
		}
	})
}

func TestParseSessionResult_ActiveTaskFormat(t *testing.T) {
	writeFile := func(t *testing.T, content string) string {
		t.Helper()
		f := filepath.Join(t.TempDir(), "ACTIVE_TASK.md")
		if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		return f
	}

	activeTaskPrefix := "# Active Task\n\n" +
		"**Active Bug File**: .doug/ACTIVE_BUG.md\n" +
		"**Failure File**: .doug/ACTIVE_FAILURE.md\n" +
		"**PRD File**: .doug/PRD.md\n\n" +
		"**Task ID**: EPIC-11-001\n" +
		"**Task Type**: feature\n" +
		"**Attempt**: 1 of 3\n\n" +
		"---\n\n" +
		"## Build System\n\n" +
		"**System**: go\n\n" +
		"---\n\n"

	t.Run("parses result block from ACTIVE_TASK.md with preceding --- dividers", func(t *testing.T) {
		content := activeTaskPrefix +
			"## Agent Result\n\n" +
			"---\n" +
			"outcome: \"SUCCESS\"\n" +
			"changelog_entry: \"Added result block\"\n" +
			"dependencies_added: []\n" +
			"---\n\n" +
			"## Implementation Summary\n"
		path := writeFile(t, content)

		result, err := ParseSessionResult(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Outcome != types.OutcomeSuccess {
			t.Errorf("outcome = %q, want %q", result.Outcome, types.OutcomeSuccess)
		}
		if result.ChangelogEntry != "Added result block" {
			t.Errorf("changelog_entry = %q, want %q", result.ChangelogEntry, "Added result block")
		}
	})

	t.Run("parses BUG outcome from ACTIVE_TASK.md", func(t *testing.T) {
		content := activeTaskPrefix +
			"## Agent Result\n\n" +
			"---\n" +
			"outcome: \"BUG\"\n" +
			"changelog_entry: \"\"\n" +
			"dependencies_added: []\n" +
			"---\n"
		path := writeFile(t, content)

		result, err := ParseSessionResult(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Outcome != types.OutcomeBug {
			t.Errorf("outcome = %q, want %q", result.Outcome, types.OutcomeBug)
		}
	})

	t.Run("empty outcome in result block returns ErrMissingOutcome", func(t *testing.T) {
		content := activeTaskPrefix +
			"## Agent Result\n\n" +
			"---\n" +
			"outcome: \"\"\n" +
			"changelog_entry: \"\"\n" +
			"dependencies_added: []\n" +
			"---\n"
		path := writeFile(t, content)

		_, err := ParseSessionResult(path)
		if !errors.Is(err, ErrMissingOutcome) {
			t.Errorf("expected ErrMissingOutcome, got: %v", err)
		}
	})

	t.Run("invalid outcome in result block returns ErrInvalidOutcome with value", func(t *testing.T) {
		content := activeTaskPrefix +
			"## Agent Result\n\n" +
			"---\n" +
			"outcome: \"completed\"\n" +
			"changelog_entry: \"\"\n" +
			"dependencies_added: []\n" +
			"---\n"
		path := writeFile(t, content)

		_, err := ParseSessionResult(path)
		var invErr *ErrInvalidOutcome
		if !errors.As(err, &invErr) {
			t.Fatalf("expected ErrInvalidOutcome, got: %v", err)
		}
		if invErr.Value != "completed" {
			t.Errorf("invalid outcome value = %q, want %q", invErr.Value, "completed")
		}
	})

	t.Run("no result block returns ErrNoFrontmatter", func(t *testing.T) {
		content := activeTaskPrefix + "## Agent Result\n\nNot filled in yet.\n"
		path := writeFile(t, content)

		_, err := ParseSessionResult(path)
		if !errors.Is(err, ErrNoFrontmatter) {
			t.Errorf("expected ErrNoFrontmatter, got: %v", err)
		}
	})
}
