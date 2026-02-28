package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain manages subprocess mode for invoke_test.go.
// When TEST_SUBPROCESS_EXIT is set, the binary exits with the given code
// instead of running the test suite. This allows the test binary to act as a
// controllable agent command in RunAgent tests.
func TestMain(m *testing.M) {
	switch os.Getenv("TEST_SUBPROCESS_EXIT") {
	case "0":
		os.Exit(0)
	case "1":
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestSplitShellArgs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "simple words",
			input: "claude --print",
			want:  []string{"claude", "--print"},
		},
		{
			name:  "double-quoted argument with spaces",
			input: `claude -p "Refer to CLAUDE.md for instructions"`,
			want:  []string{"claude", "-p", "Refer to CLAUDE.md for instructions"},
		},
		{
			name:  "single-quoted argument with spaces",
			input: "claude -p 'Refer to CLAUDE.md for instructions'",
			want:  []string{"claude", "-p", "Refer to CLAUDE.md for instructions"},
		},
		{
			name:  "escaped space outside quotes",
			input: `claude\ code -p msg`,
			want:  []string{"claude code", "-p", "msg"},
		},
		{
			name:  "escaped double-quote inside double quotes",
			input: `claude -p "say \"hello\""`,
			want:  []string{"claude", "-p", `say "hello"`},
		},
		{
			name:  "adjacent quoted and unquoted tokens merge",
			input: `pre"mid"post`,
			want:  []string{"premidpost"},
		},
		{
			name:  "leading and trailing spaces ignored",
			input: "  claude  -p  msg  ",
			want:  []string{"claude", "-p", "msg"},
		},
		{
			name:    "unterminated double quote returns error",
			input:   `claude -p "oops`,
			wantErr: true,
		},
		{
			name:    "unterminated single quote returns error",
			input:   "claude -p 'oops",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitShellArgs(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result: %v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("arg[%d]: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestRunAgent(t *testing.T) {
	// testBin is the current test binary. We use it as a controllable agent
	// by setting TEST_SUBPROCESS_EXIT before invoking RunAgent.
	//
	// os.Executable is used instead of os.Args[0] for reliability.
	// filepath.ToSlash converts the path to forward slashes so that
	// splitShellArgs (used inside RunAgent) does not mistake Windows path
	// separators (\) for POSIX escape characters. Forward slashes are valid
	// path separators on Windows and require no escaping.
	rawBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	testBin := filepath.ToSlash(rawBin)

	t.Run("returns validation error for empty command", func(t *testing.T) {
		_, err := RunAgent("", t.TempDir())
		if err == nil {
			t.Fatal("expected error for empty command, got nil")
		}
	})

	t.Run("returns validation error for whitespace-only command", func(t *testing.T) {
		_, err := RunAgent("   \t  ", t.TempDir())
		if err == nil {
			t.Fatal("expected error for whitespace-only command, got nil")
		}
	})

	t.Run("successful execution returns positive duration and no error", func(t *testing.T) {
		t.Setenv("TEST_SUBPROCESS_EXIT", "0")
		cmd := fmt.Sprintf("%s -test.run=^$", testBin)

		duration, err := RunAgent(cmd, t.TempDir())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if duration <= 0 {
			t.Errorf("expected positive duration, got %v", duration)
		}
	})

	t.Run("non-zero exit code returns error containing exit code", func(t *testing.T) {
		t.Setenv("TEST_SUBPROCESS_EXIT", "1")
		cmd := fmt.Sprintf("%s -test.run=^$", testBin)

		_, err := RunAgent(cmd, t.TempDir())
		if err == nil {
			t.Fatal("expected error for non-zero exit code, got nil")
		}
		if !strings.Contains(err.Error(), "1") {
			t.Errorf("error should contain exit code 1, got: %v", err)
		}
	})

	t.Run("duration is measured for successful run", func(t *testing.T) {
		t.Setenv("TEST_SUBPROCESS_EXIT", "0")
		cmd := fmt.Sprintf("%s -test.run=^$", testBin)

		duration, err := RunAgent(cmd, t.TempDir())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if duration < 0 {
			t.Errorf("duration must be non-negative, got %v", duration)
		}
	})
}
