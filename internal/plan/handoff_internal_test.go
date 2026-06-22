package plan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

// TestGuardEpicOverwrite_BackstopBlocksActiveOrCompleted verifies the guard
// retained after normalization still refuses to clobber an ACTIVE or COMPLETED
// package if an allocated slot is ever occupied.
func TestGuardEpicOverwrite_BackstopBlocksActiveOrCompleted(t *testing.T) {
	for _, status := range []types.EpicLifecycleStatus{types.EpicStatusActive, types.EpicStatusCompleted} {
		t.Run(string(status), func(t *testing.T) {
			dir := t.TempDir()
			metadataPath := filepath.Join(dir, "metadata.yaml")
			testutil.WriteFile(t, metadataPath, ""+
				"epic_id: \"EPIC-7\"\n"+
				"status: \""+string(status)+"\"\n"+
				"created_at: \"2026-04-01T18:00:00Z\"\n"+
				"source_plan_path: \".doug/plan/PLAN.md\"\n")

			err := guardEpicOverwrite(metadataPath)
			if err == nil {
				t.Fatal("expected guard error, got nil")
			}
			if !strings.Contains(err.Error(), `refusing to overwrite epic package "EPIC-7"`) {
				t.Fatalf("expected overwrite guard error, got: %v", err)
			}
		})
	}
}

func TestGuardEpicOverwrite_AllowsMissingOrPlanned(t *testing.T) {
	dir := t.TempDir()

	// Missing metadata: allowed.
	if err := guardEpicOverwrite(filepath.Join(dir, "missing.yaml")); err != nil {
		t.Fatalf("missing metadata should be allowed, got: %v", err)
	}

	// PLANNED metadata: allowed.
	plannedPath := filepath.Join(dir, "planned.yaml")
	testutil.WriteFile(t, plannedPath, ""+
		"epic_id: \"EPIC-7\"\n"+
		"status: \"PLANNED\"\n"+
		"created_at: \"2026-04-01T18:00:00Z\"\n"+
		"source_plan_path: \".doug/plan/PLAN.md\"\n")
	if err := guardEpicOverwrite(plannedPath); err != nil {
		t.Fatalf("PLANNED metadata should be allowed, got: %v", err)
	}
}

func TestMaxExistingEpicNumber(t *testing.T) {
	dir := t.TempDir()

	if n, err := maxExistingEpicNumber(dir); err != nil || n != 0 {
		t.Fatalf("no epics dir: got (%d, %v), want (0, nil)", n, err)
	}

	for _, id := range []string{"EPIC-2", "EPIC-48", "EPIC-9", "not-an-epic"} {
		testutil.WriteFile(t, filepath.Join(NewEpicPackagePaths(dir, id).MetadataPath), "x")
	}
	n, err := maxExistingEpicNumber(dir)
	if err != nil {
		t.Fatalf("maxExistingEpicNumber: %v", err)
	}
	if n != 48 {
		t.Fatalf("max: got %d, want 48", n)
	}
}

func TestRewriteEpicReferences_BoundaryAndSwapSafety(t *testing.T) {
	mappings := []epicIDMapping{
		{oldID: "EPIC-1", newID: "EPIC-49"},
		{oldID: "EPIC-12", newID: "EPIC-50"},
	}
	got := rewriteEpicReferences("EPIC-1, EPIC-12, EPIC-1-003, EPIC-123", mappings)
	want := "EPIC-49, EPIC-50, EPIC-49-003, EPIC-123"
	if got != want {
		t.Fatalf("boundary rewrite: got %q, want %q", got, want)
	}

	// Swap mapping must not double-apply.
	swap := []epicIDMapping{
		{oldID: "EPIC-1", newID: "EPIC-2"},
		{oldID: "EPIC-2", newID: "EPIC-1"},
	}
	if got := rewriteEpicReferences("EPIC-1 EPIC-2", swap); got != "EPIC-2 EPIC-1" {
		t.Fatalf("swap rewrite: got %q, want %q", got, "EPIC-2 EPIC-1")
	}
}
