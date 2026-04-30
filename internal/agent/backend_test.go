package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Compile-time assertion: DefaultBackend must implement Backend.
var _ Backend = DefaultBackend{}

func TestDefaultBackend_Run(t *testing.T) {
	rawBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	testBin := filepath.ToSlash(rawBin)

	t.Run("delegates to RunAgent and returns positive duration", func(t *testing.T) {
		t.Setenv("TEST_SUBPROCESS_EXIT", "0")
		cmd := fmt.Sprintf("%s -test.run=^$", testBin)
		b := DefaultBackend{}
		resp, err := b.Run(context.Background(), RunRequest{
			Command:     cmd,
			ProjectRoot: t.TempDir(),
			Output:      io.Discard,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Duration <= 0 {
			t.Errorf("expected positive duration, got %v", resp.Duration)
		}
	})

	t.Run("non-zero exit code propagates as error", func(t *testing.T) {
		t.Setenv("TEST_SUBPROCESS_EXIT", "1")
		cmd := fmt.Sprintf("%s -test.run=^$", testBin)
		b := DefaultBackend{}
		_, err := b.Run(context.Background(), RunRequest{
			Command:     cmd,
			ProjectRoot: t.TempDir(),
			Output:      io.Discard,
		})
		if err == nil {
			t.Fatal("expected error for non-zero exit code, got nil")
		}
	})

	t.Run("empty command returns validation error", func(t *testing.T) {
		b := DefaultBackend{}
		_, err := b.Run(context.Background(), RunRequest{
			Command:     "",
			ProjectRoot: t.TempDir(),
			Output:      io.Discard,
		})
		if err == nil {
			t.Fatal("expected validation error for empty command, got nil")
		}
	})

	t.Run("context cancellation propagates", func(t *testing.T) {
		t.Setenv("TEST_SUBPROCESS_SLEEP_MS", "5000")
		cmd := fmt.Sprintf("%s -test.run=^$", testBin)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		b := DefaultBackend{}
		_, err := b.Run(ctx, RunRequest{
			Command:     cmd,
			ProjectRoot: t.TempDir(),
			Output:      io.Discard,
		})
		if err == nil {
			t.Fatal("expected error from cancelled context, got nil")
		}
	})
}
