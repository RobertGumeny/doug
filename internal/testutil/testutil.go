package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// WriteFile is a test helper that creates a file (and its parent directories)
// with the given content. It calls t.Fatalf if any filesystem operation fails.
func WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdirall %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
