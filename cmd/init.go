package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
)

var initFlags struct {
	force       bool
	buildSystem string
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new doug project",
	Long:  "Scaffold a new doug project with doug.yaml, tasks.yaml, and PRD.md.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initFlags.force, "force", false, "Overwrite existing files")
	initCmd.Flags().StringVar(&initFlags.buildSystem, "build-system", "", "Build system to use (go|npm); auto-detected if not set")
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	return initProject(dir, initFlags.force, initFlags.buildSystem)
}

// initProject is the testable core of the init command. It generates doug.yaml,
// tasks.yaml, and PRD.md in the given directory.
func initProject(dir string, force bool, buildSystem string) error {
	// Guard: refuse to re-initialize an existing project unless --force is set.
	if !force {
		for _, name := range []string{"project-state.yaml", "tasks.yaml"} {
			if _, statErr := os.Stat(filepath.Join(dir, name)); statErr == nil {
				return fmt.Errorf("%s already exists — project appears to be already initialized; use --force to overwrite", name)
			}
		}
	}

	// Determine the build system (flag > auto-detect > default).
	bs := buildSystem
	if bs == "" {
		bs = config.DetectBuildSystem(dir)
	}

	type fileSpec struct {
		name    string
		content string
	}
	specs := []fileSpec{
		{"doug.yaml", dougYAMLContent(bs)},
		{"tasks.yaml", tasksYAMLContent()},
		{"PRD.md", prdContent()},
	}

	for _, spec := range specs {
		path := filepath.Join(dir, spec.name)
		if !force {
			if _, statErr := os.Stat(path); statErr == nil {
				log.Warning(fmt.Sprintf("%s already exists — skipping (use --force to overwrite)", spec.name))
				continue
			}
		}
		if err := os.WriteFile(path, []byte(spec.content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", spec.name, err)
		}
		log.Success(fmt.Sprintf("created %s", spec.name))
	}

	log.Info("project initialized — edit doug.yaml and tasks.yaml, then run: doug run")
	return nil
}

// dougYAMLContent returns the doug.yaml file content with inline YAML comments
// and the detected (or specified) build system pre-filled.
func dougYAMLContent(buildSystem string) string {
	return fmt.Sprintf(`# doug.yaml — orchestrator configuration
# See https://github.com/robertgumeny/doug for documentation.
agent_command: claude   # Command used to invoke the agent (e.g. claude, aider)
build_system: %s        # Build system: go | npm (auto-detected by init; override here)
max_retries: 5          # Max FAILURE outcomes before a task is BLOCKED
max_iterations: 20      # Max loop iterations before the run exits
kb_enabled: true        # If false, skip KB synthesis task after features complete
`, buildSystem)
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
