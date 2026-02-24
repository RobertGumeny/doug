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
