package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// buildInstallPlan — routing tests
// ---------------------------------------------------------------------------

// collectKinds returns a map from DisplayRel to entryKind for all entries in
// the plan. Helper used by routing assertions.
func collectKinds(entries []installEntry) map[string]entryKind {
	m := make(map[string]entryKind, len(entries))
	for _, e := range entries {
		m[e.DisplayRel] = e.Kind
	}
	return m
}

// collectDstPaths returns the set of DstPath values in the plan.
func collectDstPaths(entries []installEntry) map[string]bool {
	m := make(map[string]bool, len(entries))
	for _, e := range entries {
		m[e.DstPath] = true
	}
	return m
}

func TestBuildInstallPlan_PiSkillsAlwaysScaffolded(t *testing.T) {
	dir := t.TempDir()
	entries, err := buildInstallPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dsts := collectDstPaths(entries)
	for _, skill := range []string{"implement-feature", "implement-bugfix", "implement-documentation", "scaffold", "plan", "research"} {
		dst := filepath.Join(dir, ".pi", "skills", skill, "SKILL.md")
		if !dsts[dst] {
			t.Errorf("expected .pi/skills/%s/SKILL.md in plan", skill)
		}
	}
}

func TestBuildInstallPlan_PiExtensionsAlwaysScaffolded(t *testing.T) {
	dir := t.TempDir()
	entries, err := buildInstallPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dsts := collectDstPaths(entries)
	handoffDst := filepath.Join(dir, ".pi", "extensions", "handoff.ts")
	if !dsts[handoffDst] {
		t.Errorf("expected .pi/extensions/handoff.ts in plan")
	}
}

func TestBuildInstallPlan_NoProviderSpecificFiles(t *testing.T) {
	dir := t.TempDir()
	entries, err := buildInstallPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dsts := collectDstPaths(entries)
	for dst := range dsts {
		rel, _ := filepath.Rel(dir, dst)
		for _, providerDir := range []string{".claude", ".codex", ".gemini"} {
			if len(rel) >= len(providerDir) && rel[:len(providerDir)] == providerDir {
				t.Errorf("unexpected provider-specific destination in plan: %s", rel)
			}
		}
	}
}

func TestBuildInstallPlan_BugTemplateGoesToIntakeBugs(t *testing.T) {
	dir := t.TempDir()

	entries, err := buildInstallPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dsts := collectDstPaths(entries)
	if !dsts[filepath.Join(dir, ".doug", "intake", "bugs", "BUG_REPORT_TEMPLATE.md")] {
		t.Error("expected BUG_REPORT_TEMPLATE.md in plan under .doug/intake/bugs/")
	}
	if dsts[filepath.Join(dir, ".doug", "logs", "SESSION_RESULTS_TEMPLATE.md")] {
		t.Error("SESSION_RESULTS_TEMPLATE.md should not be scaffolded; ACTIVE_TASK.md is the sole result handshake")
	}
	if dsts[filepath.Join(dir, ".doug", "logs", "FAILURE_REPORT_TEMPLATE.md")] {
		t.Error("FAILURE_REPORT_TEMPLATE.md should not be scaffolded")
	}
}

func TestBuildInstallPlan_CommonFilesPresent(t *testing.T) {
	dir := t.TempDir()
	entries, err := buildInstallPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	kinds := collectKinds(entries)
	dsts := collectDstPaths(entries)

	if kinds[".gitignore"] != entryKindMergeGitignore {
		t.Errorf(".gitignore should have entryKindMergeGitignore; got %v", kinds[".gitignore"])
	}
	if kinds["AGENTS.md"] != entryKindMergeAgentsMD {
		t.Errorf("AGENTS.md should have entryKindMergeAgentsMD; got %v", kinds["AGENTS.md"])
	}
	if kinds["CLAUDE.md"] != entryKindCopy {
		t.Errorf("CLAUDE.md should have entryKindCopy; got %v", kinds["CLAUDE.md"])
	}
	if !dsts[filepath.Join(dir, ".gitignore")] {
		t.Errorf("expected .gitignore in plan")
	}
	if !dsts[filepath.Join(dir, "AGENTS.md")] {
		t.Errorf("expected AGENTS.md in plan")
	}
}

func TestBuildInstallPlan_AgentsMDEntryCarriesProjectMetadata(t *testing.T) {
	dir := t.TempDir()

	entries, err := buildInstallPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, e := range entries {
		if e.Kind == entryKindMergeAgentsMD {
			if e.projectID == "" {
				t.Error("AGENTS.md entry missing projectID")
			}
			if e.projectName == "" {
				t.Error("AGENTS.md entry missing projectName")
			}
			return
		}
	}
	t.Error("no entryKindMergeAgentsMD entry found in plan")
}

func TestBuildInstallPlan_AgentsMDEntryPreservesExistingProjectID(t *testing.T) {
	dir := t.TempDir()

	// Write an AGENTS.md with an existing project ID before building the plan.
	existing := "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nDOUG_PROJECT_ID: fixed-id-abc123\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	entries, err := buildInstallPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, e := range entries {
		if e.Kind == entryKindMergeAgentsMD {
			if e.projectID != "fixed-id-abc123" {
				t.Errorf("expected preserved projectID 'fixed-id-abc123'; got %q", e.projectID)
			}
			return
		}
	}
	t.Error("no entryKindMergeAgentsMD entry found in plan")
}
