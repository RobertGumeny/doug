package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/interactive"
	"github.com/robertgumeny/doug/internal/log"
)

// initWorkflowOptions holds the flag values passed to runInitWorkflow.
type initWorkflowOptions struct {
	force       bool
	buildSystem string            // explicit --build-system flag; empty means auto-detect
	agents      string            // comma-separated --agents flag; empty means interactive or default
	noGitInit   bool
	prompter    interactive.Prompter // optional; nil means derive from w/r/isTTY
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

	// Resolve the Prompter for agent selection. Callers may inject one via opts
	// (useful in tests); otherwise derive from the stream and TTY state.
	p := opts.prompter
	if p == nil {
		p = interactive.NewWithIO(w, br, isTTY)
	}

	// Resolve selected agents: flag > interactive TTY > default.
	var selectedAgents []string
	if opts.agents != "" {
		for _, a := range strings.Split(opts.agents, ",") {
			if a = strings.TrimSpace(a); a != "" {
				selectedAgents = append(selectedAgents, a)
			}
		}
	} else if isTTY {
		selectedAgents = selectAgentsInteractive(p)
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
			bs = selectBuildSystemInteractive(p, bs)
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

// selectAgentsInteractive uses the shared Prompter to select the primary agent
// and optionally confirm additional agents. Defaults to ["claude"] on error.
func selectAgentsInteractive(p interactive.Prompter) []string {
	options := []string{"claude", "codex", "gemini"}

	_, primary, err := p.SelectOne("Which agent are you using?", options, 0)
	if err != nil {
		return []string{"claude"}
	}

	selected := []string{primary}

	for _, opt := range options {
		if opt == primary {
			continue
		}
		add, err := p.Confirm(fmt.Sprintf("Also install skills for %s?", opt), false)
		if err == nil && add {
			selected = append(selected, opt)
		}
	}

	return selected
}

// selectBuildSystemInteractive uses the shared Prompter to select the build
// system. The detected value (if any) is presented as the default. Falls back
// to "go" when nothing is detected or the detected value is not in the options
// list.
func selectBuildSystemInteractive(p interactive.Prompter, detected string) string {
	options := []string{"go", "npm", "pnpm", "static"}
	defaultIdx := 0
	for i, o := range options {
		if o == detected {
			defaultIdx = i
			break
		}
	}
	_, selected, err := p.SelectOne("Build system:", options, defaultIdx)
	if err != nil {
		return options[defaultIdx]
	}
	return selected
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
