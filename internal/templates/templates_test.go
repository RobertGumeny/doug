package templates_test

import (
	"os"
	"path/filepath"
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
		"init/BUG_REPORT_TEMPLATE.md",
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
	if _, err := templates.Init.Open("init/SESSION_RESULTS_TEMPLATE.md"); err == nil {
		t.Error("SESSION_RESULTS_TEMPLATE.md should not be embedded; ACTIVE_TASK.md is the sole result handshake surface")
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

func TestInitTemplateFS_HasSingleOutcomeBearingHandshakeTemplate(t *testing.T) {
	entries, err := templates.Init.ReadDir("init")
	if err != nil {
		t.Fatalf("ReadDir init: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "BUG_REPORT_TEMPLATE.md" {
			continue
		}
		data, err := templates.Init.ReadFile("init/" + entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if strings.Contains(string(data), "outcome: \"\"") {
			t.Fatalf("init/%s contains an outcome result block; ACTIVE_TASK.md must be the only managed result handshake", entry.Name())
		}
	}
}

func TestInitDougReadmeDocumentsCurrentWorkspaceLayout(t *testing.T) {
	data, err := templates.Init.ReadFile("init/DOUG_README.md")
	if err != nil {
		t.Fatalf("read init/DOUG_README.md: %v", err)
	}
	content := string(data)

	for _, required := range []string{
		"`intake/`",
		"`logs/epics/`",
		"`templates/`",
		"`run.lock`",
		"`plan/epics/`",
		"`plan/history/`",
		"doug stats",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("init/DOUG_README.md missing required workspace guidance %q", required)
		}
	}
}

func TestRepositoryFacingDocsMentionCurrentResearchAndStatsContracts(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))

	readme, err := os.ReadFile(filepath.Join(repoRoot, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !strings.Contains(string(readme), "doug stats [EPIC-ID]") {
		t.Error("README.md must list doug stats in the command summary")
	}

	contributing, err := os.ReadFile(filepath.Join(repoRoot, "CONTRIBUTING.md"))
	if err != nil {
		t.Fatalf("read CONTRIBUTING.md: %v", err)
	}
	for _, required := range []string{
		".doug/intake/research/",
		".doug/PRD.md",
		".doug/tasks.yaml",
	} {
		if !strings.Contains(string(contributing), required) {
			t.Errorf("CONTRIBUTING.md missing required research guidance %q", required)
		}
	}
}

func TestInitAgentsTemplate_ContainsManagedBlockContent(t *testing.T) {
	data, err := templates.Init.ReadFile("init/AGENTS.md")
	if err != nil {
		t.Fatalf("read init/AGENTS.md: %v", err)
	}
	content := string(data)

	// These prose patterns belong to no version of the managed block.
	for _, forbidden := range []string{
		"Progressive Disclosure",
		"Working Rules",
		"Doug-Specific Instructions",
		"ACTIVE_BUG.md",
		"ACTIVE_FAILURE.md",
	} {
		if strings.Contains(content, forbidden) {
			t.Errorf("init/AGENTS.md must not contain %q", forbidden)
		}
	}

	// Project identity fields.
	for _, required := range []string{
		"DOUG_PROJECT_ID",
		"DOUG_PROJECT_NAME",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("init/AGENTS.md missing required identity field %q", required)
		}
	}

	// Operating rules that must be present so agents know the workflow contract.
	for _, required := range []string{
		".doug/ACTIVE_TASK.md",
		"canonical task brief",
		"BUG_REPORT_TEMPLATE.md",
		".doug/logs/bugs/",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("init/AGENTS.md missing required operating-rules content %q", required)
		}
	}
}
