package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/config"
)

func TestInitProject_GeneratesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
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
	if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
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
	if !strings.Contains(string(agentsData), "## Doug-Specific Instructions") {
		t.Errorf("AGENTS.md missing doug-specific section; got:\n%s", agentsData)
	}
	if !strings.Contains(string(agentsData), "docs/kb/README.md") {
		t.Errorf("AGENTS.md missing KB progressive disclosure entry; got:\n%s", agentsData)
	}
	if !strings.Contains(string(agentsData), "For doug-managed runs launched by `doug`") {
		t.Errorf("AGENTS.md missing conditional doug-managed run guidance; got:\n%s", agentsData)
	}
	if strings.Contains(string(agentsData), "Read `.doug/ACTIVE_TASK.md` for the active task brief when it exists.") {
		t.Errorf("AGENTS.md should not globally route sessions through ACTIVE_TASK.md; got:\n%s", agentsData)
	}

	// .gitignore should be created at the project root with .doug ignored.
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore not created at root: %v", err)
	}
	if !strings.Contains(string(data), ".doug/") {
		t.Errorf(".gitignore missing .doug/ entry; got:\n%s", data)
	}

	// *_TEMPLATE.md files land in .doug/logs/.
	for _, name := range []string{
		"SESSION_RESULTS_TEMPLATE.md",
		"BUG_REPORT_TEMPLATE.md",
		"FAILURE_REPORT_TEMPLATE.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, ".doug", "logs", name)); err != nil {
			t.Errorf(".doug/logs/%s not created: %v", name, err)
		}
	}

	// Skill files land under the selected provider's local skills directory.
	for _, name := range []string{
		filepath.Join("implement-feature", "SKILL.md"),
		filepath.Join("implement-bugfix", "SKILL.md"),
		filepath.Join("implement-documentation", "SKILL.md"),
		filepath.Join("plan", "SKILL.md"),
		filepath.Join("research", "SKILL.md"),
	} {
		if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", name)); err != nil {
			t.Errorf(".claude/skills/%s not created: %v", name, err)
		}
	}

	// skills-config.yaml goes to .doug/
	if _, err := os.Stat(filepath.Join(dir, ".doug", "skills-config.yaml")); err != nil {
		t.Errorf(".doug/skills-config.yaml not created: %v", err)
	}

	// .claude/settings.json is created when claude is selected.
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); err != nil {
		t.Errorf(".claude/settings.json not created: %v", err)
	}

	// .gemini/settings.json should NOT be created by init
	if _, err := os.Stat(filepath.Join(dir, ".gemini", "settings.json")); err == nil {
		t.Errorf(".gemini/settings.json should not be created by init")
	}

	// docs/kb/ directory should be created
	if _, err := os.Stat(filepath.Join(dir, "docs", "kb")); err != nil {
		t.Errorf("docs/kb/ not created: %v", err)
	}
}

func TestInitProject_MultipleAgents(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", []string{"claude", "codex"}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Skills are copied into each selected provider directory.
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "implement-feature", "SKILL.md")); err != nil {
		t.Errorf(".claude/skills/implement-feature/SKILL.md not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "skills", "implement-feature", "SKILL.md")); err != nil {
		t.Errorf(".codex/skills/implement-feature/SKILL.md not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); err != nil {
		t.Errorf(".claude/settings.json not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "config.toml")); err != nil {
		t.Errorf(".codex/config.toml not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gemini", "settings.json")); err == nil {
		t.Error(".gemini/settings.json should not be created when gemini is not selected")
	}
	if _, err := os.Stat(filepath.Join(dir, ".gemini", "skills")); err == nil {
		t.Error(".gemini/skills/ should not be created when gemini is not selected")
	}
}

func TestInitProject_TemplateContent(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
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
		if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "build_system: go") {
			t.Errorf(".doug/doug.yaml does not contain 'build_system: go'; content:\n%s", data)
		}
	})

	t.Run("package.json → build_system: npm", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "build_system: npm") {
			t.Errorf(".doug/doug.yaml does not contain 'build_system: npm'; content:\n%s", data)
		}
	})

	t.Run("pnpm-workspace.yaml → build_system: pnpm", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte("packages:\n  - packages/*\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "build_system: pnpm") {
			t.Errorf(".doug/doug.yaml does not contain 'build_system: pnpm'; content:\n%s", data)
		}
	})

	t.Run("no marker → default build_system: go", func(t *testing.T) {
		dir := t.TempDir()
		if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "build_system: go") {
			t.Errorf(".doug/doug.yaml does not contain 'build_system: go'; content:\n%s", data)
		}
	})
}

func TestInitProject_BuildSystemFlag(t *testing.T) {
	dir := t.TempDir()
	// go.mod exists (would auto-detect as go), but flag overrides to npm.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initProject(dir, false, "npm", []string{"claude"}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "build_system: npm") {
		t.Errorf("--build-system flag not respected; content:\n%s", data)
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
		err := initProject(dir, false, "", []string{"claude"}, false)
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
		// Should not error — guard only checks .doug/project-state.yaml
		if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
			t.Fatalf("unexpected error when stale root tasks.yaml exists: %v", err)
		}
	})
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
		if err := initProject(dir, true, "", []string{"claude"}, false); err != nil {
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
		if err := initProject(dir, true, "", []string{"claude"}, false); err != nil {
			t.Fatalf("unexpected error with force=true: %v", err)
		}
	})
}

func TestInitProject_InvalidBuildSystem(t *testing.T) {
	dir := t.TempDir()
	err := initProject(dir, false, "foobar", []string{"claude"}, false)
	if err == nil {
		t.Fatal("expected error for invalid build system, got nil")
	}
	if !strings.Contains(err.Error(), "foobar") {
		t.Errorf("error should mention the invalid value; got: %v", err)
	}
}

func TestInitProject_BuildSystemFlagPnpm(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "pnpm", []string{"claude"}, false); err != nil {
		t.Fatalf("unexpected error for --build-system pnpm: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "build_system: pnpm") {
		t.Errorf("--build-system pnpm not reflected in doug.yaml; content:\n%s", data)
	}
}

func TestInitProject_CreatesChangelog(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
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
	if !strings.Contains(content, "Keep a Changelog") {
		t.Errorf("CHANGELOG.md missing Keep a Changelog reference; got:\n%s", content)
	}
}

func TestInitProject_DoesNotOverwriteChangelog(t *testing.T) {
	dir := t.TempDir()
	original := "# My existing changelog\n"
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	// Run with force=true — CHANGELOG.md must still not be overwritten.
	if err := initProject(dir, true, "", []string{"claude"}, false); err != nil {
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

func TestInitProject_UnknownAgentWarning(t *testing.T) {
	dir := t.TempDir()
	// Should succeed without error even for an unknown agent.
	if err := initProject(dir, false, "", []string{"unknownbot"}, false); err != nil {
		t.Fatalf("unexpected error for unknown agent: %v", err)
	}
	// No .unknownbot/ directory should be created.
	if _, err := os.Stat(filepath.Join(dir, ".unknownbot")); err == nil {
		t.Error(".unknownbot/ directory should not have been created")
	}
}

func TestDougYAMLContent_HasInlineComments(t *testing.T) {
	content := dougYAMLContent("go", "claude", 3, 10, true)
	requiredFields := []string{
		"agent_command:",
		"build_system:",
		"max_retries:",
		"max_iterations:",
		"kb_enabled:",
		"agent_heartbeat_seconds:",
	}
	for _, field := range requiredFields {
		if !strings.Contains(content, field) {
			t.Errorf("doug.yaml content missing field %q", field)
		}
	}
	// Every field line should have an inline comment.
	for _, line := range strings.Split(content, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, ":") && !strings.Contains(line, "#") {
			t.Errorf("field line missing inline comment: %q", line)
		}
	}
}

func TestDougYAMLContent_ReflectsPromptedValues(t *testing.T) {
	content := dougYAMLContent("npm", "claude", 7, 15, false)
	if !strings.Contains(content, "max_retries: 7") {
		t.Errorf("expected max_retries: 7 in content; got:\n%s", content)
	}
	if !strings.Contains(content, "max_iterations: 15") {
		t.Errorf("expected max_iterations: 15 in content; got:\n%s", content)
	}
	if !strings.Contains(content, "kb_enabled: false") {
		t.Errorf("expected kb_enabled: false in content; got:\n%s", content)
	}
	if !strings.Contains(content, "build_system: npm") {
		t.Errorf("expected build_system: npm in content; got:\n%s", content)
	}
}

func TestDougYAMLContent_HasCommentedAgentExamples(t *testing.T) {
	content := dougYAMLContent("go", "claude", 3, 10, true)

	wantComments := []string{
		`# agent_command: codex exec`,
		`# agent_command: gemini --approval-mode auto_edit --output-format json --sandbox`,
	}
	for _, want := range wantComments {
		if !strings.Contains(content, want) {
			t.Errorf("doug.yaml content missing commented example %q", want)
		}
	}

	// Default active agent_command must remain claude (uncommented).
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "agent_command:") {
			if !strings.Contains(line, "claude") {
				t.Errorf("default agent_command line must use claude; got: %q", line)
			}
			break
		}
	}
}

func TestInitProject_AgentCommandMatchesSelection(t *testing.T) {
	for _, agent := range []string{"claude", "codex", "gemini"} {
		t.Run(agent, func(t *testing.T) {
			dir := t.TempDir()
			if err := initProject(dir, false, "go", []string{agent}, true); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			// The active agent_command line (not a comment) must contain the agent name.
			for _, line := range strings.Split(content, "\n") {
				if strings.HasPrefix(line, "agent_command:") {
					if !strings.Contains(line, agent) {
						t.Errorf("agent_command line does not contain %q; got: %q", agent, line)
					}
					return
				}
			}
			t.Errorf("no uncommented agent_command line found in doug.yaml:\n%s", content)
		})
	}
}

func TestInitProject_MergesClaudeSettings(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"custom":true,"permissions":{"allow":["Bash(custom *)"]}}`
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid json after merge: %v", err)
	}

	if got["custom"] != true {
		t.Fatalf("custom key was not preserved")
	}
	if got["defaultMode"] != "dontAsk" {
		t.Fatalf("defaultMode missing/incorrect: %#v", got["defaultMode"])
	}
}

func TestInitProject_MergesCodexConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := "web_search = \"live\"\ncustom_key = \"keep\"\n\n[sandbox_workspace_write]\nnetwork_access = true\n"
	if err := os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := initProject(dir, false, "", []string{"codex"}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	for _, want := range []string{
		`approval_policy = "never"`,
		`sandbox_mode = "workspace-write"`,
		`web_search = "cached"`,
		`custom_key = "keep"`,
		`[sandbox_workspace_write]`,
		`network_access = false`,
		`writable_roots = []`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("merged codex config missing %q; content:\n%s", want, content)
		}
	}
}

func TestInitProject_MergesGitignore(t *testing.T) {
	dir := t.TempDir()
	existing := "node_modules/\n.env\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
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
		got := mergeAgents("", section, testID, testName)
		if got != section {
			t.Fatalf("expected section only, got:\n%s", got)
		}
	})

	t.Run("appends section when marker absent", func(t *testing.T) {
		existing := "# Local Instructions\n\nKeep this.\n"
		got := mergeAgents(existing, section, testID, testName)
		if !strings.Contains(got, "# Local Instructions") {
			t.Fatalf("expected existing content to be preserved, got:\n%s", got)
		}
		if strings.Count(got, dougInstructionsMarker) != 1 {
			t.Fatalf("expected exactly one doug marker, got:\n%s", got)
		}
	})

	t.Run("does not append duplicate section when marker already present", func(t *testing.T) {
		existing := "# Local Instructions\n\n" + section
		got := mergeAgents(existing, section, testID, testName)
		if strings.Count(got, dougInstructionsMarker) != 1 {
			t.Fatalf("expected one doug marker, got:\n%s", got)
		}
	})
}

func TestMergeAgents_InjectsMetadataIntoExistingBlock(t *testing.T) {
	// Existing block without metadata (older doug init).
	existing := "# Heading\n\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\n## Doug-Specific Instructions\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"
	section := "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nDOUG_PROJECT_ID: proj-abc123\nDOUG_PROJECT_NAME: My Proj\n\n## Doug-Specific Instructions\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"
	got := mergeAgents(existing, section, "proj-abc123", "My Proj")

	if !strings.Contains(got, "DOUG_PROJECT_ID: proj-abc123") {
		t.Fatalf("expected DOUG_PROJECT_ID to be injected; got:\n%s", got)
	}
	if !strings.Contains(got, "DOUG_PROJECT_NAME: My Proj") {
		t.Fatalf("expected DOUG_PROJECT_NAME to be injected; got:\n%s", got)
	}
	if strings.Count(got, dougInstructionsMarker) != 1 {
		t.Fatalf("expected one START marker; got:\n%s", got)
	}
}

func TestMergeAgents_PreservesExistingMetadataInBlock(t *testing.T) {
	existing := "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nDOUG_PROJECT_ID: existing-id\nDOUG_PROJECT_NAME: Existing Name\n\n## Doug-Specific Instructions\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"
	section := "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->\nDOUG_PROJECT_ID: new-id\nDOUG_PROJECT_NAME: New Name\n\n## Doug-Specific Instructions\n<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->\n"
	got := mergeAgents(existing, section, "new-id", "New Name")

	if !strings.Contains(got, "DOUG_PROJECT_ID: existing-id") {
		t.Fatalf("expected existing ID to be preserved; got:\n%s", got)
	}
	if strings.Contains(got, "DOUG_PROJECT_ID: new-id") {
		t.Fatalf("new ID should not replace existing ID; got:\n%s", got)
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
	if err := initProject(dir, false, "", []string{"claude"}, true); err != nil {
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
	if err := initProject(dir, true, "", []string{"claude"}, true); err != nil {
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

	if err := initProject(dir, false, "", []string{"claude"}, false); err != nil {
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

// ---------------------------------------------------------------------------
// injectBuildSystemPermissions tests
// ---------------------------------------------------------------------------

func TestInjectBuildSystemPermissions(t *testing.T) {
	t.Run("injects npm permissions into valid JSON", func(t *testing.T) {
		base := []byte(`{"permissions":{"allow":["Read","Write"]}}`)
		out, err := injectBuildSystemPermissions(base, "npm")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, perm := range config.BuildSystems["npm"].Permissions {
			if !strings.Contains(string(out), perm) {
				t.Errorf("expected permission %q in output; got:\n%s", perm, out)
			}
		}
		// Existing perms preserved.
		if !strings.Contains(string(out), "Read") {
			t.Error("existing 'Read' permission should be preserved")
		}
	})

	t.Run("returns error on malformed JSON", func(t *testing.T) {
		_, err := injectBuildSystemPermissions([]byte(`{invalid}`), "go")
		if err == nil {
			t.Fatal("expected error for malformed JSON, got nil")
		}
	})

	t.Run("returns template unchanged for empty build system", func(t *testing.T) {
		base := []byte(`{"permissions":{"allow":["Read"]}}`)
		out, err := injectBuildSystemPermissions(base, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(out) != string(base) {
			t.Errorf("expected unchanged template; got:\n%s", out)
		}
	})

	t.Run("returns template unchanged for unknown build system", func(t *testing.T) {
		base := []byte(`{"permissions":{"allow":["Read"]}}`)
		out, err := injectBuildSystemPermissions(base, "rust")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(out) != string(base) {
			t.Errorf("expected unchanged template; got:\n%s", out)
		}
	})

	t.Run("deduplicates permissions that already exist", func(t *testing.T) {
		goPerm := config.BuildSystems["go"].Permissions[0]
		base := []byte(`{"permissions":{"allow":["` + goPerm + `"]}}`)
		out, err := injectBuildSystemPermissions(base, "go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Count(string(out), goPerm) != 1 {
			t.Errorf("permission %q should appear exactly once; got:\n%s", goPerm, out)
		}
	})
}

// ---------------------------------------------------------------------------
// Build system permission injection via initProject
// ---------------------------------------------------------------------------

func readSettingsJSON(t *testing.T, dir string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read .claude/settings.json: %v", err)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal .claude/settings.json: %v", err)
	}
	return obj
}

func settingsAllowList(t *testing.T, dir string) []string {
	t.Helper()
	obj := readSettingsJSON(t, dir)
	perms, _ := obj["permissions"].(map[string]interface{})
	allow, _ := perms["allow"].([]interface{})
	var out []string
	for _, v := range allow {
		s, _ := v.(string)
		out = append(out, s)
	}
	return out
}

func containsAll(haystack []string, needles []string) (string, bool) {
	set := make(map[string]bool, len(haystack))
	for _, s := range haystack {
		set[s] = true
	}
	for _, n := range needles {
		if !set[n] {
			return n, false
		}
	}
	return "", true
}

func TestInitProject_InjectsGoPermissions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initProject(dir, false, "", []string{"claude"}, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allow := settingsAllowList(t, dir)
	if missing, ok := containsAll(allow, config.BuildSystems["go"].Permissions); !ok {
		t.Errorf("go permission %q missing from settings.json allow list", missing)
	}
	for _, npmPerm := range config.BuildSystems["npm"].Permissions {
		for _, a := range allow {
			if a == npmPerm {
				t.Errorf("npm permission %q should not be in go project settings.json", npmPerm)
			}
		}
	}
}

func TestInitProject_InjectsNpmPermissions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initProject(dir, false, "", []string{"claude"}, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allow := settingsAllowList(t, dir)
	if missing, ok := containsAll(allow, config.BuildSystems["npm"].Permissions); !ok {
		t.Errorf("npm permission %q missing from settings.json allow list", missing)
	}
}

func TestInitProject_InjectsPnpmPermissions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte("packages:\n  - packages/*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initProject(dir, false, "", []string{"claude"}, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allow := settingsAllowList(t, dir)
	if missing, ok := containsAll(allow, config.BuildSystems["pnpm"].Permissions); !ok {
		t.Errorf("pnpm permission %q missing from settings.json allow list", missing)
	}
}

func TestInitProject_BuildSystemFlagInjectsPermissions(t *testing.T) {
	dir := t.TempDir()
	// Empty dir — no marker files, but flag overrides to npm.
	if err := initProject(dir, false, "npm", []string{"claude"}, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allow := settingsAllowList(t, dir)
	if missing, ok := containsAll(allow, config.BuildSystems["npm"].Permissions); !ok {
		t.Errorf("npm permission %q missing when --build-system npm used; allow=%v", missing, allow)
	}
}

func TestInitProject_MergeAppendsBuildSystemPerms(t *testing.T) {
	dir := t.TempDir()
	// Pre-existing settings.json with a custom permission.
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"permissions":{"allow":["Bash(custom-tool *)"]}}`
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := initProject(dir, false, "npm", []string{"claude"}, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allow := settingsAllowList(t, dir)

	// Custom perm preserved.
	found := false
	for _, a := range allow {
		if a == "Bash(custom-tool *)" {
			found = true
		}
	}
	if !found {
		t.Error("custom permission 'Bash(custom-tool *)' was not preserved after merge")
	}

	// npm perms injected.
	if missing, ok := containsAll(allow, config.BuildSystems["npm"].Permissions); !ok {
		t.Errorf("npm permission %q missing after merge; allow=%v", missing, allow)
	}
}
