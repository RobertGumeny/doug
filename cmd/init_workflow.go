package cmd

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
)

// initWorkflowOptions holds the flag values passed to runInitWorkflow.
type initWorkflowOptions struct {
	force       bool
	buildSystem string // explicit --build-system flag; empty means auto-detect
	agents      string // comma-separated --agents flag; empty means interactive or default
	noGitInit   bool
}

// runInitWorkflow is the top-level init orchestration entry point. It resolves
// agent selection, build system, and key configuration values from flags or
// interactive prompts, then delegates all file I/O to doInitProject.
//
// w and r are the output/input streams used for status messages and prompts.
// isTTY controls whether interactive prompts are displayed; when false all
// values fall back to detected or hardcoded defaults.
func runInitWorkflow(w io.Writer, r io.Reader, isTTY bool, dir string, opts initWorkflowOptions) error {
	// Pre-wrap r in a single bufio.Reader so that all prompt helpers share the
	// same buffered stream. bufio.NewReader is a no-op when given a *bufio.Reader
	// of sufficient size, so each helper ends up reading from the same buffer.
	br := bufio.NewReader(r)

	// Resolve selected agents: flag > interactive TTY > default.
	var selectedAgents []string
	if opts.agents != "" {
		for _, a := range strings.Split(opts.agents, ",") {
			if a = strings.TrimSpace(a); a != "" {
				selectedAgents = append(selectedAgents, a)
			}
		}
	} else if isTTY {
		selectedAgents = selectAgentsInteractive(w, br)
	} else {
		selectedAgents = []string{"claude"}
	}
	if len(selectedAgents) == 0 {
		selectedAgents = []string{"claude"}
	}

	// Resolve build system: flag > auto-detect > prompt (TTY) > fallback.
	bs := opts.buildSystem
	if bs == "" {
		bs = config.DetectBuildSystem(dir)
	}
	claudeSelected := false
	for _, a := range selectedAgents {
		if strings.ToLower(strings.TrimSpace(a)) == "claude" {
			claudeSelected = true
			break
		}
	}
	if opts.buildSystem == "" {
		if isTTY {
			bs = promptBuildSystemSelection(w, br, bs)
		} else if bs == "" {
			if claudeSelected {
				log.Warning("no build system detected and stdin is not a TTY — defaulting to 'go'; " +
					"set --build-system flag or add a marker file (go.mod, package.json, pnpm-workspace.yaml) to auto-detect")
			}
			bs = "go"
		}
	}
	if bs == "" {
		bs = "go"
	}

	// Resolve key config settings: prompt on TTY, otherwise use defaults.
	maxRetries, maxIterations, kbEnabled := 3, 10, true
	if isTTY {
		maxRetries = promptIntValue(w, br, "max_retries", maxRetries)
		maxIterations = promptIntValue(w, br, "max_iterations", maxIterations)
		kbEnabled = promptBoolValue(w, br, "kb_enabled", kbEnabled)
	}

	return doInitProject(w, dir, opts.force, bs, selectedAgents, opts.noGitInit, maxRetries, maxIterations, kbEnabled)
}

// selectAgentsInteractive displays a numbered agent selection menu and returns
// the selected agent names. Defaults to ["claude"] on empty or invalid input.
func selectAgentsInteractive(w io.Writer, r io.Reader) []string {
	options := []string{"claude", "codex", "gemini"}

	writeln(w, "Which agent(s) are you using? (comma-separated numbers, or press Enter for Claude)")
	for i, name := range options {
		marker := "[ ]"
		if i == 0 {
			marker = "[x]"
		}
		writef(w, "  %d. %s %s\n", i+1, marker, name)
	}
	writef(w, "Selection (e.g. 1,2): ")

	input, err := bufio.NewReader(r).ReadString('\n')
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
		selected = append(selected, options[n-1])
	}
	if len(selected) == 0 {
		return []string{"claude"}
	}
	return selected
}

// promptBuildSystemSelection displays a numbered build system menu and returns
// the selected value. Defaults to detected (or "go") on empty/invalid input.
func promptBuildSystemSelection(w io.Writer, r io.Reader, detected string) string {
	options := []string{"go", "npm", "pnpm", "static"}
	defaultBS := detected
	if defaultBS == "" {
		defaultBS = "go"
	}
	writeln(w, "Build system:")
	for i, name := range options {
		if name == defaultBS {
			writef(w, "  %d. %s (default)\n", i+1, name)
		} else {
			writef(w, "  %d. %s\n", i+1, name)
		}
	}
	writef(w, "Selection (1-%d, or press Enter for %s): ", len(options), defaultBS)

	input, err := bufio.NewReader(r).ReadString('\n')
	if err != nil || strings.TrimSpace(input) == "" {
		return defaultBS
	}
	n, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || n < 1 || n > len(options) {
		return defaultBS
	}
	return options[n-1]
}

// promptIntValue displays a labelled integer prompt and returns the entered value.
// Returns defaultVal on empty input, read error, or non-numeric/negative input.
func promptIntValue(w io.Writer, r io.Reader, label string, defaultVal int) int {
	writef(w, "%s [%d]: ", label, defaultVal)
	input, err := bufio.NewReader(r).ReadString('\n')
	if err != nil || strings.TrimSpace(input) == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}

// promptBoolValue displays a labelled boolean prompt and returns the entered value.
// Accepts true/false/yes/no/y/n/1/0; returns defaultVal on empty input or
// unrecognised value.
func promptBoolValue(w io.Writer, r io.Reader, label string, defaultVal bool) bool {
	defaultStr := "true"
	if !defaultVal {
		defaultStr = "false"
	}
	writef(w, "%s [%s]: ", label, defaultStr)
	input, err := bufio.NewReader(r).ReadString('\n')
	if err != nil || strings.TrimSpace(input) == "" {
		return defaultVal
	}
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "true", "yes", "y", "1":
		return true
	case "false", "no", "n", "0":
		return false
	default:
		return defaultVal
	}
}
