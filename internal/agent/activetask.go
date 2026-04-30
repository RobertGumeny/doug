package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	buildconfig "github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/types"
)

// ActiveTaskConfig holds the parameters for writing .doug/ACTIVE_TASK.md.
type ActiveTaskConfig struct {
	TaskID   string
	TaskType types.TaskType
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
	// BuildSystem is the build system identifier (e.g. "go", "npm", "pnpm").
	// If set and found in the BuildSystems registry, a "## Build System" briefing
	// section is injected into ACTIVE_TASK.md. If empty or unknown, the section is omitted.
	BuildSystem string
	// TestFailureOutput holds the captured output from a failed test run on the
	// previous attempt. When non-empty, it is injected into ACTIVE_TASK.md so
	// the agent can see what tests are failing and fix them.
	TestFailureOutput string
	// ContextSections appends structured context blocks to ACTIVE_TASK.md.
	// This is used for synthetic tasks like scaffold that need extra agent-facing
	// context beyond the standard task description and criteria.
	ContextSections []ActiveTaskSection
}

// ActiveTaskSection is an extra markdown section appended to ACTIVE_TASK.md.
type ActiveTaskSection struct {
	Heading string
	Body    string
}

// skillsConfigFile mirrors the YAML structure of skills-config.yaml.
type skillsConfigFile struct {
	SkillMappings map[string]string `yaml:"skill_mappings"`
}

// hardcodedSkillNames maps known task types to their built-in workflow skill names.
// This map is the canonical last-resort fallback for skill resolution. It will remain
// in place after the skills-config.yaml legacy tier is removed during final rollout.
// Repository-specific operating rules live in AGENTS.md; this map only selects the
// task workflow.
var hardcodedSkillNames = map[string]string{
	string(types.TaskTypeFeature):       "implement-feature",
	string(types.TaskTypeBugfix):        "implement-bugfix",
	string(types.TaskTypeDocumentation): "implement-documentation",
	string(types.TaskTypeManualReview):  "manual-review",
	string(types.TaskTypeScaffold):      "scaffold",
	"plan":                              "plan",
}

// DefaultSkillName returns the built-in skill name for taskType from hardcodedSkillNames.
// This is the fallback that PrepareExecution will use directly once the legacy
// skills-config.yaml tier is removed. Returns ("", false) for unknown task types.
func DefaultSkillName(taskType string) (string, bool) {
	name, ok := hardcodedSkillNames[taskType]
	return name, ok
}

// GetSkillForTaskType returns the skill name for taskType. Resolution order:
//  1. skills-config.yaml at configPath (LEGACY tier — see deprecation note below)
//  2. hardcodedSkillNames (will remain after the legacy tier is removed)
//
// Deprecated: the configPath / skills-config.yaml tier is superseded by
// policy.tasks[taskType].skill in .doug/doug.yaml (PolicyConfig.ResolveSkill).
// During final rollout: remove the configPath parameter and the os.ReadFile block,
// and have PrepareExecution call DefaultSkillName directly as the fallback.
func GetSkillForTaskType(taskType, configPath string) (string, error) {
	// LEGACY: file-based skill mapping via skills-config.yaml. Superseded by
	// policy.tasks[taskType].skill in doug.yaml. Remove this block during final rollout.
	data, err := os.ReadFile(configPath)
	if err == nil {
		var cfg skillsConfigFile
		if yamlErr := yaml.Unmarshal(data, &cfg); yamlErr == nil && cfg.SkillMappings != nil {
			if name, ok := cfg.SkillMappings[taskType]; ok && name != "" {
				return name, nil
			}
		}
	}

	// Hardcoded defaults — this tier is canonical and remains after skills-config.yaml is removed.
	if name, ok := hardcodedSkillNames[taskType]; ok {
		return name, nil
	}
	return "", fmt.Errorf("unknown task type %q: no skill mapping found", taskType)
}

// WriteActiveTask writes .doug/ACTIVE_TASK.md with task metadata and a briefing
// header. The file is archived and cleaned up after the corresponding outcome is processed.
//
// For bugfix tasks, the content of .doug/ACTIVE_BUG.md is appended as a
// "Bug Context" section. If ACTIVE_BUG.md is missing, the section is omitted
// and a warning is logged.
func WriteActiveTask(config ActiveTaskConfig, l log.Logger) error {
	var sb strings.Builder
	sb.WriteString("# Active Task\n\n")
	fmt.Fprintf(&sb, "**Active Bug File**: %s\n", filepath.Join(config.DougDir, "ACTIVE_BUG.md"))
	fmt.Fprintf(&sb, "**Failure File**: %s\n", filepath.Join(config.DougDir, "ACTIVE_FAILURE.md"))
	fmt.Fprintf(&sb, "**PRD File**: %s\n", filepath.Join(config.DougDir, "PRD.md"))
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "**Task ID**: %s\n", config.TaskID)
	fmt.Fprintf(&sb, "**Task Type**: %s\n", string(config.TaskType))
	fmt.Fprintf(&sb, "**Attempt**: %d of %d\n", config.Attempts, config.MaxRetries)
	if config.Description != "" {
		fmt.Fprintf(&sb, "**Description**: %s\n", config.Description)
	}
	if len(config.AcceptanceCriteria) > 0 {
		sb.WriteString("\n**Acceptance Criteria**:\n")
		for _, criterion := range config.AcceptanceCriteria {
			fmt.Fprintf(&sb, "- %s\n", criterion)
		}
	}

	if info, ok := buildconfig.BuildSystems[config.BuildSystem]; ok {
		sb.WriteString("\n\n---\n\n## Build System\n\n")
		fmt.Fprintf(&sb, "**System**: %s\n", config.BuildSystem)
		fmt.Fprintf(&sb, "**Install**: `%s`\n", info.InstallCmd)
		if len(info.VerifyCommands) > 0 {
			sb.WriteString("**Verify**:\n")
			for _, cmd := range info.VerifyCommands {
				fmt.Fprintf(&sb, "- `%s`\n", cmd)
			}
		}
		if len(info.CommonPitfalls) > 0 {
			sb.WriteString("**Common Pitfalls**:\n")
			for _, p := range info.CommonPitfalls {
				fmt.Fprintf(&sb, "- %s\n", p)
			}
		}
	}

	if config.TaskType == types.TaskTypeBugfix {
		bugContent, bugErr := readBugContext(config.DougDir)
		if bugErr != nil {
			l.Warning(fmt.Sprintf("bug context unavailable: %v", bugErr))
		} else {
			sb.WriteString("\n\n---\n\n## Bug Context\n\n")
			sb.WriteString(bugContent)
		}
	}

	if config.TestFailureOutput != "" {
		sb.WriteString("\n\n---\n\n## Previous Test Failure Output\n\n")
		sb.WriteString("The previous attempt reported SUCCESS but the following tests failed during orchestrator verification.\n")
		sb.WriteString("Fix the failing tests before reporting SUCCESS again.\n\n")
		sb.WriteString("```\n")
		sb.WriteString(config.TestFailureOutput)
		sb.WriteString("\n```\n")
	}

	for _, section := range config.ContextSections {
		if strings.TrimSpace(section.Heading) == "" || strings.TrimSpace(section.Body) == "" {
			continue
		}
		sb.WriteString("\n\n---\n\n## ")
		sb.WriteString(section.Heading)
		sb.WriteString("\n\n")
		sb.WriteString(section.Body)
		if !strings.HasSuffix(section.Body, "\n") {
			sb.WriteString("\n")
		}
	}

	// Append the result block that the agent fills in.
	sb.WriteString("\n\n---\n\n## Agent Result\n\n")
	sb.WriteString("Allowed `outcome` values: `SUCCESS`, `FAILURE`, `BUG`, `EPIC_COMPLETE`.\n\n")
	sb.WriteString("---\n")
	sb.WriteString("outcome: \"\"\n")
	sb.WriteString("changelog_entry: \"\"\n")
	sb.WriteString("dependencies_added: []\n")
	sb.WriteString("---\n\n")
	sb.WriteString("## Implementation Summary\n\n")
	sb.WriteString("## Files Changed\n\n")
	sb.WriteString("## Key Decisions\n\n")
	sb.WriteString("## Test Coverage\n")

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
