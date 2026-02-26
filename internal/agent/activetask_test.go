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

// makeSkillsConfig writes a skills-config.yaml to configPath and creates
// SKILL.md for each entry in mappings (skillName → skillContent).
func makeSkillsConfig(t *testing.T, configPath string, mappings map[string]string, skillContents map[string]string) {
	t.Helper()
	configDir := filepath.Dir(configPath)

	// Build YAML content.
	var sb strings.Builder
	sb.WriteString("skill_mappings:\n")
	for taskType, skillName := range mappings {
		sb.WriteString("  " + taskType + ": " + skillName + "\n")
	}
	writeFile(t, configPath, sb.String())

	// Write SKILL.md files.
	for skillName, content := range skillContents {
		skillFile := filepath.Join(configDir, "skills", skillName, "SKILL.md")
		writeFile(t, skillFile, content)
	}
}

// ---------------------------------------------------------------------------
// GetSkillForTaskType tests
// ---------------------------------------------------------------------------

func TestGetSkillForTaskType(t *testing.T) {
	t.Run("reads skill content from SKILL.md via config", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"feature": "my-feature-skill"},
			map[string]string{"my-feature-skill": "# My Feature Skill\nDo the thing."},
		)

		content, err := GetSkillForTaskType("feature", configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(content, "# My Feature Skill") {
			t.Errorf("expected skill content, got: %s", content)
		}
	})

	t.Run("falls back to hardcoded skill names when config is missing", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "missing-skills-config.yaml")

		// Write the SKILL.md at the default location derived from configPath.
		skillFile := filepath.Join(dir, "skills", "implement-feature", "SKILL.md")
		writeFile(t, skillFile, "# Hardcoded Default Feature Skill")

		content, err := GetSkillForTaskType("feature", configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(content, "# Hardcoded Default Feature Skill") {
			t.Errorf("expected default skill content, got: %s", content)
		}
	})

	t.Run("returns hardcoded fallback content when SKILL.md file is missing", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "skills-config.yaml")
		// Config maps feature → some-skill, but SKILL.md does not exist.
		writeFile(t, configPath, "skill_mappings:\n  feature: some-skill\n")

		content, err := GetSkillForTaskType("feature", configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should get the hardcoded fallback for "feature".
		if content == "" {
			t.Error("expected non-empty fallback content")
		}
		if !strings.Contains(content, "Feature Implementation") {
			t.Errorf("expected fallback feature content, got: %s", content)
		}
	})

	t.Run("returns error for unknown task type not in config and no hardcoded default", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "skills-config.yaml")
		writeFile(t, configPath, "skill_mappings:\n  feature: implement-feature\n")

		_, err := GetSkillForTaskType("unknown-type", configPath)
		if err == nil {
			t.Error("expected error for unknown task type, got nil")
		}
		if !strings.Contains(err.Error(), "unknown task type") {
			t.Errorf("error should mention unknown task type, got: %v", err)
		}
	})

	t.Run("returns error for unknown type when config is missing", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "missing.yaml")

		_, err := GetSkillForTaskType("custom-type", configPath)
		if err == nil {
			t.Error("expected error for unknown task type with no config, got nil")
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
			// No config, no SKILL.md — should return hardcoded fallback content.
			content, err := GetSkillForTaskType(taskType, configPath)
			if err != nil {
				t.Errorf("task type %q: unexpected error: %v", taskType, err)
				continue
			}
			if content == "" {
				t.Errorf("task type %q: expected non-empty fallback content", taskType)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// WriteActiveTask tests
// ---------------------------------------------------------------------------

func TestWriteActiveTask(t *testing.T) {
	t.Run("writes ACTIVE_TASK.md to logs dir", func(t *testing.T) {
		dir := t.TempDir()
		logsDir := filepath.Join(dir, "logs")
		configPath := filepath.Join(dir, ".claude", "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"feature": "impl-feature"},
			map[string]string{"impl-feature": "# Feature Skill Instructions"},
		)

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:           "EPIC-4-002",
			TaskType:         types.TaskTypeFeature,
			SessionFilePath:  "logs/sessions/EPIC-4/session-EPIC-4-002_attempt-1.md",
			LogsDir:          logsDir,
			SkillsConfigPath: configPath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		outPath := filepath.Join(logsDir, "ACTIVE_TASK.md")
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("ACTIVE_TASK.md not found: %v", err)
		}
		content := string(data)

		for _, want := range []string{
			"EPIC-4-002",
			"feature",
			"logs/sessions/EPIC-4/session-EPIC-4-002_attempt-1.md",
			"# Feature Skill Instructions",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("expected %q in ACTIVE_TASK.md, got:\n%s", want, content)
			}
		}
	})

	t.Run("overwrites existing ACTIVE_TASK.md", func(t *testing.T) {
		dir := t.TempDir()
		logsDir := filepath.Join(dir, "logs")
		configPath := filepath.Join(dir, ".claude", "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"feature": "impl-feature"},
			map[string]string{"impl-feature": "# Skill v2"},
		)

		// Write a first version.
		writeFile(t, filepath.Join(logsDir, "ACTIVE_TASK.md"), "old content")

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:           "EPIC-4-002",
			TaskType:         types.TaskTypeFeature,
			SessionFilePath:  "session.md",
			LogsDir:          logsDir,
			SkillsConfigPath: configPath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(logsDir, "ACTIVE_TASK.md"))
		if strings.Contains(string(data), "old content") {
			t.Error("ACTIVE_TASK.md was not overwritten")
		}
	})

	t.Run("bugfix task includes bug context from ACTIVE_BUG.md", func(t *testing.T) {
		dir := t.TempDir()
		logsDir := filepath.Join(dir, "logs")
		configPath := filepath.Join(dir, ".claude", "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"bugfix": "impl-bugfix"},
			map[string]string{"impl-bugfix": "# Bugfix Skill"},
		)
		writeFile(t, filepath.Join(logsDir, "ACTIVE_BUG.md"), "## Bug Report\nnull pointer at line 42")

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:           "BUG-EPIC-4-001",
			TaskType:         types.TaskTypeBugfix,
			SessionFilePath:  "session.md",
			LogsDir:          logsDir,
			SkillsConfigPath: configPath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(logsDir, "ACTIVE_TASK.md"))
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
		logsDir := filepath.Join(dir, "logs")
		configPath := filepath.Join(dir, ".claude", "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"bugfix": "impl-bugfix"},
			map[string]string{"impl-bugfix": "# Bugfix Skill"},
		)
		// Do NOT write ACTIVE_BUG.md.

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:           "BUG-EPIC-4-001",
			TaskType:         types.TaskTypeBugfix,
			SessionFilePath:  "session.md",
			LogsDir:          logsDir,
			SkillsConfigPath: configPath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(logsDir, "ACTIVE_TASK.md"))
		if strings.Contains(string(data), "Bug Context") {
			t.Error("Bug Context section should be omitted when ACTIVE_BUG.md is missing")
		}
	})

	t.Run("feature task does not include bug context section", func(t *testing.T) {
		dir := t.TempDir()
		logsDir := filepath.Join(dir, "logs")
		configPath := filepath.Join(dir, ".claude", "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"feature": "impl-feature"},
			map[string]string{"impl-feature": "# Feature Skill"},
		)
		// Write ACTIVE_BUG.md — it should NOT appear for a feature task.
		writeFile(t, filepath.Join(logsDir, "ACTIVE_BUG.md"), "## Bug Report")

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:           "EPIC-4-002",
			TaskType:         types.TaskTypeFeature,
			SessionFilePath:  "session.md",
			LogsDir:          logsDir,
			SkillsConfigPath: configPath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(logsDir, "ACTIVE_TASK.md"))
		if strings.Contains(string(data), "Bug Context") {
			t.Error("Bug Context section should not appear for non-bugfix tasks")
		}
	})

	t.Run("documentation task type is preserved correctly", func(t *testing.T) {
		dir := t.TempDir()
		logsDir := filepath.Join(dir, "logs")
		configPath := filepath.Join(dir, ".claude", "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"documentation": "impl-documentation"},
			map[string]string{"impl-documentation": "# Documentation Skill"},
		)

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:           "KB_UPDATE",
			TaskType:         types.TaskTypeDocumentation,
			SessionFilePath:  "session.md",
			LogsDir:          logsDir,
			SkillsConfigPath: configPath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(logsDir, "ACTIVE_TASK.md"))
		content := string(data)

		if !strings.Contains(content, "documentation") {
			t.Errorf("expected task type 'documentation' in ACTIVE_TASK.md, got:\n%s", content)
		}
		if !strings.Contains(content, "# Documentation Skill") {
			t.Errorf("expected documentation skill content, got:\n%s", content)
		}
	})

	t.Run("description and acceptance criteria appear in output when provided", func(t *testing.T) {
		dir := t.TempDir()
		logsDir := filepath.Join(dir, "logs")
		configPath := filepath.Join(dir, ".claude", "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"feature": "impl-feature"},
			map[string]string{"impl-feature": "# Feature Skill"},
		)

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:             "EPIC-1-001",
			TaskType:           types.TaskTypeFeature,
			SessionFilePath:    "session.md",
			LogsDir:            logsDir,
			SkillsConfigPath:   configPath,
			Description:        "Implement the first feature.",
			AcceptanceCriteria: []string{"Tests pass", "Build succeeds"},
			Attempts:           1,
			MaxRetries:         5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(logsDir, "ACTIVE_TASK.md"))
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
		logsDir := filepath.Join(dir, "logs")
		configPath := filepath.Join(dir, ".claude", "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"feature": "impl-feature"},
			map[string]string{"impl-feature": "# Feature Skill"},
		)

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:           "EPIC-1-001",
			TaskType:         types.TaskTypeFeature,
			SessionFilePath:  "session.md",
			LogsDir:          logsDir,
			SkillsConfigPath: configPath,
			Attempts:         3,
			MaxRetries:       5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(logsDir, "ACTIVE_TASK.md"))
		content := string(data)

		if !strings.Contains(content, "3 of 5") {
			t.Errorf("expected '3 of 5' in ACTIVE_TASK.md, got:\n%s", content)
		}
	})

	t.Run("empty description and criteria handled gracefully", func(t *testing.T) {
		dir := t.TempDir()
		logsDir := filepath.Join(dir, "logs")
		configPath := filepath.Join(dir, ".claude", "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"feature": "impl-feature"},
			map[string]string{"impl-feature": "# Feature Skill"},
		)

		// Empty description and nil criteria — should not panic or emit an empty bullet list.
		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:             "EPIC-1-001",
			TaskType:           types.TaskTypeFeature,
			SessionFilePath:    "session.md",
			LogsDir:            logsDir,
			SkillsConfigPath:   configPath,
			Description:        "",
			AcceptanceCriteria: nil,
			Attempts:           1,
			MaxRetries:         5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(logsDir, "ACTIVE_TASK.md"))
		content := string(data)

		// Description line must be absent.
		if strings.Contains(content, "**Description**") {
			t.Error("empty description should not emit a Description line")
		}
		// Acceptance Criteria section must be absent when criteria is empty.
		if strings.Contains(content, "**Acceptance Criteria**") {
			t.Error("empty criteria should not emit an Acceptance Criteria section")
		}
	})

	t.Run("synthetic task (empty description/criteria) produces valid output", func(t *testing.T) {
		dir := t.TempDir()
		logsDir := filepath.Join(dir, "logs")
		configPath := filepath.Join(dir, ".claude", "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"bugfix": "impl-bugfix"},
			map[string]string{"impl-bugfix": "# Bugfix Skill"},
		)
		writeFile(t, filepath.Join(logsDir, "ACTIVE_BUG.md"), "## Bug\nnull pointer")

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:             "BUG-EPIC-1-001",
			TaskType:           types.TaskTypeBugfix,
			SessionFilePath:    "session.md",
			LogsDir:            logsDir,
			SkillsConfigPath:   configPath,
			Description:        "", // synthetic — always empty
			AcceptanceCriteria: nil,
			Attempts:           1,
			MaxRetries:         5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(logsDir, "ACTIVE_TASK.md"))
		content := string(data)

		// Must still contain the basic header fields and skill content.
		if !strings.Contains(content, "BUG-EPIC-1-001") {
			t.Errorf("expected task ID in output, got:\n%s", content)
		}
		if !strings.Contains(content, "1 of 5") {
			t.Errorf("expected attempt info in output, got:\n%s", content)
		}
		if !strings.Contains(content, "# Bugfix Skill") {
			t.Errorf("expected skill content in output, got:\n%s", content)
		}
		// No empty acceptance criteria section.
		if strings.Contains(content, "**Acceptance Criteria**") {
			t.Error("synthetic task should not emit Acceptance Criteria section")
		}
	})

	t.Run("creates logs directory if it does not exist", func(t *testing.T) {
		dir := t.TempDir()
		logsDir := filepath.Join(dir, "nested", "logs")
		configPath := filepath.Join(dir, ".claude", "skills-config.yaml")
		makeSkillsConfig(t, configPath,
			map[string]string{"feature": "impl-feature"},
			map[string]string{"impl-feature": "# Skill"},
		)

		err := WriteActiveTask(ActiveTaskConfig{
			TaskID:           "EPIC-4-002",
			TaskType:         types.TaskTypeFeature,
			SessionFilePath:  "session.md",
			LogsDir:          logsDir,
			SkillsConfigPath: configPath,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, statErr := os.Stat(filepath.Join(logsDir, "ACTIVE_TASK.md")); statErr != nil {
			t.Errorf("ACTIVE_TASK.md not found: %v", statErr)
		}
	})
}
