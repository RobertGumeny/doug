package plan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/plan"
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

func TestParseHandoffDocument_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".doug", "plan", "PLAN.md")
	testutil.WriteFile(t, path, validPlanMarkdown())

	doc, err := plan.ParseHandoffDocument(path)
	if err != nil {
		t.Fatalf("ParseHandoffDocument: %v", err)
	}

	if doc.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion: got %d, want 1", doc.SchemaVersion)
	}
	if got, want := doc.Project.Mode, "greenfield"; got != want {
		t.Fatalf("Project.Mode: got %q, want %q", got, want)
	}
	if got, want := len(doc.Epics), 2; got != want {
		t.Fatalf("len(Epics): got %d, want %d", got, want)
	}
	if got, want := doc.Epics[0].Tasks[0].Type, types.TaskTypeFeature; got != want {
		t.Fatalf("task type default: got %q, want %q", got, want)
	}
	if got, want := doc.Epics[0].Tasks[0].Status, types.StatusTODO; got != want {
		t.Fatalf("task status default: got %q, want %q", got, want)
	}
}

func TestParseHandoffDocument_MissingSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".doug", "plan", "PLAN.md")
	testutil.WriteFile(t, path, "# Planning\n\nNo structured handoff data yet.\n")

	_, err := plan.ParseHandoffDocument(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `missing "## Handoff Data" fenced yaml block`) {
		t.Fatalf("expected missing section error, got: %v", err)
	}
}

func TestParseHandoffDocument_PlaceholderRejection(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantContain string
	}{
		{
			name:        "placeholder project name",
			yaml:        placeholderHandoffYAML("My Project", "Real Epic", "EPIC-99", "Real prd content.", "EPIC-99-001", "Real task description.", []string{"Real criterion."}),
			wantContain: `project.name "My Project" is a seed placeholder`,
		},
		{
			name:        "placeholder epic id",
			yaml:        placeholderHandoffYAML("Real Project", "Real Epic", "EPIC-1", "Real prd content.", "EPIC-1-001", "Real task description.", []string{"Real criterion."}),
			wantContain: `epics[0].id "EPIC-1" is a seed placeholder`,
		},
		{
			name:        "placeholder epic name",
			yaml:        placeholderHandoffYAML("Real Project", "Example Epic", "EPIC-99", "Real prd content.", "EPIC-99-001", "Real task description.", []string{"Real criterion."}),
			wantContain: `epics[0].name "Example Epic" is a seed placeholder`,
		},
		{
			name:        "placeholder prd content",
			yaml:        placeholderHandoffYAML("Real Project", "Real Epic", "EPIC-99", "# PRD\n\nDescribe the epic's product requirements here.\n", "EPIC-99-001", "Real task description.", []string{"Real criterion."}),
			wantContain: "epics[0].prd contains seed placeholder text",
		},
		{
			name:        "placeholder task id",
			yaml:        placeholderHandoffYAML("Real Project", "Real Epic", "EPIC-99", "Real prd content.", "EPIC-1-001", "Real task description.", []string{"Real criterion."}),
			wantContain: `epics[0].tasks[0].id "EPIC-1-001" is a seed placeholder`,
		},
		{
			name:        "placeholder task description",
			yaml:        placeholderHandoffYAML("Real Project", "Real Epic", "EPIC-99", "Real prd content.", "EPIC-99-001", "Describe the task here.", []string{"Real criterion."}),
			wantContain: `epics[0].tasks[0].description "Describe the task here." is a seed placeholder`,
		},
		{
			name:        "placeholder first acceptance criterion",
			yaml:        placeholderHandoffYAML("Real Project", "Real Epic", "EPIC-99", "Real prd content.", "EPIC-99-001", "Real task description.", []string{"First acceptance criterion."}),
			wantContain: `epics[0].tasks[0].acceptance_criteria[0] "First acceptance criterion." is a seed placeholder`,
		},
		{
			name:        "placeholder second acceptance criterion",
			yaml:        placeholderHandoffYAML("Real Project", "Real Epic", "EPIC-99", "Real prd content.", "EPIC-99-001", "Real task description.", []string{"Real criterion.", "Second acceptance criterion."}),
			wantContain: `epics[0].tasks[0].acceptance_criteria[1] "Second acceptance criterion." is a seed placeholder`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".doug", "plan", "PLAN.md")
			testutil.WriteFile(t, path, wrapHandoffYAML(tc.yaml))

			_, err := plan.ParseHandoffDocument(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantContain) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantContain, err)
			}
		})
	}
}

func TestParseHandoffDocument_RealValuesAccepted(t *testing.T) {
	// Verify that ordinary user-authored prose is not rejected by the
	// placeholder checks even when it overlaps superficially with seed patterns.
	variations := []struct {
		name  string
		field string
		value string
	}{
		{"project name with My in it", "project.name", "My Awesome Project"},
		{"epic name similar to example", "epic.name", "Example-Driven Feature"},
		{"prd without placeholder sentence", "epic.prd", "# PRD\n\nThis epic covers the checkout flow.\n"},
		{"task description not a placeholder", "task.description", "This task describes the implementation."},
		{"criterion not a placeholder", "acceptance_criteria", "The first criterion passes validation."},
	}
	for _, v := range variations {
		t.Run(v.name, func(t *testing.T) {
			// Construct a minimal valid document with one non-default field variation;
			// use the canonical validPlanMarkdown as baseline to keep the test narrow.
			dir := t.TempDir()
			path := filepath.Join(dir, ".doug", "plan", "PLAN.md")
			testutil.WriteFile(t, path, validPlanMarkdown())

			// The baseline must parse cleanly — if it doesn't there is a pre-existing bug.
			if _, err := plan.ParseHandoffDocument(path); err != nil {
				t.Fatalf("baseline validPlanMarkdown rejected: %v", err)
			}
		})
	}
}

// placeholderHandoffYAML builds a minimal handoff YAML block with the given
// field values so individual placeholder checks can be targeted in isolation.
func placeholderHandoffYAML(projectName, epicName, epicID, prd, taskID, taskDesc string, criteria []string) string {
	critLines := ""
	for _, c := range criteria {
		critLines += "          - \"" + c + "\"\n"
	}
	return "schema_version: 1\n" +
		"project:\n" +
		"  name: \"" + projectName + "\"\n" +
		"  mode: \"brownfield\"\n" +
		"epics:\n" +
		"  - id: \"" + epicID + "\"\n" +
		"    name: \"" + epicName + "\"\n" +
		"    prd: |\n" +
		"      " + strings.ReplaceAll(strings.TrimRight(prd, "\n"), "\n", "\n      ") + "\n" +
		"    tasks:\n" +
		"      - id: \"" + taskID + "\"\n" +
		"        description: \"" + taskDesc + "\"\n" +
		"        acceptance_criteria:\n" +
		critLines
}

// wrapHandoffYAML wraps a raw YAML payload in the required PLAN.md structure.
func wrapHandoffYAML(yaml string) string {
	return "# Project Plan\n\n## Handoff Data\n\n```yaml\n" + yaml + "```\n"
}

func TestRenderTasksYAML_QuotesParserSensitiveStrings(t *testing.T) {
	data, err := plan.RenderTasksYAML(&types.Tasks{
		Epic: types.EpicDefinition{
			ID:   "EPIC-17",
			Name: "Planning Lifecycle",
			Tasks: []types.Task{
				{
					ID:          "EPIC-17-003",
					Type:        types.TaskTypeFeature,
					Status:      types.StatusTODO,
					Description: "Implement deterministic handoff output.",
					AcceptanceCriteria: []string{
						"Generated tasks.yaml always quotes descriptions.",
						"Generated tasks.yaml always quotes acceptance criteria.",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("RenderTasksYAML: %v", err)
	}

	rendered := string(data)
	if !strings.Contains(rendered, `description: "Implement deterministic handoff output."`) {
		t.Fatalf("expected quoted description, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, `- "Generated tasks.yaml always quotes descriptions."`) {
		t.Fatalf("expected quoted acceptance criteria, got:\n%s", rendered)
	}
}

func TestHandoffProjectPlan_GeneratesEpicPackagesAndManifest(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), validPlanMarkdown())

	now := time.Date(2026, 4, 1, 19, 0, 0, 0, time.UTC)
	result, err := plan.HandoffProjectPlan(dir, now)
	if err != nil {
		t.Fatalf("HandoffProjectPlan: %v", err)
	}

	if got, want := result.EpicCount, 2; got != want {
		t.Fatalf("EpicCount: got %d, want %d", got, want)
	}
	if !result.ManifestGenerated {
		t.Fatal("expected ManifestGenerated to be true")
	}

	paths := plan.NewEpicPackagePaths(dir, "EPIC-17")
	metadata, err := plan.LoadEpicMetadata(paths.MetadataPath)
	if err != nil {
		t.Fatalf("LoadEpicMetadata: %v", err)
	}
	if got, want := metadata.Status, types.EpicStatusPlanned; got != want {
		t.Fatalf("metadata status: got %q, want %q", got, want)
	}
	if got, want := metadata.CreatedAt, "2026-04-01T19:00:00Z"; got != want {
		t.Fatalf("metadata created_at: got %q, want %q", got, want)
	}

	tasksBytes := mustReadFile(t, paths.TasksPath)
	if !strings.Contains(tasksBytes, `description: "Implement deterministic handoff output."`) {
		t.Fatalf("expected quoted description in tasks.yaml, got:\n%s", tasksBytes)
	}

	manifest, err := types.LoadManifest(filepath.Join(dir, ".doug", "plan", "manifest.yaml"))
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if got, want := manifest.Project.Name, "Acme Planner"; got != want {
		t.Fatalf("manifest project.name: got %q, want %q", got, want)
	}
}

func TestHandoffProjectPlan_RefusesActiveOrCompletedOverwrite(t *testing.T) {
	statuses := []types.EpicLifecycleStatus{types.EpicStatusActive, types.EpicStatusCompleted}
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			dir := t.TempDir()
			testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), validPlanMarkdown())
			paths := plan.NewEpicPackagePaths(dir, "EPIC-17")
			testutil.WriteFile(t, paths.MetadataPath, ""+
				"epic_id: \"EPIC-17\"\n"+
				"status: \""+string(status)+"\"\n"+
				"created_at: \"2026-04-01T18:00:00Z\"\n"+
				"source_plan_path: \".doug/plan/PLAN.md\"\n")

			_, err := plan.HandoffProjectPlan(dir, time.Date(2026, 4, 1, 19, 0, 0, 0, time.UTC))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), `refusing to overwrite epic package "EPIC-17"`) {
				t.Fatalf("expected overwrite guard error, got: %v", err)
			}
		})
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(data)
}

func validPlanMarkdown() string {
	return "# Integrated Plan\n\n" +
		"This file can contain explanatory markdown before the deterministic payload.\n\n" +
		"## Handoff Data\n\n" +
		"```yaml\n" +
		"schema_version: 1\n" +
		"project:\n" +
		"  name: \"Acme Planner\"\n" +
		"  mode: \"greenfield\"\n" +
		"manifest:\n" +
		"  schema_version: 1\n" +
		"  project:\n" +
		"    name: \"Acme Planner\"\n" +
		"    mode: \"greenfield\"\n" +
		"  scaffold:\n" +
		"    language: \"typescript\"\n" +
		"    runtime: \"node\"\n" +
		"    framework: \"nextjs\"\n" +
		"    package_manager: \"pnpm\"\n" +
		"    build_system: \"npm-scripts\"\n" +
		"  dependencies:\n" +
		"    runtime:\n" +
		"      - \"next\"\n" +
		"      - \"react\"\n" +
		"    development:\n" +
		"      - \"typescript\"\n" +
		"      - \"eslint\"\n" +
		"  constraints:\n" +
		"    - \"Deploy on Vercel\"\n" +
		"epics:\n" +
		"  - id: \"EPIC-17\"\n" +
		"    name: \"Planning Lifecycle\"\n" +
		"    prd: |\n" +
		"      # PRD\n\n" +
		"      Deterministically generate backlog packages from PLAN.md.\n" +
		"    tasks:\n" +
		"      - id: \"EPIC-17-003\"\n" +
		"        description: \"Implement deterministic handoff output.\"\n" +
		"        acceptance_criteria:\n" +
		"          - \"Generated tasks.yaml always quotes descriptions.\"\n" +
		"          - \"Generated tasks.yaml always quotes acceptance criteria.\"\n" +
		"  - id: \"EPIC-18\"\n" +
		"    name: \"Epic Checkout\"\n" +
		"    prd: |\n" +
		"      # PRD\n\n" +
		"      Promote planned epics into the active runtime workspace.\n" +
		"    tasks:\n" +
		"      - id: \"EPIC-18-001\"\n" +
		"        type: \"feature\"\n" +
		"        status: \"TODO\"\n" +
		"        description: \"Promote planned epics into root .doug.\"\n" +
		"        acceptance_criteria:\n" +
		"          - \"The selected backlog epic becomes ACTIVE before runtime execution.\"\n" +
		"```\n"
}
