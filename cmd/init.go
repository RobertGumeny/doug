package cmd

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/templates"
)

var initFlags struct {
	force       bool
	buildSystem string
	agents      string // comma-separated agent names (non-interactive override)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new doug project",
	Long:  "Scaffold a new doug project with .doug/doug.yaml, .doug/tasks.yaml, and .doug/PRD.md.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initFlags.force, "force", false, "Overwrite existing files")
	initCmd.Flags().StringVar(&initFlags.buildSystem, "build-system", "", "Build system to use (go|npm); auto-detected if not set")
	initCmd.Flags().StringVar(&initFlags.agents, "agents", "", "Comma-separated agent names to install skills for (e.g. claude,codex)")
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Determine selected agents: flag > interactive TTY > default.
	var selectedAgents []string
	if initFlags.agents != "" {
		for _, a := range strings.Split(initFlags.agents, ",") {
			if a = strings.TrimSpace(a); a != "" {
				selectedAgents = append(selectedAgents, a)
			}
		}
	} else {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			selectedAgents = promptAgentSelection()
		} else {
			selectedAgents = []string{"claude"}
		}
	}
	if len(selectedAgents) == 0 {
		selectedAgents = []string{"claude"}
	}

	return initProject(dir, initFlags.force, initFlags.buildSystem, selectedAgents)
}

// promptAgentSelection shows an interactive agent selection menu on a TTY.
// Returns the selected agent names; defaults to ["claude"] on empty input.
func promptAgentSelection() []string {
	type agentOption struct {
		name     string
		skillDir string
	}
	options := []agentOption{
		{"claude", agentRegistry["claude"].skillsDir},
		{"codex", agentRegistry["codex"].skillsDir},
		{"gemini", agentRegistry["gemini"].skillsDir},
	}

	fmt.Println("Which agent(s) are you using? (comma-separated numbers, or press Enter for Claude)")
	for i, opt := range options {
		marker := "[ ]"
		if i == 0 {
			marker = "[x]"
		}
		fmt.Printf("  %d. %s %-10s → %s\n", i+1, marker, opt.name, opt.skillDir)
	}
	fmt.Print("Selection (e.g. 1,2): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return []string{"claude"}
	}
	input = strings.TrimSpace(input)

	if input == "" {
		return []string{"claude"}
	}

	var selected []string
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > len(options) {
			continue
		}
		selected = append(selected, options[n-1].name)
	}
	if len(selected) == 0 {
		return []string{"claude"}
	}
	return selected
}

// initProject is the testable core of the init command. It generates the .doug/
// directory with doug.yaml, project-state.yaml, tasks.yaml, and PRD.md.
// selectedAgents controls which agent skill directories are populated.
func initProject(dir string, force bool, buildSystem string, selectedAgents []string) error {
	dougDir := filepath.Join(dir, ".doug")

	// Guard: refuse to re-initialize an existing project unless --force is set.
	if !force {
		if _, statErr := os.Stat(filepath.Join(dougDir, "project-state.yaml")); statErr == nil {
			return fmt.Errorf(".doug/project-state.yaml already exists — project appears to be already initialized; use --force to overwrite")
		}
	}

	// Ensure .doug/ directory exists.
	if err := os.MkdirAll(dougDir, 0o755); err != nil {
		return fmt.Errorf("create .doug directory: %w", err)
	}

	// Determine the build system (flag > auto-detect > default).
	bs := buildSystem
	if bs == "" {
		bs = config.DetectBuildSystem(dir)
	}

	// Validate explicit build system flag.
	if buildSystem != "" {
		switch bs {
		case "go", "npm":
		default:
			return fmt.Errorf("unsupported build system %q: must be one of: go, npm", bs)
		}
	}

	// Warn on unknown agent names before doing any work.
	for _, name := range selectedAgents {
		if _, ok := agentRegistry[name]; !ok {
			log.Warning(fmt.Sprintf("unknown agent %q — no skills directory defined; skipping skill copy for this agent", name))
		}
	}

	// Derive skills_dir from the first known agent.
	skillsDir := ".agents/skills" // fallback
	for _, name := range selectedAgents {
		if info, ok := agentRegistry[name]; ok {
			skillsDir = info.skillsDir
			break
		}
	}

	type fileSpec struct {
		path    string
		content string
	}
	specs := []fileSpec{
		{filepath.Join(dougDir, "doug.yaml"), dougYAMLContent(bs, skillsDir)},
		{filepath.Join(dougDir, "project-state.yaml"), projectStateContent()},
		{filepath.Join(dougDir, "tasks.yaml"), tasksYAMLContent()},
		{filepath.Join(dougDir, "PRD.md"), prdContent()},
	}

	for _, spec := range specs {
		if !force {
			if _, statErr := os.Stat(spec.path); statErr == nil {
				log.Warning(fmt.Sprintf("%s already exists — skipping (use --force to overwrite)", spec.path))
				continue
			}
		}
		if err := state.AtomicWrite(spec.path, []byte(spec.content)); err != nil {
			return fmt.Errorf("write %s: %w", spec.path, err)
		}
		log.Success(fmt.Sprintf("created %s", spec.path))
	}

	// Copy embedded init/ templates into the target project.
	if err := copyInitTemplates(dir, force, selectedAgents); err != nil {
		return err
	}

	// Create docs/kb/ directory (silent if already exists).
	kbDir := filepath.Join(dir, "docs", "kb")
	if _, statErr := os.Stat(kbDir); os.IsNotExist(statErr) {
		if err := os.MkdirAll(kbDir, 0o755); err != nil {
			return fmt.Errorf("create docs/kb directory: %w", err)
		}
		log.Success("created docs/kb/")
	}

	// Create CHANGELOG.md at project root if it does not already exist.
	// Never overwrite an existing CHANGELOG.md — it is user-maintained.
	changelogPath := filepath.Join(dir, "CHANGELOG.md")
	if _, statErr := os.Stat(changelogPath); os.IsNotExist(statErr) {
		if err := state.AtomicWrite(changelogPath, []byte(changelogContent())); err != nil {
			return fmt.Errorf("write CHANGELOG.md: %w", err)
		}
		log.Success("created CHANGELOG.md")
	}

	log.Info("project initialized — edit .doug/doug.yaml and .doug/tasks.yaml, then run: doug run")
	return nil
}

// copyInitTemplates walks the embedded init/ FS and copies files to the target project.
//
// Destination mapping:
//   - init/CLAUDE.md, init/AGENTS.md      → skipped
//   - init/skills-config.yaml             → {dir}/.doug/skills-config.yaml
//   - init/*_TEMPLATE.md                  → {dir}/.doug/logs/
//   - init/skills/**                      → {dir}/.agents/skills/
//   - init/.gitignore                     → {dir}/.gitignore
//   - init/.gemini/settings.json          → {dir}/.gemini/settings.json
func copyInitTemplates(dir string, force bool, selectedAgents []string) error {
	return fs.WalkDir(templates.Init, "init", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Strip the "init/" prefix to get the relative path within the init tree.
		rel := strings.TrimPrefix(path, "init/")

		// Skip files that are no longer scaffolded.
		switch rel {
		case "CLAUDE.md":
			return nil
		}

		// Skills: copy to shared .agents/skills/ directory.
		if strings.HasPrefix(rel, "skills/") {
			skillRel := strings.TrimPrefix(rel, "skills/")
			dst := filepath.Join(dir, ".agents", "skills", skillRel)
			data, readErr := templates.Init.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("read template %s: %w", path, readErr)
			}
			if !force {
				if _, statErr := os.Stat(dst); statErr == nil {
					log.Warning(fmt.Sprintf("%s already exists — skipping (use --force to overwrite)", dst))
					return nil
				}
			}
			if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
				return fmt.Errorf("create directory for %s: %w", dst, mkErr)
			}
			if writeErr := state.AtomicWrite(dst, data); writeErr != nil {
				return fmt.Errorf("write %s: %w", dst, writeErr)
			}
			log.Success(fmt.Sprintf("created %s", dst))
			return nil
		}

		// Determine single destination path for non-skills files.
		var dst string
		switch {
		case rel == ".gitignore":
			dst = filepath.Join(dir, rel)
		case rel == "AGENTS.md":
			dst = filepath.Join(dir, "AGENTS.md")
		case rel == "skills-config.yaml":
			dst = filepath.Join(dir, ".doug", "skills-config.yaml")
		case rel == ".gemini/settings.json":
			dst = filepath.Join(dir, ".gemini", "settings.json")
		case strings.HasSuffix(rel, "_TEMPLATE.md"):
			dst = filepath.Join(dir, ".doug", "logs", rel)
		default:
			log.Warning(fmt.Sprintf("skipping unknown template file: %s", rel))
			return nil
		}

		if !force {
			if _, statErr := os.Stat(dst); statErr == nil {
				log.Warning(fmt.Sprintf("%s already exists — skipping (use --force to overwrite)", dst))
				return nil
			}
		}

		// Ensure parent directory exists.
		if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
			return fmt.Errorf("create directory for %s: %w", dst, mkErr)
		}

		data, readErr := templates.Init.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read template %s: %w", path, readErr)
		}

		if writeErr := state.AtomicWrite(dst, data); writeErr != nil {
			return fmt.Errorf("write %s: %w", dst, writeErr)
		}

		log.Success(fmt.Sprintf("created %s", dst))
		return nil
	})
}

// dougYAMLContent returns the .doug/doug.yaml file content with inline YAML comments
// and the detected (or specified) build system pre-filled.
func dougYAMLContent(buildSystem, skillsDir string) string {
	return fmt.Sprintf(`# doug.yaml — orchestrator configuration
# See https://github.com/robertgumeny/doug for documentation.
agent_command: claude "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md" # Command used to invoke the agent (e.g. claude, codex, gemini, etc.)
# agent_command: codex --ask-for-approval never --sandbox workspace-write "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"
# agent_command: gemini --approval-mode auto_edit --sandbox "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"
skills_dir: %s # Path to skills directory relative to project root
build_system: %s # Build system: go | npm (auto-detected by init; override here)
max_retries: 3 # Max FAILURE outcomes before a task is BLOCKED
max_iterations: 10 # Max loop iterations before the run exits
kb_enabled: true # If false, skip KB synthesis task after features complete
`, skillsDir, buildSystem)
}

// tasksYAMLContent returns a starter tasks.yaml with one example epic and two tasks,
// containing all required fields.
func tasksYAMLContent() string {
	return `epic:
  id: "EPIC-1"
  name: "First Epic"
  tasks:
    - id: "EPIC-1-001"
      type: "feature"
      status: "TODO"
      description: "Implement the first feature of the project."
      acceptance_criteria:
        - "The feature is implemented and all related tests pass"
        - "Code follows the project's conventions and style guidelines"
    - id: "EPIC-1-002"
      type: "feature"
      status: "TODO"
      description: "Implement the second feature of the project."
      acceptance_criteria:
        - "The feature is implemented and all related tests pass"
        - "All acceptance criteria have been verified end-to-end"
`
}

// projectStateContent returns a minimal valid project-state.yaml for a new project.
// BootstrapFromTasks fires on first run because state.CurrentEpic.ID is empty,
// populating the rest of the state from tasks.yaml.
func projectStateContent() string {
	return "{}\n"
}

// changelogContent returns a starter CHANGELOG.md following the Keep a Changelog format.
func changelogContent() string {
	return `# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
`
}

// prdContent returns a starter PRD.md template for new projects.
func prdContent() string {
	return `# PRD: [Project Name]

**Version**: 1.0
**Status**: Draft

---

## Problem

[Describe the problem this project solves and why it matters.]

---

## Goal

[What does success look like? What will this project produce?]

---

## Non-Goals

- [What is explicitly out of scope?]

---

## Architecture

[High-level architecture diagram or description.]

---

## Epics

| Epic | Theme | Tasks | Depends On |
|------|-------|-------|------------|
| 1    | [Theme] | 2  | —          |

---

## Definition of Done

- [ ] All tasks are DONE
- [ ] Build passes
- [ ] Tests pass
`
}
