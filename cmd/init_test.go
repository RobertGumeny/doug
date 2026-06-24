package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/config"
)

func loadDougConfig(t *testing.T, dir string) *config.OrchestratorConfig {
	t.Helper()

	cfg, err := config.LoadConfig(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatalf("load doug.yaml: %v", err)
	}
	return cfg
}

func TestInitProject_GeneratesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// doug.yaml lives in .doug/
	if _, err := os.Stat(filepath.Join(dir, ".doug", "doug.yaml")); err != nil {
		t.Errorf("file .doug/doug.yaml not created: %v", err)
	}
	// tasks.yaml and PRD.md both live in .doug/
	if _, err := os.Stat(filepath.Join(dir, ".doug", "tasks.yaml")); err != nil {
		t.Errorf("file .doug/tasks.yaml not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".doug", "PRD.md")); err != nil {
		t.Errorf("file .doug/PRD.md not created: %v", err)
	}
}

func TestInitProject_CopiesTemplateFiles(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// CLAUDE.md should be created at the project root (contains @AGENTS.md).
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Errorf("CLAUDE.md not created at root: %v", err)
	}

	// AGENTS.md should be created at the project root.
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); err != nil {
		t.Errorf("AGENTS.md not created at root: %v", err)
	}
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	agentsContent := string(agentsData)
	if strings.Count(agentsContent, dougInstructionsMarker) != 1 {
		t.Errorf("AGENTS.md should contain exactly one managed doug section; got:\n%s", agentsData)
	}
	if extractManagedBlockField(agentsContent, "DOUG_PROJECT_ID") == "" {
		t.Errorf("AGENTS.md missing DOUG_PROJECT_ID metadata; got:\n%s", agentsData)
	}
	if extractManagedBlockField(agentsContent, "DOUG_PROJECT_NAME") == "" {
		t.Errorf("AGENTS.md missing DOUG_PROJECT_NAME metadata; got:\n%s", agentsData)
	}
	if strings.Contains(agentsContent, "Progressive Disclosure") || strings.Contains(agentsContent, "Working Rules") {
		t.Errorf("AGENTS.md should not contain operating rules — those belong in ACTIVE_TASK.md; got:\n%s", agentsData)
	}

	// .doug/README.md should be created as an orientation guide.
	if _, err := os.Stat(filepath.Join(dir, ".doug", "README.md")); err != nil {
		t.Errorf(".doug/README.md not created: %v", err)
	}

	// .gitignore should be created at the project root with .doug ignored.
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore not created at root: %v", err)
	}
	if !strings.Contains(string(data), ".doug/") {
		t.Errorf(".gitignore missing .doug/ entry; got:\n%s", data)
	}

	// Supported *_TEMPLATE.md files land in .doug/logs/.
	for _, name := range []string{
		"SESSION_RESULTS_TEMPLATE.md",
		"BUG_REPORT_TEMPLATE.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, ".doug", "logs", name)); err != nil {
			t.Errorf(".doug/logs/%s not created: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".doug", "logs", "FAILURE_REPORT_TEMPLATE.md")); err == nil {
		t.Error(".doug/logs/FAILURE_REPORT_TEMPLATE.md should not be created")
	}

	// Skill files land under .pi/skills/ (Pi is the supported interaction model).
	for _, name := range []string{
		filepath.Join("implement-feature", "SKILL.md"),
		filepath.Join("implement-bugfix", "SKILL.md"),
		filepath.Join("implement-documentation", "SKILL.md"),
		filepath.Join("plan", "SKILL.md"),
		filepath.Join("plan", "references", "discovery.md"),
		filepath.Join("plan", "references", "roadmapping.md"),
		filepath.Join("plan", "references", "definition.md"),
		filepath.Join("plan", "references", "feature.md"),
		filepath.Join("plan", "references", "refactor.md"),
		filepath.Join("plan", "references", "bugfix.md"),
		filepath.Join("plan", "references", "greenfield.md"),
		filepath.Join("research", "SKILL.md"),
	} {
		if _, err := os.Stat(filepath.Join(dir, ".pi", "skills", name)); err != nil {
			t.Errorf(".pi/skills/%s not created: %v", name, err)
		}
	}

	// .pi/extensions/handoff.ts is always scaffolded.
	if _, err := os.Stat(filepath.Join(dir, ".pi", "extensions", "handoff.ts")); err != nil {
		t.Errorf(".pi/extensions/handoff.ts not created: %v", err)
	}

	// No provider-specific directories should be created.
	for _, providerDir := range []string{".claude", ".codex", ".gemini"} {
		if _, err := os.Stat(filepath.Join(dir, providerDir)); err == nil {
			t.Errorf("%s/ directory should not be created by init", providerDir)
		}
	}

	// docs/kb/ directory should be created.
	if _, err := os.Stat(filepath.Join(dir, "docs", "kb")); err != nil {
		t.Errorf("docs/kb/ not created: %v", err)
	}
}

// TestInitProject_BugReportTemplate verifies that new workspaces receive a
// BUG_REPORT_TEMPLATE.md whose frontmatter and guidance match the loader/writer
// schema enforced by internal/agent.WriteBugArchive.
func TestInitProject_BugReportTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "logs", "BUG_REPORT_TEMPLATE.md"))
	if err != nil {
		t.Fatalf("read BUG_REPORT_TEMPLATE.md: %v", err)
	}
	content := string(data)

	// Loader-compatible severity vocabulary must be present.
	for _, want := range []string{"critical", "high", "medium", "low"} {
		if !strings.Contains(content, want) {
			t.Errorf("BUG_REPORT_TEMPLATE.md missing loader severity %q", want)
		}
	}

	// Loader-compatible status vocabulary must be present.
	for _, want := range []string{"open", "investigating", "fixed", "wont_fix"} {
		if !strings.Contains(content, want) {
			t.Errorf("BUG_REPORT_TEMPLATE.md missing loader status %q", want)
		}
	}

	// Stale status vocabulary must not appear.
	if strings.Contains(content, "in_progress") {
		t.Error("BUG_REPORT_TEMPLATE.md contains stale status \"in_progress\"; use \"investigating\"")
	}

	// The template must not instruct agents to write a separate ACTIVE_BUG.md.
	// Bug reporting flows through the ACTIVE_TASK.md result contract.
	if strings.Contains(content, "Use `.doug/ACTIVE_BUG.md`") {
		t.Error("BUG_REPORT_TEMPLATE.md must not instruct agents to write ACTIVE_BUG.md")
	}

	// Non-blocking bugs must be described as durable archive content.
	if !strings.Contains(content, "Non-blocking bugs are durable archive content") {
		t.Error("BUG_REPORT_TEMPLATE.md must describe non-blocking bugs as durable archive content")
	}

	// Session result routing (blocking/non-blocking) must be explained.
	if !strings.Contains(content, "ACTIVE_TASK.md") {
		t.Error("BUG_REPORT_TEMPLATE.md must reference ACTIVE_TASK.md result contract")
	}
	if !strings.Contains(content, "severity: blocking") {
		t.Error("BUG_REPORT_TEMPLATE.md must mention session severity: blocking")
	}
	if !strings.Contains(content, "severity: non-blocking") {
		t.Error("BUG_REPORT_TEMPLATE.md must mention session severity: non-blocking")
	}

	// Planning-discovered blockers guidance must be present.
	if !strings.Contains(content, "planned work") {
		t.Error("BUG_REPORT_TEMPLATE.md must say planning blockers should be represented as planned work")
	}
}

func TestInitProject_TemplateContent(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SESSION_RESULTS_TEMPLATE.md should have three frontmatter fields only.
	data, err := os.ReadFile(filepath.Join(dir, ".doug", "logs", "SESSION_RESULTS_TEMPLATE.md"))
	if err != nil {
		t.Fatalf("read SESSION_RESULTS_TEMPLATE.md: %v", err)
	}
	content := string(data)
	for _, want := range []string{`outcome: ""`, `changelog_entry: ""`, "dependencies_added: []"} {
		if !strings.Contains(content, want) {
			t.Errorf("SESSION_RESULTS_TEMPLATE.md missing field %q", want)
		}
	}
	if strings.Contains(content, "task_id:") {
		t.Errorf("SESSION_RESULTS_TEMPLATE.md must not contain task_id field")
	}
}

func TestInitProject_DetectsBuildSystem(t *testing.T) {
	t.Run("go.mod → build_system: go", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.21\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initProject(dir, false, "", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cfg := loadDougConfig(t, dir)
		if cfg.BuildSystem != "go" {
			t.Errorf("BuildSystem = %q, want %q", cfg.BuildSystem, "go")
		}
	})

	t.Run("package.json → build_system: npm", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initProject(dir, false, "", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cfg := loadDougConfig(t, dir)
		if cfg.BuildSystem != "npm" {
			t.Errorf("BuildSystem = %q, want %q", cfg.BuildSystem, "npm")
		}
	})

	t.Run("pnpm-workspace.yaml → build_system: pnpm", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte("packages:\n  - packages/*\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initProject(dir, false, "", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cfg := loadDougConfig(t, dir)
		if cfg.BuildSystem != "pnpm" {
			t.Errorf("BuildSystem = %q, want %q", cfg.BuildSystem, "pnpm")
		}
	})

	t.Run("no marker → default build_system: go", func(t *testing.T) {
		dir := t.TempDir()
		if err := initProject(dir, false, "", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cfg := loadDougConfig(t, dir)
		if cfg.BuildSystem != "go" {
			t.Errorf("BuildSystem = %q, want %q", cfg.BuildSystem, "go")
		}
	})
}

func TestInitProject_BuildSystemFlag(t *testing.T) {
	dir := t.TempDir()
	// go.mod exists (would auto-detect as go), but flag overrides to npm.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initProject(dir, false, "npm", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := loadDougConfig(t, dir)
	if cfg.BuildSystem != "npm" {
		t.Errorf("BuildSystem = %q, want %q", cfg.BuildSystem, "npm")
	}
}

func TestInitProject_GuardCheck(t *testing.T) {
	t.Run("exits with error if .doug/project-state.yaml exists", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		if err := os.MkdirAll(dougDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dougDir, "project-state.yaml"), []byte("existing content"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := initProject(dir, false, "", false)
		if err == nil {
			t.Fatal("expected error when .doug/project-state.yaml exists, got nil")
		}
		if !strings.Contains(err.Error(), "project-state.yaml") {
			t.Errorf("error message should mention project-state.yaml; got: %v", err)
		}
	})

	t.Run("stale root tasks.yaml does not trigger guard", func(t *testing.T) {
		dir := t.TempDir()
		// A stale tasks.yaml at root should NOT trigger the guard — guard only checks .doug/project-state.yaml.
		if err := os.WriteFile(filepath.Join(dir, "tasks.yaml"), []byte("existing tasks"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initProject(dir, false, "", false); err != nil {
			t.Fatalf("unexpected error when stale root tasks.yaml exists: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// dougYAMLContent — content tests
// ---------------------------------------------------------------------------

// TestDougYAMLContent_NoPolicyBlock verifies that dougYAMLContent does not
// emit a policy: block — execution routing is source-owned and not written to config.
func TestDougYAMLContent_NoPolicyBlock(t *testing.T) {
	content := dougYAMLContent("go", 3, 10, true)
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		t.Fatalf("dougYAMLContent produced invalid YAML: %v\ncontent:\n%s", err, content)
	}
	if _, ok := raw["policy"]; ok {
		t.Fatalf("dougYAMLContent must not emit policy block; execution routing is source-owned\ncontent:\n%s", content)
	}
}

// TestInitProject_NoPolicyBlock is an integration-level regression test
// verifying that init produces a doug.yaml without a policy block.
func TestInitProject_NoPolicyBlock(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", true); err != nil {
		t.Fatalf("initProject: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatalf("read doug.yaml: %v", err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse doug.yaml: %v\ncontent:\n%s", err, data)
	}
	if _, ok := raw["policy"]; ok {
		t.Fatalf("doug.yaml must not have policy block after init; content:\n%s", data)
	}
}

// TestDougYAMLContent_LintSettingsPresent verifies that lint_enabled is written
// into the generated doug.yaml as a core project/runtime setting.
func TestDougYAMLContent_LintSettingsPresent(t *testing.T) {
	content := dougYAMLContent("go", 3, 10, true)
	if !strings.Contains(content, "lint_enabled:") {
		t.Errorf("expected lint_enabled in generated doug.yaml; got:\n%s", content)
	}
	// Verify the generated yaml still has no policy block.
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		t.Fatalf("dougYAMLContent produced invalid YAML: %v\ncontent:\n%s", err, content)
	}
	if _, ok := raw["policy"]; ok {
		t.Fatalf("dougYAMLContent must not emit policy block; got:\n%s", content)
	}
}

// TestDougYAMLContent_ConfigValuesWritten verifies that maxRetries, maxIterations,
// kbEnabled, and reviewEnabled are written into the generated doug.yaml.
func TestDougYAMLContent_ConfigValuesWritten(t *testing.T) {
	content := dougYAMLContent("npm", 5, 20, false)
	if !strings.Contains(content, "build_system: npm") {
		t.Errorf("expected build_system: npm in output; got:\n%s", content)
	}
	if !strings.Contains(content, "max_retries: 5") {
		t.Errorf("expected max_retries: 5 in output; got:\n%s", content)
	}
	if !strings.Contains(content, "max_iterations: 20") {
		t.Errorf("expected max_iterations: 20 in output; got:\n%s", content)
	}
	if !strings.Contains(content, "kb_enabled: false") {
		t.Errorf("expected kb_enabled: false in output; got:\n%s", content)
	}
	if !strings.Contains(content, "review_enabled: true") {
		t.Errorf("expected review_enabled: true in output; got:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// AGENTS.md template content (EPIC-18 regression)
// ---------------------------------------------------------------------------

// TestInitProject_AgentsMDContainsManagedBlock verifies that the initialized
// AGENTS.md contains the Doug-managed block with project identity fields and
// operating rules (workflow contract and bug-capture guidance).
func TestInitProject_AgentsMDContainsManagedBlock(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	content := string(data)

	for _, forbidden := range []string{
		"Progressive Disclosure",
		"Working Rules",
		"ACTIVE_BUG.md",
		"ACTIVE_FAILURE.md",
	} {
		if strings.Contains(content, forbidden) {
			t.Errorf("AGENTS.md must not contain %q; got:\n%s", forbidden, content)
		}
	}

	for _, required := range []string{
		"DOUG_PROJECT_ID",
		"DOUG_PROJECT_NAME",
		".doug/ACTIVE_TASK.md",
		"canonical task brief",
		"BUG_REPORT_TEMPLATE.md",
		".doug/logs/bugs/",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("AGENTS.md missing required content %q; got:\n%s", required, content)
		}
	}
}

func TestInitProject_Force(t *testing.T) {
	t.Run("overwrites .doug/tasks.yaml when force=true", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		if err := os.MkdirAll(dougDir, 0o755); err != nil {
			t.Fatal(err)
		}
		original := "original content — should be replaced"
		if err := os.WriteFile(filepath.Join(dougDir, "tasks.yaml"), []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initProject(dir, true, "", false); err != nil {
			t.Fatalf("unexpected error with force=true: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dougDir, "tasks.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) == original {
			t.Error(".doug/tasks.yaml was not overwritten with --force")
		}
		if !strings.Contains(string(data), "EPIC-1") {
			t.Errorf(".doug/tasks.yaml does not contain expected content; got:\n%s", data)
		}
	})

	t.Run("proceeds without error when .doug/project-state.yaml exists and force=true", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		if err := os.MkdirAll(dougDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dougDir, "project-state.yaml"), []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initProject(dir, true, "", false); err != nil {
			t.Fatalf("unexpected error with force=true: %v", err)
		}
	})
}

func TestInitProject_InvalidBuildSystem(t *testing.T) {
	dir := t.TempDir()
	err := initProject(dir, false, "foobar", false)
	if err == nil {
		t.Fatal("expected error for invalid build system, got nil")
	}
	if !strings.Contains(err.Error(), "foobar") {
		t.Errorf("error should mention the invalid value; got: %v", err)
	}
}

func TestInitProject_BuildSystemFlagPnpm(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "pnpm", false); err != nil {
		t.Fatalf("unexpected error for --build-system pnpm: %v", err)
	}
	cfg := loadDougConfig(t, dir)
	if cfg.BuildSystem != "pnpm" {
		t.Errorf("BuildSystem = %q, want %q", cfg.BuildSystem, "pnpm")
	}
}

func TestInitProject_CreatesChangelog(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("CHANGELOG.md not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "## [Unreleased]") {
		t.Errorf("CHANGELOG.md missing [Unreleased] section; got:\n%s", content)
	}
}

func TestInitProject_DoesNotOverwriteChangelog(t *testing.T) {
	dir := t.TempDir()
	original := "# My existing changelog\n"
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	// Run with force=true — CHANGELOG.md must still not be overwritten.
	if err := initProject(dir, true, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Errorf("CHANGELOG.md was overwritten; want %q, got %q", original, string(data))
	}
}

func TestDougYAMLContent_ReflectsPromptedValues(t *testing.T) {
	dir := t.TempDir()
	testutilPath := filepath.Join(dir, ".doug")
	if err := os.MkdirAll(testutilPath, 0o755); err != nil {
		t.Fatalf("mkdir .doug: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testutilPath, "doug.yaml"), []byte(dougYAMLContent("npm", 7, 15, false)), 0o644); err != nil {
		t.Fatalf("write doug.yaml: %v", err)
	}

	cfg := loadDougConfig(t, dir)
	if cfg.BuildSystem != "npm" {
		t.Errorf("BuildSystem = %q, want %q", cfg.BuildSystem, "npm")
	}
	if cfg.MaxRetries != 7 {
		t.Errorf("MaxRetries = %d, want 7", cfg.MaxRetries)
	}
	if cfg.MaxIterations != 15 {
		t.Errorf("MaxIterations = %d, want 15", cfg.MaxIterations)
	}
	if cfg.KBEnabled {
		t.Error("KBEnabled = true, want false")
	}
}

func TestInitProject_MergesGitignore(t *testing.T) {
	dir := t.TempDir()
	existing := "node_modules/\n.env\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := initProject(dir, false, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	for _, want := range []string{"node_modules/", ".env", ".doug/"} {
		if !strings.Contains(content, want) {
			t.Fatalf("merged .gitignore missing %q; content:\n%s", want, content)
		}
	}
}

func TestMergeGitignore_Idempotent(t *testing.T) {
	existing := "# project\n.doug/\nnode_modules/\n"
	got := mergeGitignore(existing, "# doug\n.doug/\n")
	if strings.Count(got, ".doug/") != 1 {
		t.Fatalf("expected .doug/ to appear once; got:\n%s", got)
	}
	if !strings.Contains(got, "node_modules/") {
		t.Fatalf("expected existing entries to be preserved; got:\n%s", got)
	}
}

func TestMergeAgents(t *testing.T) {
	const testID = "proj-abc123"
	const testName = "My Proj"
	section := "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nDOUG_PROJECT_ID: " + testID + "\nDOUG_PROJECT_NAME: " + testName + "\n\n## Doug-Specific Instructions\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"

	t.Run("uses doug section when file is empty", func(t *testing.T) {
		got := mergeAgents("", section)
		if got != section {
			t.Fatalf("expected section only, got:\n%s", got)
		}
	})

	t.Run("appends section when marker absent", func(t *testing.T) {
		existing := "# Local Instructions\n\nKeep this.\n"
		got := mergeAgents(existing, section)
		if !strings.Contains(got, "# Local Instructions") {
			t.Fatalf("expected existing content to be preserved, got:\n%s", got)
		}
		if strings.Count(got, dougInstructionsMarker) != 1 {
			t.Fatalf("expected exactly one doug marker, got:\n%s", got)
		}
	})

	t.Run("does not duplicate section when marker already present", func(t *testing.T) {
		existing := "# Local Instructions\n\n" + section
		got := mergeAgents(existing, section)
		if strings.Count(got, dougInstructionsMarker) != 1 {
			t.Fatalf("expected one doug marker, got:\n%s", got)
		}
	})
}

// TestMergeAgents_BlockReplacedOnReconcile verifies that when the Doug-managed
// marker block is already present, mergeAgents replaces the entire block
// content with the new section (including updated operating rules). This is
// the intended init/upgrade contract: the block is Doug-owned and is refreshed
// on every reconcile pass.
func TestMergeAgents_BlockReplacedOnReconcile(t *testing.T) {
	// Existing block with old content that was manually edited.
	existing := "# Heading\n\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nDOUG_PROJECT_ID: proj-abc123\nDOUG_PROJECT_NAME: My Proj\n\nManual edit inside block.\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"
	newSection := "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nDOUG_PROJECT_ID: proj-abc123\nDOUG_PROJECT_NAME: My Proj\n\nNew operating rules.\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"
	got := mergeAgents(existing, newSection)

	if strings.Contains(got, "Manual edit inside block.") {
		t.Fatalf("expected manual edits inside the block to be overwritten; got:\n%s", got)
	}
	if !strings.Contains(got, "New operating rules.") {
		t.Fatalf("expected new section content to be present; got:\n%s", got)
	}
	if !strings.Contains(got, "DOUG_PROJECT_ID: proj-abc123") {
		t.Fatalf("expected project ID to be present in rebuilt block; got:\n%s", got)
	}
	if strings.Count(got, dougInstructionsMarker) != 1 {
		t.Fatalf("expected exactly one START marker; got:\n%s", got)
	}
}

// TestMergeAgents_ContentOutsideMarkersPreserved verifies that content before
// and after the Doug-managed block is kept verbatim during reconcile.
func TestMergeAgents_ContentOutsideMarkersPreserved(t *testing.T) {
	before := "# Custom Project Instructions\n\nKeep this content."
	after := "\n\n# Additional Instructions\n\nAlso keep this."
	existing := before + "\n\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nOLD BLOCK\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->" + after + "\n"
	newSection := "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nDOUG_PROJECT_ID: proj-abc123\nNEW BLOCK\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"
	got := mergeAgents(existing, newSection)

	if !strings.Contains(got, "Keep this content.") {
		t.Fatalf("content before markers should be preserved; got:\n%s", got)
	}
	if !strings.Contains(got, "Also keep this.") {
		t.Fatalf("content after markers should be preserved; got:\n%s", got)
	}
	if strings.Contains(got, "OLD BLOCK") {
		t.Fatalf("old block content should be replaced; got:\n%s", got)
	}
	if !strings.Contains(got, "NEW BLOCK") {
		t.Fatalf("new block content should be present; got:\n%s", got)
	}
}

// TestMergeAgents_ProjectIDPreservedViaSection verifies that the project ID
// embedded in dougSection (placed there by buildInstallPlan, which reads the
// existing AGENTS.md before constructing the section) survives the reconcile.
// Identity preservation is a buildInstallPlan responsibility; mergeAgents uses
// the section verbatim.
func TestMergeAgents_ProjectIDPreservedViaSection(t *testing.T) {
	// Simulate buildInstallPlan behaviour: the section already carries the
	// preserved project ID extracted from the existing file.
	existing := "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nDOUG_PROJECT_ID: original-id-abc123\nDOUG_PROJECT_NAME: Original Name\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"
	// dougSection already has the preserved ID (as buildInstallPlan would produce).
	newSection := "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nDOUG_PROJECT_ID: original-id-abc123\nDOUG_PROJECT_NAME: Original Name\n\nNew operating rules.\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"
	got := mergeAgents(existing, newSection)

	if !strings.Contains(got, "DOUG_PROJECT_ID: original-id-abc123") {
		t.Fatalf("expected original project ID to be present; got:\n%s", got)
	}
	if !strings.Contains(got, "New operating rules.") {
		t.Fatalf("expected new section content; got:\n%s", got)
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"my-project", "my-project"},
		{"My Project", "my-project"},
		{"my_project", "my-project"},
		{"my--project", "my-project"},
		{" project ", "project"},
		{"project123", "project123"},
		{"", ""},
	}
	for _, tc := range cases {
		got := slugify(tc.in)
		if got != tc.want {
			t.Errorf("slugify(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestGenerateProjectID_Format(t *testing.T) {
	id := generateProjectID("my-project")
	if !strings.HasPrefix(id, "my-project-") {
		t.Errorf("project ID should start with slugified dir name; got %q", id)
	}
	lastDash := strings.LastIndex(id, "-")
	suffix := id[lastDash+1:]
	if len(suffix) != 6 {
		t.Errorf("expected 6-char hex suffix; got %q (full ID: %q)", suffix, id)
	}
}

func TestGenerateProjectName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"my-project", "My Project"},
		{"my_project", "My Project"},
		{"myproject", "Myproject"},
		{"My Project", "My Project"},
	}
	for _, tc := range cases {
		got := generateProjectName(tc.in)
		if got != tc.want {
			t.Errorf("generateProjectName(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestInitProject_WritesProjectMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "DOUG_PROJECT_ID:") {
		t.Errorf("AGENTS.md missing DOUG_PROJECT_ID; got:\n%s", content)
	}
	if !strings.Contains(content, "DOUG_PROJECT_NAME:") {
		t.Errorf("AGENTS.md missing DOUG_PROJECT_NAME; got:\n%s", content)
	}
}

func TestInitProject_PreservesExistingProjectID(t *testing.T) {
	dir := t.TempDir()
	existing := "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nDOUG_PROJECT_ID: original-id-abc123\nDOUG_PROJECT_NAME: Original Name\n\n## Doug-Specific Instructions\n\nSome content.\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	// Run with --force to bypass the guard.
	if err := initProject(dir, true, "", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "DOUG_PROJECT_ID: original-id-abc123") {
		t.Errorf("project ID was not preserved; got:\n%s", content)
	}
}

func TestInitProject_AppendsDougSectionToExistingAgents(t *testing.T) {
	dir := t.TempDir()
	existing := "# Custom Project Instructions\n\nKeep this content.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := initProject(dir, false, "", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Custom Project Instructions") {
		t.Fatalf("existing AGENTS content was not preserved:\n%s", content)
	}
	if strings.Count(content, dougInstructionsMarker) != 1 {
		t.Fatalf("expected one doug section marker, got:\n%s", content)
	}
}

func TestTasksYAMLContent_HasRequiredFields(t *testing.T) {
	content := tasksYAMLContent()
	required := []string{
		`id: "EPIC-1"`,
		`id: "EPIC-1-001"`,
		`id: "EPIC-1-002"`,
		`type: "feature"`,
		`status: "TODO"`,
		"description:",
		"acceptance_criteria:",
	}
	for _, want := range required {
		if !strings.Contains(content, want) {
			t.Errorf("tasks.yaml content missing %q", want)
		}
	}
}
