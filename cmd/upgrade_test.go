package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// inspectRetiredArtifacts
// ---------------------------------------------------------------------------

func TestInspectRetiredArtifacts_None(t *testing.T) {
	dir := t.TempDir()
	items, err := inspectRetiredArtifacts(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected no drift items in clean dir, got %d", len(items))
	}
}

func TestInspectRetiredArtifacts_DetectsAll(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{".claude", ".codex", ".gemini"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}
	items, err := inspectRetiredArtifacts(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 drift items, got %d", len(items))
	}
	for _, it := range items {
		if it.Kind != driftRetiredArtifact {
			t.Errorf("expected driftRetiredArtifact, got %v", it.Kind)
		}
		if it.Action != actionRemove {
			t.Errorf("expected actionRemove, got %v", it.Action)
		}
	}
}

func TestInspectRetiredArtifacts_Partial(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	items, err := inspectRetiredArtifacts(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 drift item, got %d", len(items))
	}
	if items[0].DisplayPath != ".claude" {
		t.Errorf("expected .claude, got %s", items[0].DisplayPath)
	}
}

// ---------------------------------------------------------------------------
// inspectConfigDrift
// ---------------------------------------------------------------------------

func TestInspectConfigDrift_NoFile(t *testing.T) {
	dougDir := t.TempDir()
	items, err := inspectConfigDrift(dougDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected no items when config absent, got %d", len(items))
	}
}

func TestInspectConfigDrift_MissingPolicyBlock(t *testing.T) {
	dougDir := t.TempDir()
	cfg := "build_system: go\nmax_retries: 3\n"
	if err := os.WriteFile(filepath.Join(dougDir, "doug.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	items, err := inspectConfigDrift(dougDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected drift items for missing policy block, got none")
	}
	if items[0].Kind != driftMissingConfig {
		t.Errorf("expected driftMissingConfig, got %v", items[0].Kind)
	}
	if !strings.Contains(items[0].Description, "policy.phases") {
		t.Errorf("expected policy.phases in description, got: %s", items[0].Description)
	}
}

func TestInspectConfigDrift_AllPhasesRPC(t *testing.T) {
	dougDir := t.TempDir()
	cfg := `build_system: go
policy:
  phases:
    runtime:
      execution_mode: rpc
    planning:
      execution_mode: rpc
    scaffold:
      execution_mode: rpc
    research:
      execution_mode: rpc
    post_epic_kb:
      execution_mode: rpc
`
	if err := os.WriteFile(filepath.Join(dougDir, "doug.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	items, err := inspectConfigDrift(dougDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected no drift for fully configured policy, got %d items", len(items))
	}
}

func TestInspectConfigDrift_MissingPhases(t *testing.T) {
	dougDir := t.TempDir()
	// Only runtime is set; planning, scaffold, research, post_epic_kb missing.
	cfg := `build_system: go
policy:
  phases:
    runtime:
      execution_mode: rpc
`
	if err := os.WriteFile(filepath.Join(dougDir, "doug.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	items, err := inspectConfigDrift(dougDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect 4 missing phases: planning, scaffold, research, post_epic_kb.
	if len(items) != 4 {
		t.Errorf("expected 4 drift items, got %d", len(items))
	}
	for _, it := range items {
		if it.Kind != driftMissingConfig {
			t.Errorf("expected driftMissingConfig, got %v", it.Kind)
		}
	}
}

func TestInspectConfigDrift_WrongExecutionMode(t *testing.T) {
	dougDir := t.TempDir()
	cfg := `build_system: go
policy:
  phases:
    runtime:
      execution_mode: subprocess
    planning:
      execution_mode: rpc
    scaffold:
      execution_mode: rpc
    research:
      execution_mode: rpc
    post_epic_kb:
      execution_mode: rpc
`
	if err := os.WriteFile(filepath.Join(dougDir, "doug.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	items, err := inspectConfigDrift(dougDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 drift item for runtime subprocess, got %d", len(items))
	}
	if !strings.Contains(items[0].Description, "runtime") {
		t.Errorf("expected runtime in description, got: %s", items[0].Description)
	}
}

// ---------------------------------------------------------------------------
// inspectManagedSurfaces
// ---------------------------------------------------------------------------

func TestInspectManagedSurfaces_AllPresent(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	items, err := inspectManagedSurfaces(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected no drift after fresh init, got %d items", len(items))
		for _, it := range items {
			t.Logf("  drift: %s — %s", it.DisplayPath, it.Description)
		}
	}
}

func TestInspectManagedSurfaces_MissingSkill(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	skillPath := filepath.Join(dir, ".pi", "skills", "implement-feature", "SKILL.md")
	if err := os.Remove(skillPath); err != nil {
		t.Fatalf("remove skill: %v", err)
	}
	items, err := inspectManagedSurfaces(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, it := range items {
		if it.Kind == driftMissingManaged && strings.Contains(it.DisplayPath, "implement-feature") {
			found = true
			if it.Action != actionReinstall {
				t.Errorf("expected actionReinstall, got %v", it.Action)
			}
		}
	}
	if !found {
		t.Error("expected driftMissingManaged item for implement-feature/SKILL.md")
	}
}

func TestInspectManagedSurfaces_OutdatedSkill(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	skillPath := filepath.Join(dir, ".pi", "skills", "implement-feature", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("outdated content"), 0o644); err != nil {
		t.Fatalf("write modified skill: %v", err)
	}
	items, err := inspectManagedSurfaces(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, it := range items {
		if it.Kind == driftOutdatedManaged && strings.Contains(it.DisplayPath, "implement-feature") {
			found = true
			if it.Action != actionReinstall {
				t.Errorf("expected actionReinstall, got %v", it.Action)
			}
		}
	}
	if !found {
		t.Error("expected driftOutdatedManaged item for implement-feature/SKILL.md")
	}
}

func TestInspectManagedSurfaces_MissingHandoff(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	handoffPath := filepath.Join(dir, ".pi", "extensions", "handoff.ts")
	if err := os.Remove(handoffPath); err != nil {
		t.Fatalf("remove handoff.ts: %v", err)
	}
	items, err := inspectManagedSurfaces(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, it := range items {
		if it.Kind == driftMissingManaged && strings.Contains(it.DisplayPath, "handoff.ts") {
			found = true
		}
	}
	if !found {
		t.Error("expected driftMissingManaged item for handoff.ts")
	}
}

// ---------------------------------------------------------------------------
// reportDrift
// ---------------------------------------------------------------------------

func TestReportDrift_AllKinds(t *testing.T) {
	items := []driftItem{
		{Kind: driftRetiredArtifact, DisplayPath: ".claude", Description: "pre-Pi directory", Action: actionRemove},
		{Kind: driftMissingConfig, DisplayPath: ".doug/doug.yaml", Description: "policy.phases absent", Action: actionPatch},
		{Kind: driftMissingManaged, DisplayPath: ".pi/skills/scaffold/SKILL.md", Description: "absent", Action: actionReinstall},
		{Kind: driftOutdatedManaged, DisplayPath: ".pi/skills/research/SKILL.md", Description: "differs", Action: actionReinstall},
	}
	var buf bytes.Buffer
	reportDrift(&buf, items)
	out := buf.String()

	checks := []string{
		".claude",
		".doug/doug.yaml",
		".pi/skills/scaffold/SKILL.md",
		".pi/skills/research/SKILL.md",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in report output", want)
		}
	}
}

func TestReportDrift_Empty(t *testing.T) {
	var buf bytes.Buffer
	reportDrift(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty items, got: %q", buf.String())
	}
}

// ---------------------------------------------------------------------------
// filterDriftItems
// ---------------------------------------------------------------------------

func TestFilterDriftItems(t *testing.T) {
	items := []driftItem{
		{Kind: driftRetiredArtifact},
		{Kind: driftMissingConfig},
		{Kind: driftRetiredArtifact},
		{Kind: driftOutdatedManaged},
	}
	if got := filterDriftItems(items, driftRetiredArtifact); len(got) != 2 {
		t.Errorf("expected 2 retired, got %d", len(got))
	}
	if got := filterDriftItems(items, driftMissingConfig); len(got) != 1 {
		t.Errorf("expected 1 missingConfig, got %d", len(got))
	}
	if got := filterDriftItems(items, driftMissingManaged); len(got) != 0 {
		t.Errorf("expected 0 missingManaged, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// inspectWorkspace (integration)
// ---------------------------------------------------------------------------

func TestInspectWorkspace_FreshInit_NoRetiredArtifacts(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	dougDir := filepath.Join(dir, ".doug")

	// Fresh init should have no retired artifacts or managed surface drift.
	items, err := inspectWorkspace(dir, dougDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, it := range items {
		if it.Kind == driftRetiredArtifact {
			t.Errorf("unexpected retired artifact: %s", it.DisplayPath)
		}
		if it.Kind == driftMissingManaged || it.Kind == driftOutdatedManaged {
			t.Errorf("unexpected managed surface drift: %s — %s", it.DisplayPath, it.Description)
		}
	}
}

// ---------------------------------------------------------------------------
// applyUpgrade
// ---------------------------------------------------------------------------

func TestApplyUpgrade_RemoveRetiredArtifact_WithForce(t *testing.T) {
	dir := t.TempDir()
	retiredDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(retiredDir, 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}

	items := []driftItem{{
		Kind:        driftRetiredArtifact,
		AbsPath:     retiredDir,
		DisplayPath: ".claude",
		Description: "pre-Pi provider directory",
		Action:      actionRemove,
	}}

	var buf bytes.Buffer
	if err := applyUpgrade(&buf, dir, items, true); err != nil {
		t.Fatalf("applyUpgrade: %v", err)
	}

	if _, statErr := os.Stat(retiredDir); !os.IsNotExist(statErr) {
		t.Error("expected .claude to be removed after apply with --force")
	}
}

func TestApplyUpgrade_RemoveRetiredArtifact_WithoutForce(t *testing.T) {
	dir := t.TempDir()
	retiredDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(retiredDir, 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}

	items := []driftItem{{
		Kind:        driftRetiredArtifact,
		AbsPath:     retiredDir,
		DisplayPath: ".claude",
		Description: "pre-Pi provider directory",
		Action:      actionRemove,
	}}

	var buf bytes.Buffer
	if err := applyUpgrade(&buf, dir, items, false); err != nil {
		t.Fatalf("applyUpgrade: %v", err)
	}

	if _, statErr := os.Stat(retiredDir); statErr != nil {
		t.Error("expected .claude to remain when --force is not set")
	}
}

func TestApplyUpgrade_ReinstallManagedSurface(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}

	skillPath := filepath.Join(dir, ".pi", "skills", "implement-feature", "SKILL.md")
	originalData, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read original skill: %v", err)
	}

	// Corrupt the file.
	if err := os.WriteFile(skillPath, []byte("stale content"), 0o644); err != nil {
		t.Fatalf("write stale content: %v", err)
	}

	items := []driftItem{{
		Kind:        driftOutdatedManaged,
		AbsPath:     skillPath,
		DisplayPath: ".pi/skills/implement-feature/SKILL.md",
		Description: "managed surface differs from current embedded template",
		Action:      actionReinstall,
	}}

	var buf bytes.Buffer
	if err := applyUpgrade(&buf, dir, items, false); err != nil {
		t.Fatalf("applyUpgrade: %v", err)
	}

	restored, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read restored skill: %v", err)
	}
	if !bytes.Equal(restored, originalData) {
		t.Error("expected SKILL.md to be restored to embedded template content after reinstall")
	}
}

func TestApplyUpgrade_PatchGuidance(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".doug", "doug.yaml")

	items := []driftItem{{
		Kind:        driftMissingConfig,
		AbsPath:     configPath,
		DisplayPath: ".doug/doug.yaml",
		Description: "policy.phases block is absent — add execution_mode: rpc for all phases",
		Action:      actionPatch,
	}}

	var buf bytes.Buffer
	if err := applyUpgrade(&buf, dir, items, false); err != nil {
		t.Fatalf("applyUpgrade: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Manual action required") {
		t.Errorf("expected 'Manual action required' in output, got: %q", out)
	}
	if !strings.Contains(out, ".doug/doug.yaml") {
		t.Errorf("expected '.doug/doug.yaml' in output, got: %q", out)
	}
}

func TestApplyUpgrade_Mixed_AllCasesReconciled(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}

	// Set up a retired artifact.
	retiredDir := filepath.Join(dir, ".codex")
	if err := os.MkdirAll(retiredDir, 0o755); err != nil {
		t.Fatalf("mkdir .codex: %v", err)
	}

	// Corrupt a managed surface.
	handoffPath := filepath.Join(dir, ".pi", "extensions", "handoff.ts")
	originalHandoff, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("read handoff.ts: %v", err)
	}
	if err := os.WriteFile(handoffPath, []byte("// stale"), 0o644); err != nil {
		t.Fatalf("corrupt handoff.ts: %v", err)
	}

	items := []driftItem{
		{Kind: driftRetiredArtifact, AbsPath: retiredDir, DisplayPath: ".codex", Action: actionRemove},
		{Kind: driftMissingConfig, AbsPath: filepath.Join(dir, ".doug", "doug.yaml"), DisplayPath: ".doug/doug.yaml", Description: "policy.phases.runtime missing execution_mode: rpc", Action: actionPatch},
		{Kind: driftOutdatedManaged, AbsPath: handoffPath, DisplayPath: ".pi/extensions/handoff.ts", Action: actionReinstall},
	}

	var buf bytes.Buffer
	if err := applyUpgrade(&buf, dir, items, true); err != nil {
		t.Fatalf("applyUpgrade: %v", err)
	}

	// Retired artifact should be gone.
	if _, statErr := os.Stat(retiredDir); !os.IsNotExist(statErr) {
		t.Error("expected .codex to be removed")
	}

	// Managed surface should be restored.
	restored, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("read restored handoff.ts: %v", err)
	}
	if !bytes.Equal(restored, originalHandoff) {
		t.Error("expected handoff.ts to be restored to embedded template content")
	}

	// Config drift should produce guidance output.
	if !strings.Contains(buf.String(), "Manual action required") {
		t.Errorf("expected guidance output for config drift, got: %q", buf.String())
	}
}

func TestApplyUpgrade_UserAuthoredSurfacesUntouched(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}

	// Write custom content to user-authored surfaces.
	agentsPath := filepath.Join(dir, "AGENTS.md")
	gitignorePath := filepath.Join(dir, ".gitignore")
	prdPath := filepath.Join(dir, ".doug", "PRD.md")

	agentsBefore, _ := os.ReadFile(agentsPath)
	gitignoreBefore, _ := os.ReadFile(gitignorePath)
	prdBefore, _ := os.ReadFile(prdPath)

	// Apply only a reinstall action (the only category that touches files).
	skillPath := filepath.Join(dir, ".pi", "skills", "scaffold", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("corrupt skill: %v", err)
	}
	items := []driftItem{{
		Kind:        driftOutdatedManaged,
		AbsPath:     skillPath,
		DisplayPath: ".pi/skills/scaffold/SKILL.md",
		Action:      actionReinstall,
	}}

	var buf bytes.Buffer
	if err := applyUpgrade(&buf, dir, items, false); err != nil {
		t.Fatalf("applyUpgrade: %v", err)
	}

	// User-authored files must not change.
	agentsAfter, _ := os.ReadFile(agentsPath)
	gitignoreAfter, _ := os.ReadFile(gitignorePath)
	prdAfter, _ := os.ReadFile(prdPath)

	if !bytes.Equal(agentsBefore, agentsAfter) {
		t.Error("AGENTS.md was modified by upgrade — must not touch user-authored surfaces")
	}
	if !bytes.Equal(gitignoreBefore, gitignoreAfter) {
		t.Error(".gitignore was modified by upgrade — must not touch user-authored surfaces")
	}
	if !bytes.Equal(prdBefore, prdAfter) {
		t.Error(".doug/PRD.md was modified by upgrade — must not touch user-authored surfaces")
	}
}
