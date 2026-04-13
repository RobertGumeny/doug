package cmd

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/prompt"
	"github.com/robertgumeny/doug/internal/state"
)

var initFlags struct {
	force       bool
	buildSystem string
	agents      string // comma-separated agent names (non-interactive override)
	noGitInit   bool
}
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new doug project",
	Long:  "Scaffold a new doug project with .doug/doug.yaml, .doug/tasks.yaml, and .doug/PRD.md.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initFlags.force, "force", false, "Overwrite existing files")
	initCmd.Flags().StringVar(&initFlags.buildSystem, "build-system", "", "Build system to use (go|npm|pnpm); auto-detected if not set")
	initCmd.Flags().StringVar(&initFlags.agents, "agents", "", "Comma-separated agent names to install skills for (e.g. claude,codex)")
	initCmd.Flags().BoolVar(&initFlags.noGitInit, "no-git-init", false, "Skip running git init")
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	isTTY := prompt.IsTTY(os.Stdin)
	return runInitWorkflow(os.Stdout, os.Stdin, isTTY, dir, initWorkflowOptions{
		force:       initFlags.force,
		buildSystem: initFlags.buildSystem,
		agents:      initFlags.agents,
		noGitInit:   initFlags.noGitInit,
	})
}

// initProject is retained as a backward-compatible wrapper for tests. It uses
// non-interactive defaults (maxRetries=3, maxIterations=10, kbEnabled=true)
// and discards terminal output.
func initProject(dir string, force bool, buildSystem string, selectedAgents []string, noGitInit bool) error {
	return doInitProject(io.Discard, dir, force, buildSystem, selectedAgents, noGitInit, 3, 10, true)
}

// doInitProject is the testable core of the init command. It generates the
// .doug/ directory with all required files. w receives status messages;
// maxRetries, maxIterations, and kbEnabled are pre-resolved by the caller
// (from flags, prompts, or defaults).
func doInitProject(w io.Writer, dir string, force bool, buildSystem string, selectedAgents []string, noGitInit bool, maxRetries, maxIterations int, kbEnabled bool) error {
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

	// Startup header.
	writeln(w, "")
	writef(w, "Initializing doug project in %s\n", dir)
	writeln(w, "")

	// Validate explicit buildSystem before doing any file work.
	if buildSystem != "" {
		switch buildSystem {
		case "go", "npm", "pnpm", "static":
		default:
			return fmt.Errorf("unsupported build system %q: must be one of: go, npm, pnpm, static", buildSystem)
		}
	}

	// Determine the build system: value passed in > auto-detect > fallback.
	bs := buildSystem
	if bs == "" {
		bs = config.DetectBuildSystem(dir)
	}
	if bs == "" {
		bs = "go"
	}

	// Warn on unknown agent names before doing any work.
	for _, name := range selectedAgents {
		if _, ok := agentRegistry[name]; !ok {
			log.Warning(fmt.Sprintf("unknown agent %q — no skills directory defined; skipping skill copy for this agent", name))
		}
	}

	primaryAgent := "claude"
	if len(selectedAgents) > 0 {
		primaryAgent = strings.ToLower(strings.TrimSpace(selectedAgents[0]))
	}

	type fileSpec struct {
		path    string
		content string
	}
	specs := []fileSpec{
		{filepath.Join(dougDir, "doug.yaml"), dougYAMLContent(bs, primaryAgent, maxRetries, maxIterations, kbEnabled)},
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
		relPath, _ := filepath.Rel(dir, spec.path)
		writef(w, "  ✓ %s\n", relPath)
		log.Success(fmt.Sprintf("created %s", spec.path))
	}

	// Copy embedded init/ templates into the target project.
	if err := copyInitTemplates(w, dir, force, selectedAgents, bs); err != nil {
		return err
	}

	// Create docs/kb/ directory (silent if already exists).
	kbDir := filepath.Join(dir, "docs", "kb")
	if _, statErr := os.Stat(kbDir); os.IsNotExist(statErr) {
		if err := os.MkdirAll(kbDir, 0o755); err != nil {
			return fmt.Errorf("create docs/kb directory: %w", err)
		}
		writef(w, "  ✓ docs/kb/\n")
		log.Success("created docs/kb/")
	}

	// Create CHANGELOG.md at project root if it does not already exist.
	// Never overwrite an existing CHANGELOG.md — it is user-maintained.
	changelogPath := filepath.Join(dir, "CHANGELOG.md")
	if _, statErr := os.Stat(changelogPath); os.IsNotExist(statErr) {
		if err := state.AtomicWrite(changelogPath, []byte(changelogContent())); err != nil {
			return fmt.Errorf("write CHANGELOG.md: %w", err)
		}
		writef(w, "  ✓ CHANGELOG.md\n")
		log.Success("created CHANGELOG.md")
	}

	if !noGitInit {
		gitDir := filepath.Join(dir, ".git")
		if _, statErr := os.Stat(gitDir); os.IsNotExist(statErr) {
			gitCmd := exec.Command("git", "init", dir)
			if out, err := gitCmd.CombinedOutput(); err != nil {
				log.Warning(fmt.Sprintf("git init failed: %v\n%s", err, out))
			} else {
				log.Success("initialized git repository")
			}
		}
	}

	writeln(w, "")
	writeln(w, "Done. Next steps:")
	writeln(w, "  1. Edit .doug/PRD.md     — describe your project")
	writeln(w, "  2. Edit .doug/tasks.yaml — define your tasks")
	writeln(w, "  3. Run: doug run")
	writeln(w, "")
	log.Info("project initialized")
	return nil
}

// copyInitTemplates builds an install plan from the embedded init/ FS and
// executes it against dir. The plan captures all routing rules (which files go
// where, which merge strategy applies) as explicit installEntry values so that
// routing and execution are separately testable.
func copyInitTemplates(w io.Writer, dir string, force bool, selectedAgents []string, buildSystem string) error {
	agentSelected := make(map[string]bool)
	for _, name := range selectedAgents {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			agentSelected[name] = true
		}
	}

	entries, err := buildInstallPlan(dir, agentSelected, buildSystem)
	if err != nil {
		return err
	}
	return executeInstallPlan(w, dir, entries, force)
}

// injectBuildSystemPermissions appends build-system-specific Bash permissions
// to the "permissions.allow" array in the settings.json template. Returns the
// template unchanged if bs is empty, not in the BuildSystems registry, or has
// no permissions defined. Returns an error only when the template JSON is malformed.
func injectBuildSystemPermissions(template []byte, bs string) ([]byte, error) {
	info, ok := config.BuildSystems[bs]
	if !ok || len(info.Permissions) == 0 {
		return template, nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(template, &obj); err != nil {
		return nil, err
	}

	// Navigate/create permissions.allow.
	permsVal := obj["permissions"]
	permsMap, _ := permsVal.(map[string]interface{})
	if permsMap == nil {
		permsMap = make(map[string]interface{})
		obj["permissions"] = permsMap
	}

	allowVal := permsMap["allow"]
	allowArr, _ := allowVal.([]interface{})

	toAdd := make([]interface{}, len(info.Permissions))
	for i, p := range info.Permissions {
		toAdd[i] = p
	}

	merged, _ := mergeStringArrays(allowArr, toAdd)
	permsMap["allow"] = merged

	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// dougYAMLContent returns the .doug/doug.yaml file content with inline YAML comments,
// the detected (or specified) build system pre-filled, and the selected primary agent's
// mode-specific commands as the active run/plan/scaffold commands (others commented out).
// maxRetries, maxIterations, and kbEnabled are written from the provided values (typically
// chosen interactively during init or set to defaults for non-interactive runs).
func dougYAMLContent(buildSystem, primaryAgent string, maxRetries, maxIterations int, kbEnabled bool) string {
	agent := primaryAgent
	if _, ok := agentRegistry[agent]; !ok {
		agent = "claude"
	}

	activeInfo := agentRegistry[agent]
	activeLines := []string{
		fmt.Sprintf("run_agent_command: '%s' # Command used for doug run and post-epic KB synthesis", activeInfo.runCommand),
		fmt.Sprintf("plan_agent_command: '%s' # Command used for interactive doug plan sessions", activeInfo.planCommand),
		fmt.Sprintf("scaffold_agent_command: '%s' # Command used for doug scaffold", activeInfo.scaffoldCommand),
	}

	agentBlock := strings.Join(activeLines, "\n")

	kbStr := "true"
	if !kbEnabled {
		kbStr = "false"
	}

	return fmt.Sprintf(`# doug.yaml — orchestrator configuration
# See https://github.com/robertgumeny/doug for documentation.
%s
build_system: %s # Build system: go | npm | pnpm (auto-detected by init; override here)
max_retries: %d # Max FAILURE outcomes before a task is BLOCKED
max_iterations: %d # Max loop iterations before the run exits
kb_enabled: %s # If false, skip KB synthesis task after features complete
agent_heartbeat_seconds: 30 # Periodic liveness log cadence while agent runs (0 disables)
`, agentBlock, buildSystem, maxRetries, maxIterations, kbStr)
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

### Added

### Changed

### Fixed

### Removed
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

// slugify converts a string to a lowercase, hyphen-separated slug containing
// only alphanumeric characters and hyphens. Consecutive separators are collapsed.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		case r == '-' || r == '_' || r == ' ':
			if !prevHyphen {
				b.WriteRune('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// generateProjectID returns a stable project identifier of the form
// "<slug>-<6hexchars>", where the slug is derived from the project directory
// name and the suffix is randomly generated.
func generateProjectID(dirName string) string {
	slug := slugify(dirName)
	if slug == "" {
		slug = "project"
	}
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return slug + "-000000"
	}
	return fmt.Sprintf("%s-%x", slug, b)
}

// generateProjectName returns a human-readable display name derived from the
// project directory name by title-casing each word after splitting on hyphens,
// underscores, and spaces.
func generateProjectName(dirName string) string {
	s := strings.NewReplacer("-", " ", "_", " ").Replace(dirName)
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
