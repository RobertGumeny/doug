package agent

import (
	"fmt"
	"os"
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

func TestRunAgent(t *testing.T) {
	// testBin is the current test binary. We use it as a controllable agent
	// by setting TEST_SUBPROCESS_EXIT before invoking RunAgent.
	testBin := os.Args[0]

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
