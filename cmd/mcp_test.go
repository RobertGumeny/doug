package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMCPRejectsInvalidConfigBeforeServing(t *testing.T) {
	dir := t.TempDir()
	dougDir := filepath.Join(dir, ".doug")
	if err := os.MkdirAll(dougDir, 0o755); err != nil {
		t.Fatalf("create .doug: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dougDir, "doug.yaml"), []byte("build_system: rust\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out bytes.Buffer
	err := runMCP(dir, strings.NewReader(""), &out)
	if err == nil {
		t.Fatal("expected invalid config error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid config") || !strings.Contains(err.Error(), "build_system") {
		t.Fatalf("expected actionable invalid config error, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected MCP server not to write frames after config rejection, got %q", out.String())
	}
}
