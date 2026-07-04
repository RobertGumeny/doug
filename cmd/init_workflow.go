package cmd

import (
	"bufio"
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
	buildSystem string // explicit --build-system flag; empty means auto-detect
	noGitInit   bool
	prompter    interactive.Prompter // optional; nil means derive from w/r/isTTY
}

const (
	initDefaultMaxRetries    = 3
	initDefaultMaxIterations = 10
	initDefaultKBEnabled     = true
)

// runInitWorkflow is the top-level init orchestration entry point. It resolves
// the build system and key configuration values from flags or interactive
// prompts, then delegates all file I/O to doInitProject.
//
// w and r are the output/input streams used for status messages and prompts.
// isTTY controls whether interactive prompts are displayed; when false all
// values fall back to detected or hardcoded defaults.
func runInitWorkflow(w io.Writer, r io.Reader, isTTY bool, dir string, opts initWorkflowOptions) error {
	// Pre-wrap r in a single bufio.Reader so that all prompt helpers share the
	// same buffered stream. bufio.NewReader is a no-op when given a *bufio.Reader
	// of sufficient size, so each helper ends up reading from the same buffer.
	br := bufio.NewReader(r)

	// Resolve the Prompter for interactive prompts. Callers may inject one via
	// opts (useful in tests); otherwise derive from the stream and TTY state.
	p := opts.prompter
	if p == nil {
		p = interactive.NewWithIO(w, br, isTTY)
	}

	// Resolve build system: flag > auto-detect > prompt (TTY) > fallback.
	bs := opts.buildSystem
	if bs == "" {
		bs = config.DetectBuildSystem(dir)
	}
	if opts.buildSystem == "" {
		if isTTY {
			bs = selectBuildSystemInteractive(p, bs)
		} else if bs == "" {
			log.Warning("no build system detected and stdin is not a TTY — defaulting to 'go'; " +
				"set --build-system flag or add a marker file (go.mod, package.json, pnpm-workspace.yaml) to auto-detect")
			bs = "go"
		}
	}
	if bs == "" {
		bs = "go"
	}

	// Resolve key config settings: prompt on TTY, otherwise use defaults.
	maxRetries, maxIterations, kbEnabled := initDefaultMaxRetries, initDefaultMaxIterations, initDefaultKBEnabled
	if isTTY {
		maxRetries = promptConfigInt(p, "max_retries — max FAILURE outcomes before a task is BLOCKED", maxRetries)
		maxIterations = promptConfigInt(p, "max_iterations — max orchestrator loop iterations before Doug stops", maxIterations)
		kbEnabled, _ = p.Confirm("kb_enabled — synthesize knowledge-base updates after feature work", kbEnabled)
	}

	return doInitProject(w, dir, opts.force, bs, opts.noGitInit, maxRetries, maxIterations, kbEnabled)
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

// promptConfigInt prompts for an integer config value via the shared Prompter.
// Returns defaultVal on empty input, parse error, or negative value.
func promptConfigInt(p interactive.Prompter, label string, defaultVal int) int {
	s, err := p.Text(label, strconv.Itoa(defaultVal))
	if err != nil {
		return defaultVal
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}
