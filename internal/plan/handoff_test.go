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
		{"first epic id", "epic.id", "EPIC-1"},
		{"epic name similar to example", "epic.name", "Example-Driven Feature"},
		{"first task id", "task.id", "EPIC-1-001"},
		{"prd without placeholder sentence", "epic.prd", "# PRD\n\nThis epic covers the checkout flow.\n"},
		{"task description not a placeholder", "task.description", "This task describes the implementation."},
		{"criterion not a placeholder", "acceptance_criteria", "The first criterion passes validation."},
	}
	for _, v := range variations {
		t.Run(v.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".doug", "plan", "PLAN.md")
			var yaml string
			switch v.field {
			case "project.name":
				yaml = placeholderHandoffYAML(v.value, "Real Epic", "EPIC-99", "Real prd content.", "EPIC-99-001", "Real task description.", []string{"Real criterion."})
			case "epic.id":
				yaml = placeholderHandoffYAML("Real Project", "Real Epic", v.value, "Real prd content.", "EPIC-99-001", "Real task description.", []string{"Real criterion."})
			case "epic.name":
				yaml = placeholderHandoffYAML("Real Project", v.value, "EPIC-99", "Real prd content.", "EPIC-99-001", "Real task description.", []string{"Real criterion."})
			case "task.id":
				yaml = placeholderHandoffYAML("Real Project", "Real Epic", "EPIC-99", "Real prd content.", v.value, "Real task description.", []string{"Real criterion."})
			case "epic.prd":
				yaml = placeholderHandoffYAML("Real Project", "Real Epic", "EPIC-99", v.value, "EPIC-99-001", "Real task description.", []string{"Real criterion."})
			case "task.description":
				yaml = placeholderHandoffYAML("Real Project", "Real Epic", "EPIC-99", "Real prd content.", "EPIC-99-001", v.value, []string{"Real criterion."})
			case "acceptance_criteria":
				yaml = placeholderHandoffYAML("Real Project", "Real Epic", "EPIC-99", "Real prd content.", "EPIC-99-001", "Real task description.", []string{v.value})
			default:
				t.Fatalf("unsupported field %q", v.field)
			}
			testutil.WriteFile(t, path, wrapHandoffYAML(yaml))

			if _, err := plan.ParseHandoffDocument(path); err != nil {
				t.Fatalf("%s rejected: %v", v.field, err)
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
	originalPlan := validPlanMarkdown()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), originalPlan)

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
	if got, want := result.ArchivedPlanPath, ".doug/plan/history/PLAN-20260401T190000.000000000Z.md"; got != want {
		t.Fatalf("ArchivedPlanPath: got %q, want %q", got, want)
	}

	paths := plan.NewEpicPackagePaths(dir, "EPIC-1")
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

	archivedPlan := mustReadFile(t, filepath.Join(dir, result.ArchivedPlanPath))
	if archivedPlan != originalPlan {
		t.Fatalf("archived PLAN.md mismatch:\ngot:\n%s\nwant:\n%s", archivedPlan, originalPlan)
	}

	activePlan := mustReadFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"))
	for _, want := range []string{
		"# Planning Session",
		"**Last handoff:**",
		"2026-04-01T19:00:00Z",
		"EPIC-1, EPIC-2",
		"Start the next cycle fresh",
		`  name: "My Project"`,
	} {
		if !strings.Contains(activePlan, want) {
			t.Fatalf("expected %q in reseeded PLAN.md, got:\n%s", want, activePlan)
		}
	}
	for _, unwanted := range []string{
		`  name: "Acme Planner"`,
		`  - id: "EPIC-17"`,
		`  - id: "EPIC-18"`,
		"Determinstically generate backlog packages from PLAN.md.",
	} {
		if strings.Contains(activePlan, unwanted) {
			t.Fatalf("did not expect %q in reseeded PLAN.md, got:\n%s", unwanted, activePlan)
		}
	}
}

func TestHandoffProjectPlan_ArchivesAndReseedsWorkbook(t *testing.T) {
	dir := t.TempDir()
	originalPlan := "# Project Plan\n\n" +
		"## Handoff Data\n\n" +
		"```yaml\n" +
		"schema_version: 1\n" +
		"project:\n" +
		"  name: \"My Service\"\n" +
		"  mode: \"brownfield\"\n" +
		"epics:\n" +
		"  - id: \"EPIC-5\"\n" +
		"    name: \"Auth Hardening\"\n" +
		"    prd: |\n" +
		"      # PRD\n\n" +
		"      Harden the authentication flow.\n" +
		"    tasks:\n" +
		"      - id: \"EPIC-5-001\"\n" +
		"        description: \"Rotate secrets on deploy.\"\n" +
		"        acceptance_criteria:\n" +
		"          - \"Secrets rotate without downtime.\"\n" +
		"```\n"
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), originalPlan)

	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	result, err := plan.HandoffProjectPlan(dir, now)
	if err != nil {
		t.Fatalf("HandoffProjectPlan: %v", err)
	}

	// original workbook must be archived verbatim
	archived := mustReadFile(t, filepath.Join(dir, result.ArchivedPlanPath))
	if archived != originalPlan {
		t.Fatalf("archived PLAN.md differs from original:\ngot:\n%s\nwant:\n%s", archived, originalPlan)
	}

	// active PLAN.md must be reseeded, not contain stale epic content
	active := mustReadFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"))
	if strings.Contains(active, "Auth Hardening") {
		t.Fatalf("reseeded PLAN.md must not contain handed-off epic name, got:\n%s", active)
	}
	if strings.Contains(active, "EPIC-5-001") {
		t.Fatalf("reseeded PLAN.md must not contain handed-off task id, got:\n%s", active)
	}
	if strings.Contains(active, "Harden the authentication flow") {
		t.Fatalf("reseeded PLAN.md must not contain handed-off epic prd content, got:\n%s", active)
	}
	for _, want := range []string{
		"# Planning Session",
		"**Last handoff:**",
		"2026-04-20T12:00:00Z",
		"EPIC-1",
		"Start the next cycle fresh",
	} {
		if !strings.Contains(active, want) {
			t.Fatalf("expected %q in reseeded PLAN.md, got:\n%s", want, active)
		}
	}
}

func TestHandoffProjectPlan_AllowsFirstEpicIdentifiers(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), wrapHandoffYAML(placeholderHandoffYAML(
		"Real Project",
		"Real Epic",
		"EPIC-1",
		"# PRD\n\nReal prd content.\n",
		"EPIC-1-001",
		"Real task description.",
		[]string{"Real criterion."},
	)))

	if _, err := plan.HandoffProjectPlan(dir, time.Date(2026, 4, 1, 19, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("HandoffProjectPlan: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".doug", "plan", "epics", "EPIC-1", "tasks.yaml")); err != nil {
		t.Fatalf("expected EPIC-1 tasks.yaml, stat err: %v", err)
	}
}

func TestHandoffProjectPlan_PreservesActiveOrCompletedEpics(t *testing.T) {
	// With concrete allocation, an existing ACTIVE/COMPLETED epic raises the
	// allocation floor instead of being overwritten. Submitted epics receive the
	// next free numbers and the existing package is left untouched.
	statuses := []types.EpicLifecycleStatus{types.EpicStatusActive, types.EpicStatusCompleted}
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			dir := t.TempDir()
			testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), validPlanMarkdown())
			paths := plan.NewEpicPackagePaths(dir, "EPIC-17")
			existingMetadata := "" +
				"epic_id: \"EPIC-17\"\n" +
				"status: \"" + string(status) + "\"\n" +
				"created_at: \"2026-04-01T18:00:00Z\"\n" +
				"source_plan_path: \".doug/plan/PLAN.md\"\n"
			testutil.WriteFile(t, paths.MetadataPath, existingMetadata)

			result, err := plan.HandoffProjectPlan(dir, time.Date(2026, 4, 1, 19, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatalf("HandoffProjectPlan: %v", err)
			}
			if got, want := result.EpicCount, 2; got != want {
				t.Fatalf("EpicCount: got %d, want %d", got, want)
			}

			// Existing EPIC-17 package must be untouched (no overwrite).
			if got := mustReadFile(t, paths.MetadataPath); got != existingMetadata {
				t.Fatalf("existing epic metadata was modified:\ngot:\n%s\nwant:\n%s", got, existingMetadata)
			}

			// Submitted epics allocate above the existing maximum (EPIC-18, EPIC-19).
			for _, id := range []string{"EPIC-18", "EPIC-19"} {
				if _, err := os.Stat(plan.NewEpicPackagePaths(dir, id).TasksPath); err != nil {
					t.Fatalf("expected %s tasks.yaml, stat err: %v", id, err)
				}
			}
		})
	}
}

func TestHandoffProjectPlan_AllocatesNextEpicNumber(t *testing.T) {
	// Existing metadata through EPIC-48 (with a gap) must allocate EPIC-49 for a
	// single submitted epic regardless of whether the submitted ID was a
	// placeholder token or a concrete number, and must not fill the gap.
	for _, submitted := range []string{"EPIC-<X>", "EPIC-200"} {
		t.Run(submitted, func(t *testing.T) {
			dir := t.TempDir()
			seedExistingEpic(t, dir, "EPIC-3")  // a populated low number
			seedExistingEpic(t, dir, "EPIC-48") // the maximum; EPIC-4..47 are gaps

			taskID := submitted + "-001"
			testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), wrapHandoffYAML(placeholderHandoffYAML(
				"Real Project",
				"Real Epic",
				submitted,
				"# PRD\n\nWork tracked under "+submitted+" starting with "+taskID+".\n",
				taskID,
				"Implement "+taskID+" behavior.",
				[]string{"Task " + taskID + " passes verification."},
			)))

			if _, err := plan.HandoffProjectPlan(dir, time.Date(2026, 4, 1, 19, 0, 0, 0, time.UTC)); err != nil {
				t.Fatalf("HandoffProjectPlan: %v", err)
			}

			paths := plan.NewEpicPackagePaths(dir, "EPIC-49")
			tasks := mustReadFile(t, paths.TasksPath)
			if !strings.Contains(tasks, `id: "EPIC-49"`) {
				t.Fatalf("expected allocated epic id EPIC-49 in tasks.yaml, got:\n%s", tasks)
			}
			if !strings.Contains(tasks, `id: "EPIC-49-001"`) {
				t.Fatalf("expected task id prefixed EPIC-49-, got:\n%s", tasks)
			}
			if strings.Contains(tasks, submitted) {
				t.Fatalf("submitted identifier %q leaked into tasks.yaml:\n%s", submitted, tasks)
			}
			if !strings.Contains(tasks, "Implement EPIC-49-001 behavior.") {
				t.Fatalf("expected rewritten task reference in description, got:\n%s", tasks)
			}
			if !strings.Contains(tasks, "Task EPIC-49-001 passes verification.") {
				t.Fatalf("expected rewritten task reference in acceptance criteria, got:\n%s", tasks)
			}

			prd := mustReadFile(t, paths.PRDPath)
			if !strings.Contains(prd, "Work tracked under EPIC-49 starting with EPIC-49-001.") {
				t.Fatalf("expected rewritten epic/task references in PRD, got:\n%s", prd)
			}

			// Gap-filling is forbidden: no EPIC-4 package should have been created.
			if _, err := os.Stat(plan.NewEpicPackagePaths(dir, "EPIC-4").EpicDir); !os.IsNotExist(err) {
				t.Fatalf("handoff must not fill numeric gaps (EPIC-4), stat err: %v", err)
			}
		})
	}
}

func TestHandoffProjectPlan_AllocatesConsecutiveIDsInDocumentOrder(t *testing.T) {
	dir := t.TempDir()
	seedExistingEpic(t, dir, "EPIC-10")

	planMarkdown := "# Plan\n\n## Handoff Data\n\n```yaml\n" +
		"schema_version: 1\n" +
		"project:\n" +
		"  name: \"Real Project\"\n" +
		"  mode: \"brownfield\"\n" +
		"epics:\n" +
		"  - id: \"EPIC-<A>\"\n" +
		"    name: \"First Epic\"\n" +
		"    prd: |\n" +
		"      # PRD\n\n" +
		"      First epic body.\n" +
		"    tasks:\n" +
		"      - id: \"EPIC-<A>-001\"\n" +
		"        description: \"First task.\"\n" +
		"        acceptance_criteria:\n" +
		"          - \"First done.\"\n" +
		"  - id: \"EPIC-99\"\n" +
		"    name: \"Second Epic\"\n" +
		"    prd: |\n" +
		"      # PRD\n\n" +
		"      Second epic body.\n" +
		"    tasks:\n" +
		"      - id: \"EPIC-99-001\"\n" +
		"        description: \"Second task.\"\n" +
		"        acceptance_criteria:\n" +
		"          - \"Second done.\"\n" +
		"```\n"
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), planMarkdown)

	if _, err := plan.HandoffProjectPlan(dir, time.Date(2026, 4, 1, 19, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("HandoffProjectPlan: %v", err)
	}

	first := mustReadFile(t, plan.NewEpicPackagePaths(dir, "EPIC-11").TasksPath)
	if !strings.Contains(first, `id: "EPIC-11-001"`) {
		t.Fatalf("expected first submitted epic to allocate EPIC-11, got:\n%s", first)
	}
	second := mustReadFile(t, plan.NewEpicPackagePaths(dir, "EPIC-12").TasksPath)
	if !strings.Contains(second, `id: "EPIC-12-001"`) {
		t.Fatalf("expected second submitted epic to allocate EPIC-12, got:\n%s", second)
	}
}

func seedExistingEpic(t *testing.T, dir, epicID string) {
	t.Helper()
	paths := plan.NewEpicPackagePaths(dir, epicID)
	testutil.WriteFile(t, paths.MetadataPath, ""+
		"epic_id: \""+epicID+"\"\n"+
		"status: \"PLANNED\"\n"+
		"created_at: \"2026-04-01T18:00:00Z\"\n"+
		"source_plan_path: \".doug/plan/PLAN.md\"\n")
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
