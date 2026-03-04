package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitProject_GeneratesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", []string{"claude"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// doug.yaml lives in .doug/
	if _, err := os.Stat(filepath.Join(dir, ".doug", "doug.yaml")); err != nil {
		t.Errorf("file .doug/doug.yaml not created: %v", err)
	}
	// tasks.yaml lives in .doug/, PRD.md stays at root
	if _, err := os.Stat(filepath.Join(dir, ".doug", "tasks.yaml")); err != nil {
		t.Errorf("file .doug/tasks.yaml not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "PRD.md")); err != nil {
		t.Errorf("file PRD.md not created: %v", err)
	}
}

func TestInitProject_CopiesTemplateFiles(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", []string{"claude"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// CLAUDE.md and AGENTS.md should NOT be created (skipped in new routing).
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s should not be created at root (skipped in new routing)", name)
		}
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

	// Skill files land under .claude/skills/ (claude is default agent).
	for _, name := range []string{
		filepath.Join("implement-feature", "SKILL.md"),
		filepath.Join("implement-bugfix", "SKILL.md"),
		filepath.Join("implement-documentation", "SKILL.md"),
	} {
		if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", name)); err != nil {
			t.Errorf(".claude/skills/%s not created: %v", name, err)
		}
	}

	// skills-config.yaml goes to .doug/
	if _, err := os.Stat(filepath.Join(dir, ".doug", "skills-config.yaml")); err != nil {
		t.Errorf(".doug/skills-config.yaml not created: %v", err)
	}

	// docs/kb/ directory should be created
	if _, err := os.Stat(filepath.Join(dir, "docs", "kb")); err != nil {
		t.Errorf("docs/kb/ not created: %v", err)
	}
}

func TestInitProject_MultipleAgents(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", []string{"claude", "codex"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Skills for claude
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "implement-feature", "SKILL.md")); err != nil {
		t.Errorf(".claude/skills/implement-feature/SKILL.md not created: %v", err)
	}
	// Skills for codex
	if _, err := os.Stat(filepath.Join(dir, ".codex", "skills", "implement-feature", "SKILL.md")); err != nil {
		t.Errorf(".codex/skills/implement-feature/SKILL.md not created: %v", err)
	}
}

func TestInitProject_TemplateContent(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "", []string{"claude"}); err != nil {
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
		if err := initProject(dir, false, "", []string{"claude"}); err != nil {
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
		if err := initProject(dir, false, "", []string{"claude"}); err != nil {
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

	t.Run("no marker → default build_system: go", func(t *testing.T) {
		dir := t.TempDir()
		if err := initProject(dir, false, "", []string{"claude"}); err != nil {
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
	if err := initProject(dir, false, "npm", []string{"claude"}); err != nil {
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
		err := initProject(dir, false, "", []string{"claude"})
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
		if err := initProject(dir, false, "", []string{"claude"}); err != nil {
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
		if err := initProject(dir, true, "", []string{"claude"}); err != nil {
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
		if err := initProject(dir, true, "", []string{"claude"}); err != nil {
			t.Fatalf("unexpected error with force=true: %v", err)
		}
	})
}

func TestInitProject_InvalidBuildSystem(t *testing.T) {
	dir := t.TempDir()
	err := initProject(dir, false, "foobar", []string{"claude"})
	if err == nil {
		t.Fatal("expected error for invalid build system, got nil")
	}
	if !strings.Contains(err.Error(), "foobar") {
		t.Errorf("error should mention the invalid value; got: %v", err)
	}
}

func TestInitProject_UnknownAgentWarning(t *testing.T) {
	dir := t.TempDir()
	// Should succeed without error even for an unknown agent.
	if err := initProject(dir, false, "", []string{"unknownbot"}); err != nil {
		t.Fatalf("unexpected error for unknown agent: %v", err)
	}
	// No .unknownbot/ directory should be created.
	if _, err := os.Stat(filepath.Join(dir, ".unknownbot")); err == nil {
		t.Error(".unknownbot/ directory should not have been created")
	}
}

func TestInitProject_SkillsDirMatchesAgent(t *testing.T) {
	tests := []struct {
		name      string
		agents    []string
		wantInYAML string
	}{
		{"codex agent", []string{"codex"}, "skills_dir: .codex/skills"},
		{"claude agent", []string{"claude"}, "skills_dir: .claude/skills"},
		{"codex first wins", []string{"codex", "claude"}, "skills_dir: .codex/skills"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := initProject(dir, false, "", tt.agents); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(dir, ".doug", "doug.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tt.wantInYAML) {
				t.Errorf("doug.yaml missing %q; content:\n%s", tt.wantInYAML, data)
			}
		})
	}
}

func TestDougYAMLContent_HasInlineComments(t *testing.T) {
	content := dougYAMLContent("go", ".claude/skills")
	requiredFields := []string{
		"agent_command:",
		"skills_dir:",
		"build_system:",
		"max_retries:",
		"max_iterations:",
		"kb_enabled:",
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
