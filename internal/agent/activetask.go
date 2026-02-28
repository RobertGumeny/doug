package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/types"
)

// ActiveTaskConfig holds the parameters for writing logs/ACTIVE_TASK.md.
type ActiveTaskConfig struct {
	TaskID          string
	TaskType        types.TaskType
	SessionFilePath string
	// LogsDir is the path to the logs directory (e.g. "logs"). ACTIVE_TASK.md
	// is written to {LogsDir}/ACTIVE_TASK.md. For bugfix tasks, ACTIVE_BUG.md
	// is also read from this directory.
	LogsDir         string
	// SkillsConfigPath is the path to skills-config.yaml
	// (e.g. ".claude/skills-config.yaml"). The skill files are resolved
	// relative to its parent directory.
	SkillsConfigPath string
	// Description is the task description from tasks.yaml. Empty for synthetic tasks.
	Description string
	// AcceptanceCriteria is the list of acceptance criteria from tasks.yaml.
	// Empty for synthetic tasks (bugfix, documentation).
	AcceptanceCriteria []string
	// Attempts is the current attempt number (already incremented before WriteActiveTask is called).
	Attempts int
	// MaxRetries is the configured maximum number of retries from doug.yaml.
	MaxRetries int
}

// skillsConfigFile mirrors the YAML structure of skills-config.yaml.
type skillsConfigFile struct {
	SkillMappings map[string]string `yaml:"skill_mappings"`
}

// hardcodedSkillNames maps known task types to their default skill names.
// These are used when skills-config.yaml is absent or does not contain the type.
var hardcodedSkillNames = map[string]string{
	string(types.TaskTypeFeature):       "implement-feature",
	string(types.TaskTypeBugfix):        "implement-bugfix",
	string(types.TaskTypeDocumentation): "implement-documentation",
	string(types.TaskTypeManualReview):  "manual-review",
}

// hardcodedSkillContent maps known task types to minimal fallback instructions.
// These are returned when the resolved SKILL.md file is missing from disk.
var hardcodedSkillContent = map[string]string{
	string(types.TaskTypeFeature): `# Feature Implementation

Implement the feature described in tasks.yaml.
Follow all instructions in CLAUDE.md.
Write your session summary to the session file path provided above.`,

	string(types.TaskTypeBugfix): `# Bug Fix

Fix the bug described in logs/ACTIVE_BUG.md.
Follow all instructions in CLAUDE.md.
Write your session summary to the session file path provided above.`,

	string(types.TaskTypeDocumentation): `# Documentation Synthesis

Synthesize session logs into documentation.
Follow all instructions in CLAUDE.md.
Write your session summary to the session file path provided above.`,

	string(types.TaskTypeManualReview): `# Manual Review

This task requires human intervention.
Review the current project state and provide guidance.`,
}

// GetSkillForTaskType resolves the skill instructions for taskType.
//
// Resolution order:
//  1. Read skills-config.yaml at configPath to find the skill name.
//     If the file is missing, fall back to hardcodedSkillNames.
//  2. Load the skill file content from {configDir}/skills/{skillName}/SKILL.md.
//     If the file is missing, fall back to hardcodedSkillContent and log a warning.
//
// Returns an error for task types not found in the config and with no hardcoded default.
func GetSkillForTaskType(taskType, configPath string) (string, error) {
	skillName, err := resolveSkillName(taskType, configPath)
	if err != nil {
		return "", err
	}

	configDir := filepath.Dir(configPath)
	skillFilePath := filepath.Join(configDir, "skills", skillName, "SKILL.md")

	data, err := os.ReadFile(skillFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Warning(fmt.Sprintf("skill file not found at %s, using hardcoded fallback", skillFilePath))
			if fallback, ok := hardcodedSkillContent[taskType]; ok {
				return fallback, nil
			}
			// resolveSkillName already validated the type, so this branch is unreachable
			// for the four known types. Guard it anyway.
			return "", fmt.Errorf("no fallback content for task type %q", taskType)
		}
		return "", fmt.Errorf("read skill file %s: %w", skillFilePath, err)
	}

	return string(data), nil
}

// WriteActiveTask writes logs/ACTIVE_TASK.md with task metadata and skill
// instructions. The file is always overwritten; it is never archived.
//
// For bugfix tasks, the content of logs/ACTIVE_BUG.md is appended as a
// "Bug Context" section. If ACTIVE_BUG.md is missing, the section is omitted
// and a warning is logged.
func WriteActiveTask(config ActiveTaskConfig) error {
	skillContent, err := GetSkillForTaskType(string(config.TaskType), config.SkillsConfigPath)
	if err != nil {
		return fmt.Errorf("get skill for task type %q: %w", config.TaskType, err)
	}

	var sb strings.Builder
	sb.WriteString("# Active Task\n\n")
	sb.WriteString(fmt.Sprintf("**Task ID**: %s\n", config.TaskID))
	sb.WriteString(fmt.Sprintf("**Task Type**: %s\n", string(config.TaskType)))
	sb.WriteString(fmt.Sprintf("**Session File**: %s\n", config.SessionFilePath))
	sb.WriteString(fmt.Sprintf("**Attempt**: %d of %d\n", config.Attempts, config.MaxRetries))
	if config.Description != "" {
		sb.WriteString(fmt.Sprintf("**Description**: %s\n", config.Description))
	}
	if len(config.AcceptanceCriteria) > 0 {
		sb.WriteString("\n**Acceptance Criteria**:\n")
		for _, criterion := range config.AcceptanceCriteria {
			sb.WriteString(fmt.Sprintf("- %s\n", criterion))
		}
	}

	sb.WriteString("# Skill to Use\n\n")
	sb.WriteString("\n---\n\n")
	sb.WriteString(skillContent)

	if config.TaskType == types.TaskTypeBugfix {
		bugContent, bugErr := readBugContext(config.LogsDir)
		if bugErr != nil {
			log.Warning(fmt.Sprintf("bug context unavailable: %v", bugErr))
		} else {
			sb.WriteString("\n\n---\n\n## Bug Context\n\n")
			sb.WriteString(bugContent)
		}
	}

	outPath := filepath.Join(config.LogsDir, "ACTIVE_TASK.md")
	if err := os.MkdirAll(config.LogsDir, 0o755); err != nil {
		return fmt.Errorf("create logs directory %s: %w", config.LogsDir, err)
	}
	if err := os.WriteFile(outPath, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("write ACTIVE_TASK.md: %w", err)
	}

	return nil
}

// resolveSkillName returns the skill name for taskType by reading
// skills-config.yaml. If the file is absent or the type is not listed,
// hardcodedSkillNames is consulted. Returns an error when the type is unknown
// in both sources.
func resolveSkillName(taskType, configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err == nil {
		var cfg skillsConfigFile
		if yamlErr := yaml.Unmarshal(data, &cfg); yamlErr == nil && cfg.SkillMappings != nil {
			if name, ok := cfg.SkillMappings[taskType]; ok && name != "" {
				return name, nil
			}
		}
	}

	// Config absent or type not listed — try hardcoded defaults.
	if name, ok := hardcodedSkillNames[taskType]; ok {
		return name, nil
	}
	return "", fmt.Errorf("unknown task type %q: no skill mapping found", taskType)
}

// readBugContext reads logs/ACTIVE_BUG.md and returns its content.
func readBugContext(logsDir string) (string, error) {
	bugPath := filepath.Join(logsDir, "ACTIVE_BUG.md")
	data, err := os.ReadFile(bugPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", bugPath, err)
	}
	return string(data), nil
}
