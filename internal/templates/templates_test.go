package templates_test

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/templates"
)

func TestRuntimeFS_ContainsSessionResult(t *testing.T) {
	f, err := templates.Runtime.Open("runtime/session_result.md")
	if err != nil {
		t.Fatalf("runtime/session_result.md not found in embedded Runtime FS: %v", err)
	}
	if closeErr := f.Close(); closeErr != nil {
		t.Fatalf("close runtime/session_result.md: %v", closeErr)
	}
}

func TestInitFS_ContainsExpectedFiles(t *testing.T) {
	expectedFiles := []string{
		"init/CLAUDE.md",
		"init/AGENTS.md",
		"init/.gitignore",
		"init/.claude/settings.json",
		"init/.codex/config.toml",
		"init/.gemini/settings.json",
		"init/.gemini/policies/doug-default.json",
		"init/SESSION_RESULTS_TEMPLATE.md",
		"init/BUG_REPORT_TEMPLATE.md",
		"init/FAILURE_REPORT_TEMPLATE.md",
		"init/skills/implement-feature/SKILL.md",
		"init/skills/implement-bugfix/SKILL.md",
		"init/skills/implement-documentation/SKILL.md",
		"init/skills/plan/SKILL.md",
		"init/skills/plan/references/discovery.md",
		"init/skills/plan/references/roadmapping.md",
		"init/skills/plan/references/definition.md",
		"init/skills/plan/references/feature.md",
		"init/skills/plan/references/refactor.md",
		"init/skills/plan/references/bugfix.md",
		"init/skills/plan/references/greenfield.md",
		"init/skills/scaffold/SKILL.md",
	}
	for _, path := range expectedFiles {
		f, err := templates.Init.Open(path)
		if err != nil {
			t.Errorf("expected file %q not found in embedded Init FS: %v", path, err)
			continue
		}
		if closeErr := f.Close(); closeErr != nil {
			t.Errorf("close %q: %v", path, closeErr)
		}
	}

	if _, err := templates.Init.Open("init/settings.json"); err == nil {
		t.Error("init/settings.json should not be present in the embedded FS")
	}
}

func TestInitSkillTemplates_KeepWorkflowBoundary(t *testing.T) {
	cases := []struct {
		path      string
		forbidden []string
		required  []string
	}{
		{
			path: "init/skills/plan/SKILL.md",
			forbidden: []string{
				"use `.doug/ACTIVE_TASK.md` as the planning brief",
				"Write the result into the `## Agent Result` block in `.doug/ACTIVE_TASK.md`",
			},
			required: []string{
				"planning brief provided by the user, launch prompt, or repository workflow",
				"repository's designated planning artifact",
				"deterministic derivative artifacts out of scope",
				"combine them as needed",
				"Report the result using the mechanism defined by the repository instructions or task brief",
			},
		},
		{
			path: "init/skills/scaffold/SKILL.md",
			forbidden: []string{
				"provided in ACTIVE_TASK.md",
				"use `.doug/ACTIVE_TASK.md` as the source of truth for the scaffold task",
				"Write the result into the `## Agent Result` block in `.doug/ACTIVE_TASK.md`",
			},
			required: []string{
				"manifest or structured scaffold brief",
				"source of truth for the requested stack, dependencies, and constraints",
				"Report the result using the mechanism defined by the repository instructions or task brief",
			},
		},
	}

	for _, tc := range cases {
		data, err := templates.Init.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		content := string(data)

		for _, forbidden := range tc.forbidden {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s must not contain %q", tc.path, forbidden)
			}
		}
		for _, required := range tc.required {
			if !strings.Contains(content, required) {
				t.Errorf("%s missing required contract text %q", tc.path, required)
			}
		}
	}
}

func TestSessionResult_ThreeFrontmatterFieldsOnly(t *testing.T) {
	content := templates.SessionResult

	// Must have exactly the three required fields.
	for _, want := range []string{`outcome: ""`, `changelog_entry: ""`, "dependencies_added: []"} {
		if !strings.Contains(content, want) {
			t.Errorf("runtime/session_result.md missing required frontmatter field %q", want)
		}
	}

	// Must NOT have any of the removed fields.
	for _, forbidden := range []string{"task_id:", "timestamp:", "files_modified:", "tests_run:", "build_successful:"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("runtime/session_result.md must not contain frontmatter field %q", forbidden)
		}
	}
}
