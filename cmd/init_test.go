package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitProject_GeneratesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{"doug.yaml", "tasks.yaml", "PRD.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("file %s not created: %v", name, err)
		}
	}
}

func TestInitProject_CopiesTemplateFiles(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// CLAUDE.md and AGENTS.md land at project root.
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("root file %s not created: %v", name, err)
		}
	}

	// *_TEMPLATE.md files land in logs/.
	for _, name := range []string{
		"SESSION_RESULTS_TEMPLATE.md",
		"BUG_REPORT_TEMPLATE.md",
		"FAILURE_REPORT_TEMPLATE.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, "logs", name)); err != nil {
			t.Errorf("logs/%s not created: %v", name, err)
		}
	}

	// Skill files land under .claude/skills/.
	for _, name := range []string{
		filepath.Join("implement-feature", "SKILL.md"),
		filepath.Join("implement-bugfix", "SKILL.md"),
		filepath.Join("implement-documentation", "SKILL.md"),
	} {
		if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", name)); err != nil {
			t.Errorf(".claude/skills/%s not created: %v", name, err)
		}
	}
}

func TestInitProject_TemplateContent(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SESSION_RESULTS_TEMPLATE.md should have three frontmatter fields only.
	data, err := os.ReadFile(filepath.Join(dir, "logs", "SESSION_RESULTS_TEMPLATE.md"))
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
		if err := initProject(dir, false, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "doug.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "build_system: go") {
			t.Errorf("doug.yaml does not contain 'build_system: go'; content:\n%s", data)
		}
	})

	t.Run("package.json → build_system: npm", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initProject(dir, false, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "doug.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "build_system: npm") {
			t.Errorf("doug.yaml does not contain 'build_system: npm'; content:\n%s", data)
		}
	})

	t.Run("no marker → default build_system: go", func(t *testing.T) {
		dir := t.TempDir()
		if err := initProject(dir, false, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "doug.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "build_system: go") {
			t.Errorf("doug.yaml does not contain 'build_system: go'; content:\n%s", data)
		}
	})
}

func TestInitProject_BuildSystemFlag(t *testing.T) {
	dir := t.TempDir()
	// go.mod exists (would auto-detect as go), but flag overrides to npm.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := initProject(dir, false, "npm"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "doug.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "build_system: npm") {
		t.Errorf("--build-system flag not respected; content:\n%s", data)
	}
}

func TestInitProject_GuardCheck(t *testing.T) {
	for _, existingFile := range []string{"project-state.yaml", "tasks.yaml"} {
		t.Run("exits with error if "+existingFile+" exists", func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, existingFile), []byte("existing content"), 0o644); err != nil {
				t.Fatal(err)
			}
			err := initProject(dir, false, "")
			if err == nil {
				t.Fatal("expected error when existing file present, got nil")
			}
			if !strings.Contains(err.Error(), existingFile) {
				t.Errorf("error message should mention %q; got: %v", existingFile, err)
			}
		})
	}
}

func TestInitProject_Force(t *testing.T) {
	t.Run("overwrites tasks.yaml when force=true", func(t *testing.T) {
		dir := t.TempDir()
		original := "original content — should be replaced"
		if err := os.WriteFile(filepath.Join(dir, "tasks.yaml"), []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initProject(dir, true, ""); err != nil {
			t.Fatalf("unexpected error with force=true: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "tasks.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) == original {
			t.Error("tasks.yaml was not overwritten with --force")
		}
		if !strings.Contains(string(data), "EPIC-1") {
			t.Errorf("tasks.yaml does not contain expected content; got:\n%s", data)
		}
	})

	t.Run("proceeds without error when project-state.yaml exists and force=true", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "project-state.yaml"), []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initProject(dir, true, ""); err != nil {
			t.Fatalf("unexpected error with force=true: %v", err)
		}
	})
}

func TestDougYAMLContent_HasInlineComments(t *testing.T) {
	content := dougYAMLContent("go")
	requiredFields := []string{
		"agent_command:",
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
