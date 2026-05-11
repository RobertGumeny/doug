// Package interactive provides the shared interactive command UX foundation for
// Doug CLI commands. It owns the terminal interaction abstraction so command
// packages can drive prompts without importing Bubble Tea types directly.
//
// Responsibilities:
//   - Expose stable prompt methods (SelectOne, Confirm, Text) via the Prompter interface.
//   - Route to the Bubble Tea runtime when a real terminal is available.
//   - Fall back to plain line-reading for non-interactive environments (CI, pipes, tests).
//
// Non-responsibilities:
//   - Command-specific business logic (belongs in cmd/ or the relevant internal package).
//   - State, config, or YAML I/O (belongs in internal/state and internal/config).
//   - Log output (belongs in internal/log).
//   - Full-screen TUI layouts — this package drives simple single-prompt flows, not
//     full-screen applications.
package interactive

import (
	"io"
	"os"

	"github.com/robertgumeny/doug/internal/prompt"
)

// Prompter is the public integration seam for Doug commands that need interactive
// user input. Command packages depend only on this interface; they never import
// Bubble Tea types directly.
type Prompter interface {
	// SelectOne presents a numbered list and returns the chosen index and value.
	// Returns an error only when options is empty.
	SelectOne(question string, options []string, defaultIdx int) (int, string, error)

	// Confirm presents a yes/no question and returns the answer.
	Confirm(question string, defaultYes bool) (bool, error)

	// Text presents a free-text prompt and returns the entered string.
	// Returns defaultVal when the user submits an empty response.
	Text(question string, defaultVal string) (string, error)

	// Compose presents a multi-line text entry prompt with the given header.
	// The user submits with Ctrl+D. Returns defaultVal when no text is entered.
	Compose(header string, defaultVal string) (string, error)
}

// IsInteractive reports whether the current process is running in an interactive
// terminal. When false, New() returns the plain fallback prompter and all prompt
// methods return their default values without reading from the terminal.
//
// Call this once per command invocation when the command must either warn the
// user or bail out before the first prompt.
func IsInteractive() bool {
	return prompt.IsTTY(os.Stdin)
}

// New returns a Prompter appropriate for the current environment. When os.Stdin
// is connected to a terminal, a Bubble Tea-backed prompter is returned. Otherwise
// a plain line-reader fallback is used (suitable for CI and piped input).
func New() Prompter {
	if prompt.IsTTY(os.Stdin) {
		return &teaPrompter{}
	}
	return &fallbackPrompter{w: os.Stdout, r: os.Stdin}
}

// NewWithIO returns a Prompter that reads from r and writes to w. isTTY controls
// whether the interactive (Bubble Tea) path is taken. Use this constructor in
// tests — production code should call New().
func NewWithIO(w io.Writer, r io.Reader, isTTY bool) Prompter {
	if isTTY {
		return &teaPrompter{}
	}
	return &fallbackPrompter{w: w, r: r}
}

// fallbackPrompter is the plain line-reader implementation used in non-interactive
// environments. It delegates to internal/prompt so the behaviour is identical to
// the existing command helpers.
type fallbackPrompter struct {
	w io.Writer
	r io.Reader
}

func (p *fallbackPrompter) SelectOne(question string, options []string, defaultIdx int) (int, string, error) {
	return prompt.SelectOne(p.w, p.r, false, question, options, defaultIdx)
}

func (p *fallbackPrompter) Confirm(question string, defaultYes bool) (bool, error) {
	return prompt.Confirm(p.w, p.r, false, question, defaultYes)
}

func (p *fallbackPrompter) Text(question string, defaultVal string) (string, error) {
	return prompt.Text(p.w, p.r, false, question, defaultVal)
}

func (p *fallbackPrompter) Compose(_ string, defaultVal string) (string, error) {
	return defaultVal, nil
}
