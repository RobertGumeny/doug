package runlock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTryAcquireFailsFastWhileHeld(t *testing.T) {
	dougDir := filepath.Join(t.TempDir(), ".doug")
	lock, err := TryAcquire(dougDir, "first")
	if err != nil {
		t.Fatalf("TryAcquire first: %v", err)
	}
	defer func() { _ = lock.Close() }()

	_, err = TryAcquire(dougDir, "second")
	if !errors.Is(err, ErrHeld) {
		t.Fatalf("TryAcquire second err = %v, want ErrHeld", err)
	}
}

func TestReleasedOrStaleLockFileDoesNotPermanentlyBlock(t *testing.T) {
	dougDir := filepath.Join(t.TempDir(), ".doug")
	if err := os.MkdirAll(dougDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(Path(dougDir), []byte("owner: crashed\n"), 0o644); err != nil {
		t.Fatalf("write stale lock file: %v", err)
	}

	lock, err := TryAcquire(dougDir, "later")
	if err != nil {
		t.Fatalf("TryAcquire stale file: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("Close first lock: %v", err)
	}

	reacquired, err := TryAcquire(dougDir, "after-release")
	if err != nil {
		t.Fatalf("TryAcquire after release: %v", err)
	}
	defer func() { _ = reacquired.Close() }()
	data, err := os.ReadFile(Path(dougDir))
	if err != nil {
		t.Fatalf("ReadFile lock metadata: %v", err)
	}
	if !strings.Contains(string(data), "owner: after-release") {
		t.Fatalf("lock metadata was not refreshed after release:\n%s", data)
	}
}
