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

// TestInspectConfigDrift_NoPolicyBlock verifies that a config without a policy:
// block produces no drift — the policy block is retired and its absence is correct.
func TestInspectConfigDrift_NoPolicyBlock(t *testing.T) {
	dougDir := t.TempDir()
	cfg := "build_system: go\nmax_retries: 3\n"
	if err := os.WriteFile(filepath.Join(dougDir, "doug.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	items, err := inspectConfigDrift(dougDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected no drift when policy block is absent, got %d items", len(items))
	}
}

// TestInspectConfigDrift_RetiredPolicyBlockFlagged verifies that an existing
// policy: block in doug.yaml is flagged as a retired field to be removed.
func TestInspectConfigDrift_RetiredPolicyBlockFlagged(t *testing.T) {
	dougDir := t.TempDir()
	cfg := `build_system: go
policy:
  phases:
    runtime:
      interaction_mode: rpc
`
	if err := os.WriteFile(filepath.Join(dougDir, "doug.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	items, err := inspectConfigDrift(dougDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 drift item for retired policy block, got %d", len(items))
	}
	if items[0].Kind != driftMissingConfig {
		t.Errorf("expected driftMissingConfig, got %v", items[0].Kind)
	}
	if !strings.Contains(items[0].Description, "policy:") || !strings.Contains(items[0].Description, "retired") {
		t.Errorf("expected retired policy: mention in description, got: %s", items[0].Description)
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
	skillPath := filepath.Join(dir, ".pi", "skills", "doug-implement-feature", "SKILL.md")
	if err := os.Remove(skillPath); err != nil {
		t.Fatalf("remove skill: %v", err)
	}
	items, err := inspectManagedSurfaces(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, it := range items {
		if it.Kind == driftMissingManaged && strings.Contains(it.DisplayPath, "doug-implement-feature") {
			found = true
			if it.Action != actionReinstall {
				t.Errorf("expected actionReinstall, got %v", it.Action)
			}
		}
	}
	if !found {
		t.Error("expected driftMissingManaged item for doug-implement-feature/SKILL.md")
	}
}

func TestInspectManagedSurfaces_OutdatedSkill(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	skillPath := filepath.Join(dir, ".pi", "skills", "doug-implement-feature", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("outdated content"), 0o644); err != nil {
		t.Fatalf("write modified skill: %v", err)
	}
	items, err := inspectManagedSurfaces(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, it := range items {
		if it.Kind == driftOutdatedManaged && strings.Contains(it.DisplayPath, "doug-implement-feature") {
			found = true
			if it.Action != actionReinstall {
				t.Errorf("expected actionReinstall, got %v", it.Action)
			}
		}
	}
	if !found {
		t.Error("expected driftOutdatedManaged item for doug-implement-feature/SKILL.md")
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
		{Kind: driftMissingManaged, DisplayPath: ".pi/skills/doug-scaffold/SKILL.md", Description: "absent", Action: actionReinstall},
		{Kind: driftOutdatedManaged, DisplayPath: ".pi/skills/doug-research/SKILL.md", Description: "differs", Action: actionReinstall},
	}
	var buf bytes.Buffer
	reportDrift(&buf, items)
	out := buf.String()

	checks := []string{
		".claude",
		".doug/doug.yaml",
		".pi/skills/doug-scaffold/SKILL.md",
		".pi/skills/doug-research/SKILL.md",
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

	skillPath := filepath.Join(dir, ".pi", "skills", "doug-implement-feature", "SKILL.md")
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
		DisplayPath: ".pi/skills/doug-implement-feature/SKILL.md",
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
		Description: "policy.phases block is absent — restore managed interaction_mode defaults",
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
		{Kind: driftMissingConfig, AbsPath: filepath.Join(dir, ".doug", "doug.yaml"), DisplayPath: ".doug/doug.yaml", Description: "policy.phases.runtime missing interaction_mode: rpc", Action: actionPatch},
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

// ---------------------------------------------------------------------------
// Representative stale-workspace scenario regression tests
//
// These tests simulate realistic consuming-repository states to catch
// regressions in the full inspect→apply pipeline. Each scenario reflects
// a real upgrade pattern: full pre-Pi state, partial migration, idempotency
// after apply, and preservation of the filesystem in dry-run (inspect-only) mode.
// ---------------------------------------------------------------------------

// TestUpgrade_FullPrePiWorkspace verifies that a workspace in a pre-Pi state
// (all three retired artifacts present, minimal config without policy block, no .pi/
// directory) generates drift items in the retired and managed categories.
func TestUpgrade_FullPrePiWorkspace(t *testing.T) {
	dir := t.TempDir()
	dougDir := filepath.Join(dir, ".doug")
	if err := os.MkdirAll(dougDir, 0o755); err != nil {
		t.Fatalf("mkdir .doug: %v", err)
	}

	// Pre-Pi retired artifacts.
	for _, name := range []string{".claude", ".codex", ".gemini"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}

	// Minimal doug.yaml without policy block.
	prePiConfig := "build_system: go\nmax_retries: 3\n"
	if err := os.WriteFile(filepath.Join(dougDir, "doug.yaml"), []byte(prePiConfig), 0o644); err != nil {
		t.Fatalf("write doug.yaml: %v", err)
	}

	// No .pi/ directory — all managed surfaces absent.

	items, err := inspectWorkspace(dir, dougDir)
	if err != nil {
		t.Fatalf("inspectWorkspace: %v", err)
	}

	retired := filterDriftItems(items, driftRetiredArtifact)
	cfgDrift := filterDriftItems(items, driftMissingConfig)
	missingManaged := filterDriftItems(items, driftMissingManaged)

	if len(retired) != 3 {
		t.Errorf("expected 3 retired artifact items, got %d", len(retired))
	}
	// No policy block — no config drift expected.
	if len(cfgDrift) != 0 {
		t.Errorf("expected no config drift when policy block is absent, got %d items", len(cfgDrift))
	}
	if len(missingManaged) == 0 {
		t.Error("expected missing managed surface items for absent .pi/ directory, got none")
	}
}

// TestUpgrade_PartialDriftWorkspace verifies detection in a partially migrated
// workspace: one retired artifact still present, config has only two of five
// required phases at rpc, and one Pi skill is outdated.
func TestUpgrade_PartialDriftWorkspace(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	dougDir := filepath.Join(dir, ".doug")

	// One retired artifact still present.
	if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0o755); err != nil {
		t.Fatalf("mkdir .codex: %v", err)
	}

	// Config with a retired policy block — should generate 1 config drift item.
	partialConfig := `build_system: go
policy:
  phases:
    runtime:
      interaction_mode: rpc
    planning:
      interaction_mode: interactive
`
	if err := os.WriteFile(filepath.Join(dougDir, "doug.yaml"), []byte(partialConfig), 0o644); err != nil {
		t.Fatalf("write doug.yaml: %v", err)
	}

	// One outdated skill.
	skillPath := filepath.Join(dir, ".pi", "skills", "doug-research", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("outdated content"), 0o644); err != nil {
		t.Fatalf("write outdated skill: %v", err)
	}

	items, err := inspectWorkspace(dir, dougDir)
	if err != nil {
		t.Fatalf("inspectWorkspace: %v", err)
	}

	retired := filterDriftItems(items, driftRetiredArtifact)
	cfgDrift := filterDriftItems(items, driftMissingConfig)
	outdated := filterDriftItems(items, driftOutdatedManaged)

	if len(retired) != 1 {
		t.Errorf("expected 1 retired artifact, got %d", len(retired))
	}
	// Policy block is retired — 1 config drift item.
	if len(cfgDrift) != 1 {
		t.Errorf("expected 1 config drift item for retired policy block, got %d", len(cfgDrift))
	}
	if len(outdated) == 0 {
		t.Error("expected at least one outdated managed surface for research/SKILL.md, got none")
	}
}

// TestUpgrade_IdempotentAfterApply verifies that after applyUpgrade (with --force),
// a subsequent inspection finds no retired artifacts or managed surface drift.
// Config drift is excluded since it requires manual operator action.
func TestUpgrade_IdempotentAfterApply(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	dougDir := filepath.Join(dir, ".doug")

	// Retired artifact.
	retiredDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(retiredDir, 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}

	// Corrupted managed surface.
	skillPath := filepath.Join(dir, ".pi", "skills", "doug-scaffold", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("stale content"), 0o644); err != nil {
		t.Fatalf("corrupt skill: %v", err)
	}

	// First pass: inspect and apply with --force.
	items, err := inspectWorkspace(dir, dougDir)
	if err != nil {
		t.Fatalf("first inspectWorkspace: %v", err)
	}
	var buf bytes.Buffer
	if err := applyUpgrade(&buf, dir, items, true); err != nil {
		t.Fatalf("applyUpgrade: %v", err)
	}

	// Second pass: re-inspect; no retired artifacts or surface drift should remain.
	items2, err := inspectWorkspace(dir, dougDir)
	if err != nil {
		t.Fatalf("second inspectWorkspace: %v", err)
	}
	for _, it := range items2 {
		if it.Kind == driftRetiredArtifact {
			t.Errorf("retired artifact still present after apply: %s", it.DisplayPath)
		}
		if it.Kind == driftMissingManaged || it.Kind == driftOutdatedManaged {
			t.Errorf("managed surface drift remains after apply: %s — %s", it.DisplayPath, it.Description)
		}
	}
}

// TestUpgrade_DryRunPreservesFilesystem verifies that the inspect+report path
// (equivalent to --dry-run) leaves retired artifacts and managed surfaces on
// disk exactly as found.
func TestUpgrade_DryRunPreservesFilesystem(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	dougDir := filepath.Join(dir, ".doug")

	// Retired artifact.
	retiredDir := filepath.Join(dir, ".codex")
	if err := os.MkdirAll(retiredDir, 0o755); err != nil {
		t.Fatalf("mkdir .codex: %v", err)
	}

	// Stale skill.
	skillPath := filepath.Join(dir, ".pi", "skills", "doug-implement-bugfix", "SKILL.md")
	staleContent := []byte("stale content")
	if err := os.WriteFile(skillPath, staleContent, 0o644); err != nil {
		t.Fatalf("write stale skill: %v", err)
	}

	// Dry-run: inspect + report only, no apply.
	items, err := inspectWorkspace(dir, dougDir)
	if err != nil {
		t.Fatalf("inspectWorkspace: %v", err)
	}
	var buf bytes.Buffer
	reportDrift(&buf, items)

	// Retired artifact must still exist (dry-run made no changes).
	if _, statErr := os.Stat(retiredDir); statErr != nil {
		t.Error("expected .codex to remain on disk after dry-run (inspect+report only)")
	}

	// Stale skill must be unchanged.
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read skill after dry-run: %v", err)
	}
	if !bytes.Equal(content, staleContent) {
		t.Error("expected skill content unchanged after dry-run (inspect+report only)")
	}
}

// ---------------------------------------------------------------------------
// inspectConfigDrift — extended field detection
// ---------------------------------------------------------------------------

// TestInspectConfigDrift_DetectsStandaloneExecutionFields verifies that
// interaction_mode, execution_mode, and *_agent_command fields at the top level
// of doug.yaml (outside any policy block) are flagged as retired.
func TestInspectConfigDrift_DetectsStandaloneExecutionFields(t *testing.T) {
	dougDir := t.TempDir()
	cfg := `build_system: go
interaction_mode: rpc
execution_mode: pi
code_agent_command: pi run code
max_retries: 3
`
	if err := os.WriteFile(filepath.Join(dougDir, "doug.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	items, err := inspectConfigDrift(dougDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 drift item for retired standalone fields, got %d", len(items))
	}
	item := items[0]
	if item.Kind != driftMissingConfig {
		t.Errorf("expected driftMissingConfig, got %v", item.Kind)
	}
	if item.Action != actionStripConfig {
		t.Errorf("expected actionStripConfig, got %v", item.Action)
	}
	for _, want := range []string{"interaction_mode:", "execution_mode:", "code_agent_command:", "retired"} {
		if !strings.Contains(item.Description, want) {
			t.Errorf("expected %q in description, got: %s", want, item.Description)
		}
	}
}

// ---------------------------------------------------------------------------
// applyUpgrade — actionStripConfig: stale policy removal and field preservation
// ---------------------------------------------------------------------------

// TestApplyUpgrade_StripRetiredPolicyConfig verifies that applyUpgrade with
// actionStripConfig removes the policy: block from doug.yaml while preserving
// core project settings (build_system, max_retries, kb_enabled).
func TestApplyUpgrade_StripRetiredPolicyConfig(t *testing.T) {
	dougDir := t.TempDir()
	configPath := filepath.Join(dougDir, "doug.yaml")
	cfg := `build_system: go
max_retries: 3
kb_enabled: true
policy:
  phases:
    runtime:
      interaction_mode: rpc
    planning:
      interaction_mode: interactive
`
	if err := os.WriteFile(configPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	items := []driftItem{{
		Kind:        driftMissingConfig,
		AbsPath:     configPath,
		DisplayPath: ".doug/doug.yaml",
		Description: "retired execution config fields (policy:)",
		Action:      actionStripConfig,
	}}

	var buf bytes.Buffer
	if err := applyUpgrade(&buf, dougDir, items, false); err != nil {
		t.Fatalf("applyUpgrade: %v", err)
	}

	result, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read result config: %v", err)
	}
	resultStr := string(result)

	// Retired field must be gone.
	if strings.Contains(resultStr, "policy:") {
		t.Errorf("expected policy: to be removed, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "interaction_mode:") {
		t.Errorf("expected interaction_mode: to be removed (nested in policy), got: %s", resultStr)
	}

	// Core settings must be preserved.
	for _, want := range []string{"build_system:", "max_retries:", "kb_enabled:"} {
		if !strings.Contains(resultStr, want) {
			t.Errorf("expected %q to be preserved in stripped config, got: %s", want, resultStr)
		}
	}
}

// TestApplyUpgrade_StripStandaloneExecutionFields verifies that standalone
// top-level retirement fields (interaction_mode, execution_mode, *_agent_command)
// are removed from doug.yaml while preserving non-execution settings.
func TestApplyUpgrade_StripStandaloneExecutionFields(t *testing.T) {
	dougDir := t.TempDir()
	configPath := filepath.Join(dougDir, "doug.yaml")
	cfg := `build_system: go
max_retries: 5
interaction_mode: rpc
execution_mode: pi
code_agent_command: pi run code
plan_agent_command: pi run plan
kb_enabled: true
`
	if err := os.WriteFile(configPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	items := []driftItem{{
		Kind:        driftMissingConfig,
		AbsPath:     configPath,
		DisplayPath: ".doug/doug.yaml",
		Description: "retired execution config fields",
		Action:      actionStripConfig,
	}}

	var buf bytes.Buffer
	if err := applyUpgrade(&buf, dougDir, items, false); err != nil {
		t.Fatalf("applyUpgrade: %v", err)
	}

	result, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read result config: %v", err)
	}
	resultStr := string(result)

	// All retired execution fields must be gone.
	for _, retired := range []string{"interaction_mode:", "execution_mode:", "code_agent_command:", "plan_agent_command:"} {
		if strings.Contains(resultStr, retired) {
			t.Errorf("expected %q to be removed, got: %s", retired, resultStr)
		}
	}

	// Non-execution settings must be preserved.
	for _, want := range []string{"build_system:", "max_retries:", "kb_enabled:"} {
		if !strings.Contains(resultStr, want) {
			t.Errorf("expected %q to be preserved, got: %s", want, resultStr)
		}
	}
}

// TestApplyUpgrade_StripConfig_PreservesNonExecutionSettings verifies that a
// config file containing only core project settings (no execution fields) is
// left semantically intact after a strip operation — a safety-net idempotency check.
func TestApplyUpgrade_StripConfig_PreservesNonExecutionSettings(t *testing.T) {
	dougDir := t.TempDir()
	configPath := filepath.Join(dougDir, "doug.yaml")
	cfg := `build_system: npm
max_retries: 4
max_iterations: 15
kb_enabled: false
agent_heartbeat_seconds: 60
lint_enabled: true
lint_command: npm run lint
`
	if err := os.WriteFile(configPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	items := []driftItem{{
		Kind:        driftMissingConfig,
		AbsPath:     configPath,
		DisplayPath: ".doug/doug.yaml",
		Description: "test strip with no retired fields present",
		Action:      actionStripConfig,
	}}

	var buf bytes.Buffer
	if err := applyUpgrade(&buf, dougDir, items, false); err != nil {
		t.Fatalf("applyUpgrade: %v", err)
	}

	result, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read result config: %v", err)
	}
	resultStr := string(result)

	for _, want := range []string{
		"build_system:",
		"max_retries:",
		"max_iterations:",
		"kb_enabled:",
		"agent_heartbeat_seconds:",
		"lint_enabled:",
		"lint_command:",
	} {
		if !strings.Contains(resultStr, want) {
			t.Errorf("expected %q to be preserved, got: %s", want, resultStr)
		}
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
	skillPath := filepath.Join(dir, ".pi", "skills", "doug-scaffold", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("corrupt skill: %v", err)
	}
	items := []driftItem{{
		Kind:        driftOutdatedManaged,
		AbsPath:     skillPath,
		DisplayPath: ".pi/skills/doug-scaffold/SKILL.md",
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
