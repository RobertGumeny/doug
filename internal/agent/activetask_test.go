package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeFile is a test helper that creates a file (and its parent directories)
// with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// makeSkillsConfig writes a skills-config.yaml to configPath.
func makeSkillsConfig(t *testing.T, configPath string, mappings map[string]string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("skill_mappings:\n")
	for taskType, skillName := range mappings {
		sb.WriteString("  " + taskType + ": " + skillName + "\n")
	}
	writeFile(t, configPath, sb.String())
}

// ---------------------------------------------------------------------------
// GetSkillForTaskType tests
// ---------------------------------------------------------------------------

func TestGetSkillForTaskType(t *testing.T) {
	t.Run("reads skill name from config", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "skills-config.yaml")
		makeSkillsConfig(t, configPath, map[string]string{"feature": "my-feature-skill"})

		name, err := GetSkillForTaskType("feature", configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "my-feature-skill" {
			t.Errorf("expected %q, got %q", "my-feature-skill", name)
		}
	})

	t.Run("falls back to hardcoded name when config is missing", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "missing.yaml")

		name, err := GetSkillForTaskType("feature", configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "implement-feature" {
			t.Errorf("expected %q, got %q", "implement-feature", name)
		}
	})

	t.Run("falls back to hardcoded name when type not in config", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "skills-config.yaml")
		makeSkillsConfig(t, configPath, map[string]string{"feature": "my-feature"})

		name, err := GetSkillForTaskType("bugfix", configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "implement-bugfix" {
			t.Errorf("expected %q, got %q", "implement-bugfix", name)
		}
	})

	t.Run("returns error for unknown task type not in config and no hardcoded default", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "skills-config.yaml")
		makeSkillsConfig(t, configPath, map[string]string{"feature": "implement-feature"})

		_, err := GetSkillForTaskType("unknown-type", configPath)
		if err == nil {
			t.Error("expected error for unknown task type, got nil")
		}
		if !strings.Contains(err.Error(), "unknown task type") {
			t.Errorf("error should mention unknown task type, got: %v", err)
		}
	})

	t.Run("handles all four known task types with hardcoded fallback", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "missing.yaml")

		knownTypes := []string{
			string(types.TaskTypeFeature),
			string(types.TaskTypeBugfix),
			string(types.TaskTypeDocumentation),
			string(types.TaskTypeManualReview),
		}
		for _, taskType := range knownTypes {
			name, err := GetSkillForTaskType(taskType, configPath)
			if err != nil {
				t.Errorf("task type %q: unexpected error: %v", taskType, err)
				continue
			}
			if name == "" {
				t.Errorf("task type %q: expected non-empty skill name", taskType)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// WriteActiveTask tests
// ---------------------------------------------------------------------------

func TestWriteActiveTask(t *testing.T) {
	t.Run("writes ACTIVE_TASK.md to doug dir", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:          "EPIC-4-002",
			TaskType:        types.TaskTypeFeature,
			SessionFilePath: ".doug/logs/sessions/EPIC-4/session-EPIC-4-002_attempt-1.md",
			DougDir:         dougDir,
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
			".doug/logs/sessions/EPIC-4/session-EPIC-4-002_attempt-1.md",
			"**Session File**",
			"**Active Bug File**",
			"**Failure File**",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("expected %q in ACTIVE_TASK.md, got:\n%s", want, content)
			}
		}
	})

	t.Run("briefing header contains DougDir paths", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:          "EPIC-1-001",
			TaskType:        types.TaskTypeFeature,
			SessionFilePath: "session.md",
			DougDir:         dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		content := string(data)

		if !strings.Contains(content, filepath.Join(dougDir, "ACTIVE_BUG.md")) {
			t.Errorf("expected Active Bug File path in header, got:\n%s", content)
		}
		if !strings.Contains(content, filepath.Join(dougDir, "ACTIVE_FAILURE.md")) {
			t.Errorf("expected Failure File path in header, got:\n%s", content)
		}
	})

	t.Run("no skill content in output", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:          "EPIC-4-002",
			TaskType:        types.TaskTypeFeature,
			SessionFilePath: "session.md",
			DougDir:         dougDir,
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
		writeFile(t, filepath.Join(dougDir, "ACTIVE_TASK.md"), "old content")

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:          "EPIC-4-002",
			TaskType:        types.TaskTypeFeature,
			SessionFilePath: "session.md",
			DougDir:         dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		if strings.Contains(string(data), "old content") {
			t.Error("ACTIVE_TASK.md was not overwritten")
		}
	})

	t.Run("bugfix task includes bug context from ACTIVE_BUG.md", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		writeFile(t, filepath.Join(dougDir, "ACTIVE_BUG.md"), "## Bug Report\nnull pointer at line 42")

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:          "BUG-EPIC-4-001",
			TaskType:        types.TaskTypeBugfix,
			SessionFilePath: "session.md",
			DougDir:         dougDir,
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
			t.Error("expected ACTIVE_BUG.md content in Bug Context section")
		}
	})

	t.Run("bugfix task omits bug context section when ACTIVE_BUG.md is missing", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		// Do NOT write ACTIVE_BUG.md.

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:          "BUG-EPIC-4-001",
			TaskType:        types.TaskTypeBugfix,
			SessionFilePath: "session.md",
			DougDir:         dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dougDir, "ACTIVE_TASK.md"))
		if strings.Contains(string(data), "Bug Context") {
			t.Error("Bug Context section should be omitted when ACTIVE_BUG.md is missing")
		}
	})

	t.Run("feature task does not include bug context section", func(t *testing.T) {
		dir := t.TempDir()
		dougDir := filepath.Join(dir, ".doug")
		// Write ACTIVE_BUG.md — it should NOT appear for a feature task.
		writeFile(t, filepath.Join(dougDir, "ACTIVE_BUG.md"), "## Bug Report")

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:          "EPIC-4-002",
			TaskType:        types.TaskTypeFeature,
			SessionFilePath: "session.md",
			DougDir:         dougDir,
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

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:          "KB_UPDATE",
			TaskType:        types.TaskTypeDocumentation,
			SessionFilePath: "session.md",
			DougDir:         dougDir,
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

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:             "EPIC-1-001",
			TaskType:           types.TaskTypeFeature,
			SessionFilePath:    "session.md",
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

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:          "EPIC-1-001",
			TaskType:        types.TaskTypeFeature,
			SessionFilePath: "session.md",
			DougDir:         dougDir,
			Attempts:        3,
			MaxRetries:      5,
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

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:             "EPIC-1-001",
			TaskType:           types.TaskTypeFeature,
			SessionFilePath:    "session.md",
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
		writeFile(t, filepath.Join(dougDir, "ACTIVE_BUG.md"), "## Bug\nnull pointer")

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:             "BUG-EPIC-1-001",
			TaskType:           types.TaskTypeBugfix,
			SessionFilePath:    "session.md",
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

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:          "EPIC-4-002",
			TaskType:        types.TaskTypeFeature,
			SessionFilePath: "session.md",
			DougDir:         dougDir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, statErr := os.Stat(filepath.Join(dougDir, "ACTIVE_TASK.md")); statErr != nil {
			t.Errorf("ACTIVE_TASK.md not found: %v", statErr)
		}
	})
}
