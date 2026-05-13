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

func TestBuildInstallPlan_ClaudeOnlySelected(t *testing.T) {
	dir := t.TempDir()
	agentSelected := map[string]bool{"claude": true}

	entries, err := buildInstallPlan(dir, agentSelected, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kinds := collectKinds(entries)
	dsts := collectDstPaths(entries)

	// Claude settings.json must be present and use JSON merge.
	if kinds[".claude/settings.json"] != entryKindMergeJSON {
		t.Errorf(".claude/settings.json should have entryKindMergeJSON; got %v", kinds[".claude/settings.json"])
	}

	// .gitignore must be present with gitignore merge strategy.
	if kinds[".gitignore"] != entryKindMergeGitignore {
		t.Errorf(".gitignore should have entryKindMergeGitignore; got %v", kinds[".gitignore"])
	}

	// AGENTS.md must be present with agents merge strategy.
	if kinds["AGENTS.md"] != entryKindMergeAgentsMD {
		t.Errorf("AGENTS.md should have entryKindMergeAgentsMD; got %v", kinds["AGENTS.md"])
	}

	// CLAUDE.md must be a plain copy.
	if kinds["CLAUDE.md"] != entryKindCopy {
		t.Errorf("CLAUDE.md should have entryKindCopy; got %v", kinds["CLAUDE.md"])
	}

	// No .codex or .gemini destinations.
	for dst := range dsts {
		rel, _ := filepath.Rel(dir, dst)
		if len(rel) >= 6 && rel[:6] == ".codex" {
			t.Errorf("unexpected .codex destination: %s", rel)
		}
		if len(rel) >= 6 && rel[:6] == ".gemin" {
			t.Errorf("unexpected .gemini destination: %s", rel)
		}
	}

	// Skills land under .claude/skills/
	claudeSkillDst := filepath.Join(dir, ".claude", "skills", "implement-feature", "SKILL.md")
	if !dsts[claudeSkillDst] {
		t.Errorf("expected .claude/skills/implement-feature/SKILL.md in plan")
	}
	claudeManualReviewDst := filepath.Join(dir, ".claude", "skills", "manual-review", "SKILL.md")
	if !dsts[claudeManualReviewDst] {
		t.Errorf("expected .claude/skills/manual-review/SKILL.md in plan")
	}
}

func TestBuildInstallPlan_CodexOnlySelected(t *testing.T) {
	dir := t.TempDir()
	agentSelected := map[string]bool{"codex": true}

	entries, err := buildInstallPlan(dir, agentSelected, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kinds := collectKinds(entries)
	dsts := collectDstPaths(entries)

	// Codex config.toml must use TOML merge strategy.
	if kinds[".codex/config.toml"] != entryKindMergeCodexTOML {
		t.Errorf(".codex/config.toml should have entryKindMergeCodexTOML; got %v", kinds[".codex/config.toml"])
	}

	// No .claude destinations.
	claudeSettings := filepath.Join(dir, ".claude", "settings.json")
	if dsts[claudeSettings] {
		t.Errorf(".claude/settings.json should not be in plan when claude is not selected")
	}

	// Skills land under .codex/skills/
	codexSkillDst := filepath.Join(dir, ".codex", "skills", "implement-feature", "SKILL.md")
	if !dsts[codexSkillDst] {
		t.Errorf("expected .codex/skills/implement-feature/SKILL.md in plan")
	}
	codexManualReviewDst := filepath.Join(dir, ".codex", "skills", "manual-review", "SKILL.md")
	if !dsts[codexManualReviewDst] {
		t.Errorf("expected .codex/skills/manual-review/SKILL.md in plan")
	}
}

func TestBuildInstallPlan_GeminiSelected(t *testing.T) {
	dir := t.TempDir()
	agentSelected := map[string]bool{"gemini": true}

	entries, err := buildInstallPlan(dir, agentSelected, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kinds := collectKinds(entries)

	// gemini settings.json must use JSON merge.
	if kinds[".gemini/settings.json"] != entryKindMergeJSON {
		t.Errorf(".gemini/settings.json should have entryKindMergeJSON; got %v", kinds[".gemini/settings.json"])
	}
	// gemini policies/doug-default.json must also use JSON merge.
	policyKey := filepath.Join(".gemini", "policies", "doug-default.json")
	if kinds[policyKey] != entryKindMergeJSON {
		t.Errorf(".gemini/policies/doug-default.json should have entryKindMergeJSON; got %v", kinds[policyKey])
	}
}

func TestBuildInstallPlan_MultipleAgentsAllGetSkills(t *testing.T) {
	dir := t.TempDir()
	agentSelected := map[string]bool{"claude": true, "codex": true}

	entries, err := buildInstallPlan(dir, agentSelected, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dsts := collectDstPaths(entries)

	for _, provider := range []string{".claude", ".codex"} {
		dst := filepath.Join(dir, provider, "skills", "implement-feature", "SKILL.md")
		if !dsts[dst] {
			t.Errorf("expected %s/skills/implement-feature/SKILL.md in plan", provider)
		}
		manualReviewDst := filepath.Join(dir, provider, "skills", "manual-review", "SKILL.md")
		if !dsts[manualReviewDst] {
			t.Errorf("expected %s/skills/manual-review/SKILL.md in plan", provider)
		}
	}

	// gemini should not appear
	geminiDst := filepath.Join(dir, ".gemini", "skills", "implement-feature", "SKILL.md")
	if dsts[geminiDst] {
		t.Errorf(".gemini/skills should not appear when gemini is not selected")
	}
}

func TestBuildInstallPlan_TemplateFilesGoToDougLogs(t *testing.T) {
	dir := t.TempDir()
	agentSelected := map[string]bool{"claude": true}

	entries, err := buildInstallPlan(dir, agentSelected, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dsts := collectDstPaths(entries)
	for _, name := range []string{
		"SESSION_RESULTS_TEMPLATE.md",
		"BUG_REPORT_TEMPLATE.md",
		"FAILURE_REPORT_TEMPLATE.md",
	} {
		dst := filepath.Join(dir, ".doug", "logs", name)
		if !dsts[dst] {
			t.Errorf("expected %s in plan under .doug/logs/", name)
		}
	}
}

func TestBuildInstallPlan_AgentsMDEntryCarriesProjectMetadata(t *testing.T) {
	dir := t.TempDir()
	agentSelected := map[string]bool{"claude": true}

	entries, err := buildInstallPlan(dir, agentSelected, "")
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

	agentSelected := map[string]bool{"claude": true}
	entries, err := buildInstallPlan(dir, agentSelected, "")
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

func TestBuildInstallPlan_PiSkillsAlwaysScaffolded(t *testing.T) {
	for _, agents := range []map[string]bool{
		{"claude": true},
		{"codex": true},
		{"gemini": true},
		{},
	} {
		dir := t.TempDir()
		entries, err := buildInstallPlan(dir, agents, "go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		dsts := collectDstPaths(entries)
		for _, skill := range []string{"implement-feature", "implement-bugfix", "implement-documentation", "scaffold", "plan", "research", "manual-review"} {
			dst := filepath.Join(dir, ".pi", "skills", skill, "SKILL.md")
			if !dsts[dst] {
				t.Errorf("expected .pi/skills/%s/SKILL.md in plan (agents=%v)", skill, agents)
			}
		}
	}
}

func TestBuildInstallPlan_PiExtensionsAlwaysScaffolded(t *testing.T) {
	for _, agents := range []map[string]bool{
		{"claude": true},
		{"codex": true},
		{"gemini": true},
		{},
	} {
		dir := t.TempDir()
		entries, err := buildInstallPlan(dir, agents, "go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		dsts := collectDstPaths(entries)
		handoffDst := filepath.Join(dir, ".pi", "extensions", "handoff.ts")
		if !dsts[handoffDst] {
			t.Errorf("expected .pi/extensions/handoff.ts in plan (agents=%v)", agents)
		}
	}
}

func TestBuildInstallPlan_NoAgentsSelected(t *testing.T) {
	dir := t.TempDir()
	agentSelected := map[string]bool{}

	entries, err := buildInstallPlan(dir, agentSelected, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dsts := collectDstPaths(entries)

	// No agent-specific files.
	for _, providerDir := range []string{".claude", ".codex", ".gemini"} {
		for dst := range dsts {
			rel, _ := filepath.Rel(dir, dst)
			if len(rel) >= len(providerDir) && rel[:len(providerDir)] == providerDir {
				t.Errorf("unexpected provider destination when no agents selected: %s", rel)
			}
		}
	}

	// Common files still present.
	if !dsts[filepath.Join(dir, ".gitignore")] {
		t.Errorf("expected .gitignore in plan even when no agents selected")
	}
	if !dsts[filepath.Join(dir, "AGENTS.md")] {
		t.Errorf("expected AGENTS.md in plan even when no agents selected")
	}
}
