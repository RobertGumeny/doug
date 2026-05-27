package templates_test

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/templates"
)

func TestInitFS_ContainsExpectedFiles(t *testing.T) {
	expectedFiles := []string{
		"init/CLAUDE.md",
		"init/AGENTS.md",
		"init/DOUG_README.md",
		"init/.gitignore",
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
		"init/.pi/extensions/handoff.ts",
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
				"Doug",
				"doug",
				".doug",
				"ACTIVE_TASK.md",
				"PLAN.md",
				"PRD",
				"handoff",
				"deterministic",
				"canonical run brief",
				"task contract",
			},
			required: []string{
				"planning brief provided by the user, launch prompt, or repository workflow",
				"repository's designated planning artifact",
				"combine them as needed",
				"Implementation-Ready",
				"Report the result using the mechanism defined by the repository instructions or task brief",
			},
		},
		{
			path: "init/skills/research/SKILL.md",
			forbidden: []string{
				"Doug",
				"doug",
				".doug",
				"PRD Alignment",
				"ACTIVE_TASK.md",
			},
			required: []string{
				"Create or update the research report requested by the user or repository workflow",
				"If no output path is specified, ask where the report should be saved",
				"Requirement Alignment",
			},
		},
		{
			path: "init/skills/scaffold/SKILL.md",
			forbidden: []string{
				"provided in ACTIVE_TASK.md",
				"use `.doug/ACTIVE_TASK.md` as the source of truth for the scaffold task",
				"Write the result into the `## Agent Result` block in `.doug/ACTIVE_TASK.md`",
				"Report `SUCCESS`",
				"report `SUCCESS`",
			},
			required: []string{
				"manifest or structured scaffold brief",
				"source of truth for the requested stack, dependencies, and constraints",
				"Report the result using the mechanism defined by the repository instructions or task brief",
				"Report completion only after any required install step has completed without error",
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

func TestInitAgentsTemplate_ContainsOnlyProjectIdentity(t *testing.T) {
	data, err := templates.Init.ReadFile("init/AGENTS.md")
	if err != nil {
		t.Fatalf("read init/AGENTS.md: %v", err)
	}
	content := string(data)

	for _, forbidden := range []string{
		"Progressive Disclosure",
		"Working Rules",
		"Doug-Specific Instructions",
		"canonical task brief",
		"ACTIVE_BUG.md",
	} {
		if strings.Contains(content, forbidden) {
			t.Errorf("init/AGENTS.md should not contain operating rules — found %q", forbidden)
		}
	}

	for _, required := range []string{
		"DOUG_PROJECT_ID",
		"DOUG_PROJECT_NAME",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("init/AGENTS.md missing required identity field %q", required)
		}
	}
}
