package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/types"
)

// ActiveTaskConfig holds the parameters for writing .doug/ACTIVE_TASK.md.
type ActiveTaskConfig struct {
	TaskID          string
	TaskType        types.TaskType
	SessionFilePath string
	// DougDir is the path to the .doug/ directory. ACTIVE_TASK.md is written
	// to {DougDir}/ACTIVE_TASK.md. For bugfix tasks, ACTIVE_BUG.md is also
	// read from this directory.
	DougDir string
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

// hardcodedSkillContent maps known task types to their default skill names.
// These are used when skills-config.yaml is absent or does not contain the type.
var hardcodedSkillContent = map[string]string{
	string(types.TaskTypeFeature):       "implement-feature",
	string(types.TaskTypeBugfix):        "implement-bugfix",
	string(types.TaskTypeDocumentation): "implement-documentation",
	string(types.TaskTypeManualReview):  "manual-review",
}

// GetSkillForTaskType returns the skill name for taskType by reading skills-config.yaml
// at configPath. If the file is absent or the type is not listed,
// hardcodedSkillContent is consulted. Returns an error when the type is unknown
// in both sources.
func GetSkillForTaskType(taskType, configPath string) (string, error) {
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
	if name, ok := hardcodedSkillContent[taskType]; ok {
		return name, nil
	}
	return "", fmt.Errorf("unknown task type %q: no skill mapping found", taskType)
}

// WriteActiveTask writes .doug/ACTIVE_TASK.md with task metadata and a briefing
// header. The file is always overwritten; it is never archived.
//
// For bugfix tasks, the content of .doug/ACTIVE_BUG.md is appended as a
// "Bug Context" section. If ACTIVE_BUG.md is missing, the section is omitted
// and a warning is logged.
func WriteActiveTask(config ActiveTaskConfig) error {
	var sb strings.Builder
	sb.WriteString("# Active Task\n\n")
	sb.WriteString(fmt.Sprintf("**Session File**: %s\n", config.SessionFilePath))
	sb.WriteString(fmt.Sprintf("**Active Bug File**: %s\n", filepath.Join(config.DougDir, "ACTIVE_BUG.md")))
	sb.WriteString(fmt.Sprintf("**Failure File**: %s\n", filepath.Join(config.DougDir, "ACTIVE_FAILURE.md")))
	sb.WriteString(fmt.Sprintf("**PRD File**: %s\n", filepath.Join(config.DougDir, "PRD.md")))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("**Task ID**: %s\n", config.TaskID))
	sb.WriteString(fmt.Sprintf("**Task Type**: %s\n", string(config.TaskType)))
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

	if config.TaskType == types.TaskTypeBugfix {
		bugContent, bugErr := readBugContext(config.DougDir)
		if bugErr != nil {
			log.Warning(fmt.Sprintf("bug context unavailable: %v", bugErr))
		} else {
			sb.WriteString("\n\n---\n\n## Bug Context\n\n")
			sb.WriteString(bugContent)
		}
	}

	outPath := filepath.Join(config.DougDir, "ACTIVE_TASK.md")
	if err := os.MkdirAll(config.DougDir, 0o755); err != nil {
		return fmt.Errorf("create .doug directory %s: %w", config.DougDir, err)
	}
	if err := os.WriteFile(outPath, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("write ACTIVE_TASK.md: %w", err)
	}

	return nil
}

// readBugContext reads .doug/ACTIVE_BUG.md and returns its content.
func readBugContext(dougDir string) (string, error) {
	bugPath := filepath.Join(dougDir, "ACTIVE_BUG.md")
	data, err := os.ReadFile(bugPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", bugPath, err)
	}
	return string(data), nil
}
