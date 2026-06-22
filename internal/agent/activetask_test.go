package agent

import (
	"github.com/robertgumeny/doug/internal/testutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/types"
)

// writeActiveTask is a test helper that calls WriteActiveTask with a discard logger.
func writeActiveTask(config ActiveTaskConfig) error {
	return WriteActiveTask(config, log.Discard())
}

// ---------------------------------------------------------------------------
// DefaultSkillName tests
// ---------------------------------------------------------------------------

func TestDefaultSkillName(t *testing.T) {
	knownTypes := []struct {
		taskType string
		want     string
	}{
		{string(types.TaskTypeFeature), "implement-feature"},
		{string(types.TaskTypeBugfix), "implement-bugfix"},
		{string(types.TaskTypeDocumentation), "implement-documentation"},
		{string(types.TaskTypeScaffold), "scaffold"},
		{string(types.TaskTypePlan), "plan"},
		{string(types.TaskTypeResearch), "research"},
	}
	for _, tc := range knownTypes {
		name, ok := DefaultSkillName(tc.taskType)
		if !ok {
			t.Errorf("DefaultSkillName(%q): expected ok=true", tc.taskType)
		}
		if name != tc.want {
			t.Errorf("DefaultSkillName(%q) = %q, want %q", tc.taskType, name, tc.want)
		}
	}

	_, ok := DefaultSkillName("unknown-type")
	if ok {
		t.Error("DefaultSkillName(\"unknown-type\"): expected ok=false")
	}
}

// ---------------------------------------------------------------------------
// WriteActiveTask tests
// ---------------------------------------------------------------------------

func TestWriteActiveTask(t *testing.T) {
	t.Run("writes ACTIVE_TASK.md to doug dir", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "EPIC-4-002",
			TaskType: types.TaskTypeFeature,
			DougDir:  dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		outPath := filepath.Join(dougDir, "ACTIVE_TASK.md")
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("ACTIVE_TASK.md not found: %v", err)
		}
		content := string(data)

		for _, want := range []string{
			"EPIC-4-002",
			"feature",
			"Blocking bug",
			"PRD",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("expected %q in ACTIVE_TASK.md, got:\n%s", want, content)
			}
		}
		if strings.Contains(content, "**Session File**") {
			t.Error("**Session File** should not appear in ACTIVE_TASK.md")
		}
	})

	t.Run("no lifecycle section in ordinary task brief", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "EPIC-4-002",
			TaskType: types.TaskTypeFeature,
			DougDir:  dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)
		// Lifecycle prose is redundant with AGENTS.md; it must not be repeated here.
		if strings.Contains(content, "## Doug Lifecycle") {
			t.Error("Doug Lifecycle section must not appear in ordinary task briefs (redundant with AGENTS.md)")
		}
	})

	t.Run("briefing header contains DougDir paths", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "EPIC-1-001",
			TaskType: types.TaskTypeFeature,
			DougDir:  dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		if strings.Contains(content, "ACTIVE_BUG.md") {
			t.Errorf("ACTIVE_BUG.md must not appear in feature task brief, got:\n%s", content)
		}
		if strings.Contains(content, "ACTIVE_FAILURE.md") {
			t.Errorf("ACTIVE_FAILURE.md must not appear in task brief (agent uses result block for FAILURE), got:\n%s", content)
		}
		// PRD reference must be repo-relative when ProjectRoot is provided.
		if !strings.Contains(content, ".doug/PRD.md") {
			t.Errorf("expected repo-relative PRD path in header, got:\n%s", content)
		}
		if strings.Contains(content, filepath.Join(dougDir, "PRD.md")) {
			t.Errorf("PRD path must be repo-relative, not absolute; got:\n%s", content)
		}
		if !strings.Contains(content, "Blocking bug") {
			t.Errorf("expected blocking bug instruction in feature task brief, got:\n%s", content)
		}
	})

	t.Run("no skill content in output", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "EPIC-4-002",
			TaskType: types.TaskTypeFeature,
			DougDir:  dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		// No skill content should be embedded.
		if strings.Contains(content, "# Skill to Use") {
			t.Error("skill content should not be embedded in ACTIVE_TASK.md")
		}
	})

	t.Run("overwrites existing ACTIVE_TASK.md", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		testutil.WriteFile(t, filepath.Join(dougDir, "ACTIVE_TASK.md"), "old content")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "EPIC-4-002",
			TaskType: types.TaskTypeFeature,
			DougDir:  dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		if strings.Contains(string(data), "old content") {
			t.Error("ACTIVE_TASK.md was not overwritten")
		}
	})

	t.Run("bugfix task includes bug context from payload fields", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:        "BUG-EPIC-4-001",
			TaskType:      types.TaskTypeBugfix,
			DougDir:       dougDir,
			BugID:         "BUG-EPIC-4-001",
			BugSeverity:   "high",
			BugSourceTask: "EPIC-4-001",
			BugBody:       "## Summary\nnull pointer at line 42",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		if !strings.Contains(content, "Bug Context") {
			t.Error("expected Bug Context section in bugfix ACTIVE_TASK.md")
		}
		if !strings.Contains(content, "null pointer at line 42") {
			t.Error("expected bug body content in Bug Context section")
		}
		if !strings.Contains(content, "BUG-EPIC-4-001") {
			t.Error("expected bug ID in Bug Context section")
		}
		if !strings.Contains(content, "EPIC-4-001") {
			t.Error("expected source task in Bug Context section")
		}
		// Must not reference ACTIVE_BUG.md
		if strings.Contains(content, "ACTIVE_BUG.md") {
			t.Error("bugfix brief must not reference ACTIVE_BUG.md")
		}
	})

	t.Run("bugfix task omits bug context section when payload is empty", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		// No bug payload fields — BugID is empty.

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "BUG-EPIC-4-001",
			TaskType: types.TaskTypeBugfix,
			DougDir:  dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		if strings.Contains(string(data), "Bug Context") {
			t.Error("Bug Context section should be omitted when BugID is empty")
		}
	})

	t.Run("feature task does not include bug context section", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "EPIC-4-002",
			TaskType: types.TaskTypeFeature,
			DougDir:  dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		if strings.Contains(string(data), "Bug Context") {
			t.Error("Bug Context section should not appear for non-bugfix tasks")
		}
	})

	t.Run("documentation task type is preserved correctly", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "KB_UPDATE",
			TaskType: types.TaskTypeDocumentation,
			DougDir:  dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		if !strings.Contains(content, "documentation") {
			t.Errorf("expected task type 'documentation' in ACTIVE_TASK.md, got:\n%s", content)
		}
	})

	t.Run("description and acceptance criteria appear in output when provided", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:             "EPIC-1-001",
			TaskType:           types.TaskTypeFeature,
			DougDir:            dougDir,
			Description:        "Implement the first feature.",
			AcceptanceCriteria: []string{"Tests pass", "Build succeeds"},
			Attempts:           1,
			MaxRetries:         5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		if !strings.Contains(content, "Implement the first feature.") {
			t.Errorf("expected description in ACTIVE_TASK.md, got:\n%s", content)
		}
		if !strings.Contains(content, "Tests pass") {
			t.Errorf("expected first criterion in ACTIVE_TASK.md, got:\n%s", content)
		}
		if !strings.Contains(content, "Build succeeds") {
			t.Errorf("expected second criterion in ACTIVE_TASK.md, got:\n%s", content)
		}
		if !strings.Contains(content, "**Acceptance Criteria**") {
			t.Errorf("expected Acceptance Criteria header in ACTIVE_TASK.md, got:\n%s", content)
		}
	})

	t.Run("attempt and max_retries appear in output", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:     "EPIC-1-001",
			TaskType:   types.TaskTypeFeature,
			DougDir:    dougDir,
			Attempts:   3,
			MaxRetries: 5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		if !strings.Contains(content, "3 of 5") {
			t.Errorf("expected '3 of 5' in ACTIVE_TASK.md, got:\n%s", content)
		}
	})

	t.Run("empty description and criteria handled gracefully", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:             "EPIC-1-001",
			TaskType:           types.TaskTypeFeature,
			DougDir:            dougDir,
			Description:        "",
			AcceptanceCriteria: nil,
			Attempts:           1,
			MaxRetries:         5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		if strings.Contains(content, "**Description**") {
			t.Error("empty description should not emit a Description line")
		}
		if strings.Contains(content, "**Acceptance Criteria**") {
			t.Error("empty criteria should not emit an Acceptance Criteria section")
		}
	})

	t.Run("synthetic task (empty description/criteria) produces valid output", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:             "BUG-EPIC-1-001",
			TaskType:           types.TaskTypeBugfix,
			DougDir:            dougDir,
			Description:        "",
			AcceptanceCriteria: nil,
			Attempts:           1,
			MaxRetries:         5,
			BugID:              "BUG-EPIC-1-001",
			BugSeverity:        "high",
			BugSourceTask:      "EPIC-1-001",
			BugBody:            "## Summary\nnull pointer",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		if !strings.Contains(content, "BUG-EPIC-1-001") {
			t.Errorf("expected task ID in output, got:\n%s", content)
		}
		if !strings.Contains(content, "1 of 5") {
			t.Errorf("expected attempt info in output, got:\n%s", content)
		}
		if strings.Contains(content, "**Acceptance Criteria**") {
			t.Error("synthetic task should not emit Acceptance Criteria section")
		}
	})

	t.Run("creates .doug directory if it does not exist", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, "nested", ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "EPIC-4-002",
			TaskType: types.TaskTypeFeature,
			DougDir:  dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, statErr := os.Stat(filepath.Join(dougDir, "ACTIVE_TASK.md")); statErr != nil {
			t.Errorf("ACTIVE_TASK.md not found: %v", statErr)
		}
	})

	t.Run("result block section is appended at the bottom", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "EPIC-1-001",
			TaskType: types.TaskTypeFeature,
			DougDir:  dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		for _, want := range []string{
			"## Result",
			"Set `outcome` to one of: `SUCCESS`, `FAILURE`, `BUG`, `EPIC_COMPLETE`.",
			`outcome: ""`,
			`changelog_entry: ""`,
			"dependencies_added: []",
			"bugs: []",
			// Feature tasks support both blocking and non-blocking bug reporting.
			"severity: blocking",
			"severity: non-blocking",
			"## Summary",
			"## Files Changed",
			"## Key Decisions",
			"## Verification",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("expected %q in ACTIVE_TASK.md result block, got:\n%s", want, content)
			}
		}

		// Result block must appear after the task metadata.
		resultIdx := strings.Index(content, "\n## Result\n")
		taskIDIdx := strings.Index(content, "EPIC-1-001")
		if resultIdx < taskIDIdx {
			t.Error("## Result section should appear after task metadata")
		}
	})

	t.Run("context sections are appended as structured task context", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "SCAFFOLD",
			TaskType: types.TaskTypeScaffold,
			DougDir:  dougDir,
			ContextSections: []ActiveTaskSection{
				{
					Heading: "Manifest Context",
					Body:    "```yaml\nconstraints:\n  - Deploy on Vercel\n```",
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		if !strings.Contains(content, "## Manifest Context") {
			t.Errorf("expected manifest context heading in ACTIVE_TASK.md, got:\n%s", content)
		}
		if !strings.Contains(content, "Deploy on Vercel") {
			t.Errorf("expected manifest body in ACTIVE_TASK.md, got:\n%s", content)
		}
	})

	t.Run("blocking bug instruction states testable rule for when to report BUG", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "EPIC-5-001",
			TaskType: types.TaskTypeFeature,
			DougDir:  dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		for _, want := range []string{
			"Blocking bug",
			"acceptance criteria",
			"severity: blocking",
			"severity: non-blocking",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("expected %q in feature task brief blocking bug instruction, got:\n%s", want, content)
			}
		}
	})

	t.Run("scaffold and research briefs omit blocking bug guidance", func(t *testing.T) {
		for _, tt2 := range []struct {
			name     string
			taskType types.TaskType
			taskID   string
		}{
			{"scaffold", types.TaskTypeScaffold, "SCAFFOLD"},
			{"research", types.TaskTypeResearch, "RESEARCH-001"},
			{"plan", types.TaskTypePlan, "PLAN-001"},
		} {
			tt2 := tt2
			t.Run(tt2.name, func(t *testing.T) {
				dir := t.TempDir()
				dougDir := filepath.Join(dir, ".doug")
				err := writeActiveTask(ActiveTaskConfig{
					TaskID:   tt2.taskID,
					TaskType: tt2.taskType,
					DougDir:  dougDir,
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
				content := string(data)
				if strings.Contains(content, "Blocking bug") {
					t.Errorf("%s brief must not include blocking bug instruction, got:\n%s", tt2.name, content)
				}
				if strings.Contains(content, "death spiral") {
					t.Errorf("%s brief must not mention death spiral, got:\n%s", tt2.name, content)
				}
				// BUG must not be listed as a valid outcome
				if strings.Contains(content, "outcome: BUG") {
					t.Errorf("%s brief must not list BUG as a valid outcome, got:\n%s", tt2.name, content)
				}
				// non-blocking bugs are still recordable for all task types
				if !strings.Contains(content, "non-blocking") {
					t.Errorf("%s brief must still document non-blocking bug field, got:\n%s", tt2.name, content)
				}
			})
		}
	})

	t.Run("PRD reference is repo-relative when ProjectRoot is provided", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		err := writeActiveTask(ActiveTaskConfig{
			TaskID:      "EPIC-5-002",
			TaskType:    types.TaskTypeFeature,
			DougDir:     dougDir,
			ProjectRoot: dir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)
		if !strings.Contains(content, ".doug/PRD.md") {
			t.Errorf("expected repo-relative .doug/PRD.md in brief, got:\n%s", content)
		}
		if strings.Contains(content, filepath.Join(dougDir, "PRD.md")) {
			t.Errorf("PRD path must not be absolute, got:\n%s", content)
		}
	})

	t.Run("PRD reference falls back to .doug/PRD.md when ProjectRoot is empty", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		err := writeActiveTask(ActiveTaskConfig{
			TaskID:   "EPIC-5-002",
			TaskType: types.TaskTypeFeature,
			DougDir:  dougDir,
			// ProjectRoot intentionally omitted.
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)
		if !strings.Contains(content, ".doug/PRD.md") {
			t.Errorf("expected .doug/PRD.md fallback in brief, got:\n%s", content)
		}
	})

	t.Run("bugfix result block omits BUG outcome option", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		err := writeActiveTask(ActiveTaskConfig{
			TaskID:        "BUG-EPIC-5-001",
			TaskType:      types.TaskTypeBugfix,
			DougDir:       dougDir,
			BugID:         "BUG-EPIC-5-001",
			BugSeverity:   "high",
			BugSourceTask: "EPIC-5-001",
			BugBody:       "## Summary\nnil map write",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)
		// Result block must list outcomes that exclude BUG.
		if !strings.Contains(content, "Set `outcome` to one of: `SUCCESS`, `FAILURE`, `EPIC_COMPLETE`.") {
			t.Errorf("bugfix result block must not include BUG outcome, got:\n%s", content)
		}
		if strings.Contains(content, "`BUG`") && strings.Contains(content, "Set `outcome`") {
			// Ensure BUG doesn't appear in the outcome list line.
			for _, line := range strings.Split(content, "\n") {
				if strings.HasPrefix(line, "Set `outcome`") && strings.Contains(line, "BUG") {
					t.Errorf("BUG must not appear in bugfix outcome list line: %s", line)
				}
			}
		}
	})

	t.Run("bugfix brief disallows BUG outcome and explains death spiral risk", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:        "BUG-EPIC-5-001",
			TaskType:      types.TaskTypeBugfix,
			DougDir:       dougDir,
			BugID:         "BUG-EPIC-5-001",
			BugSeverity:   "high",
			BugSourceTask: "EPIC-5-001",
			BugBody:       "## Summary\nnil map write",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		// Must not offer BUG as a valid outcome instruction (standard blocking bug guidance).
		if strings.Contains(content, "Blocking bug:") {
			t.Error("bugfix brief must not include the standard blocking bug instruction")
		}
		// Must explicitly warn about the death spiral.
		if !strings.Contains(content, "death spiral") {
			t.Errorf("bugfix brief must mention death spiral risk, got:\n%s", content)
		}
		// Must still document non-blocking path.
		if !strings.Contains(content, "non-blocking") {
			t.Errorf("bugfix brief must document non-blocking bug reporting, got:\n%s", content)
		}
		// ACTIVE_FAILURE.md must not be referenced.
		if strings.Contains(content, "ACTIVE_FAILURE.md") {
			t.Error("bugfix brief must not reference ACTIVE_FAILURE.md")
		}
	})
}

// ---------------------------------------------------------------------------
// WriteActiveTask build system briefing tests
// ---------------------------------------------------------------------------

func TestWriteActiveTask_BuildSystemBriefing(t *testing.T) {
	t.Run("go build system injects briefing section", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:      "EPIC-1-001",
			TaskType:    types.TaskTypeFeature,
			DougDir:     dougDir,
			BuildSystem: "go",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		for _, want := range []string{
			"## Build System",
			"**System**: go",
			"go mod download",
			"go build ./...",
			"go test ./...",
			"go mod tidy",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("expected %q in ACTIVE_TASK.md build system section; got:\n%s", want, content)
			}
		}
	})

	t.Run("npm build system injects npm briefing", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := writeActiveTask(ActiveTaskConfig{
			TaskID:      "EPIC-1-001",
			TaskType:    types.TaskTypeFeature,
			DougDir:     dougDir,
			BuildSystem: "npm",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		if !strings.Contains(content, "## Build System") {
			t.Error("expected ## Build System section")
		}
		if !strings.Contains(content, "npm ci") {
			t.Errorf("expected npm install cmd in briefing; got:\n%s", content)
		}
	})
}

func TestWriteActiveTask_UnknownBuildSystem(t *testing.T) {
	dir := t.TempDir()
	dougDir := filepath.Join(dir, ".doug")

	// Should not panic; section simply omitted.
	err := writeActiveTask(ActiveTaskConfig{
		TaskID:      "EPIC-1-001",
		TaskType:    types.TaskTypeFeature,
		DougDir:     dougDir,
		BuildSystem: "rust",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
	if strings.Contains(string(data), "## Build System") {
		t.Error("unknown build system should not inject ## Build System section")
	}
}

func TestWriteActiveTask_EmptyBuildSystem(t *testing.T) {
	dir := t.TempDir()
	dougDir := filepath.Join(dir, ".doug")

	err := writeActiveTask(ActiveTaskConfig{
		TaskID:      "EPIC-1-001",
		TaskType:    types.TaskTypeFeature,
		DougDir:     dougDir,
		BuildSystem: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
	if strings.Contains(string(data), "## Build System") {
		t.Error("empty build system should not inject ## Build System section")
	}
}
