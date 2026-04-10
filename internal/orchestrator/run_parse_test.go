package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/robertgumeny/doug/internal/agent"
)

func TestParseAgentResult_MissingOutcomeErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ACTIVE_TASK.md")
	content := "## Agent Result\n\n---\noutcome: \"\"\nchangelog_entry: \"\"\ndependencies_added: []\n---\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write ACTIVE_TASK.md: %v", err)
	}

	_, err := parseAgentResult(path)
	if !errors.Is(err, agent.ErrMissingOutcome) {
		t.Fatalf("expected ErrMissingOutcome, got %v", err)
	}
}

func TestParseAgentResult_NoFrontmatterErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ACTIVE_TASK.md")
	content := "## Agent Result\n\nNot filled in yet.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write ACTIVE_TASK.md: %v", err)
	}

	_, err := parseAgentResult(path)
	if !errors.Is(err, agent.ErrNoFrontmatter) {
		t.Fatalf("expected ErrNoFrontmatter, got %v", err)
	}
}
