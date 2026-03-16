package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/types"
)

func TestParseAgentResult_DocumentationMissingOutcomeFallsBackToEpicComplete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ACTIVE_TASK.md")
	content := "## Agent Result\n\n---\noutcome: \"\"\nchangelog_entry: \"\"\ndependencies_added: []\n---\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write ACTIVE_TASK.md: %v", err)
	}

	result, err := parseAgentResult(types.TaskTypeDocumentation, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != types.OutcomeEpicComplete {
		t.Fatalf("outcome = %q, want %q", result.Outcome, types.OutcomeEpicComplete)
	}
}

func TestParseAgentResult_FeatureMissingOutcomeStillErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ACTIVE_TASK.md")
	content := "## Agent Result\n\n---\noutcome: \"\"\nchangelog_entry: \"\"\ndependencies_added: []\n---\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write ACTIVE_TASK.md: %v", err)
	}

	_, err := parseAgentResult(types.TaskTypeFeature, path)
	if !errors.Is(err, agent.ErrMissingOutcome) {
		t.Fatalf("expected ErrMissingOutcome, got %v", err)
	}
}

func TestParseAgentResult_DocumentationNoFrontmatterFallsBackToEpicComplete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ACTIVE_TASK.md")
	content := "## Agent Result\n\nNot filled in yet.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write ACTIVE_TASK.md: %v", err)
	}

	result, err := parseAgentResult(types.TaskTypeDocumentation, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != types.OutcomeEpicComplete {
		t.Fatalf("outcome = %q, want %q", result.Outcome, types.OutcomeEpicComplete)
	}
}
