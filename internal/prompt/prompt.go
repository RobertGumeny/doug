// Package prompt provides reusable interactive prompt helpers for CLI commands.
// Each function accepts an io.Writer for output and an io.Reader for input so
// they are testable without a real terminal. The isTTY parameter controls
// whether the prompt is displayed; when false the function returns the default
// value silently, satisfying the non-interactive / flag-provided-value path.
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// IsTTY reports whether f (typically os.Stdin) is connected to an interactive
// terminal. Returns false if the stat call fails.
func IsTTY(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// SelectOne displays a numbered list of options on w and reads a selection from r.
//
// When isTTY is false the function returns (defaultIdx, options[defaultIdx], nil)
// without writing to w or reading from r, allowing flag-provided values to bypass
// the prompt.
//
// On empty input the default is returned. On out-of-range or non-numeric input
// the default is also returned (no error).
func SelectOne(w io.Writer, r io.Reader, isTTY bool, question string, options []string, defaultIdx int) (int, string, error) {
	if len(options) == 0 {
		return 0, "", fmt.Errorf("prompt.SelectOne: options list must not be empty")
	}
	if defaultIdx < 0 || defaultIdx >= len(options) {
		defaultIdx = 0
	}
	if !isTTY {
		return defaultIdx, options[defaultIdx], nil
	}

	writef(w, "%s\n", question)
	for i, opt := range options {
		marker := "[ ]"
		if i == defaultIdx {
			marker = "[x]"
		}
		writef(w, "  %d. %s %s\n", i+1, marker, opt)
	}
	writef(w, "Selection (1-%d, or press Enter for %s): ", len(options), options[defaultIdx])

	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && line == "" {
		return defaultIdx, options[defaultIdx], nil
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultIdx, options[defaultIdx], nil
	}
	n, convErr := strconv.Atoi(line)
	if convErr != nil || n < 1 || n > len(options) {
		return defaultIdx, options[defaultIdx], nil
	}
	idx := n - 1
	return idx, options[idx], nil
}

// Confirm displays a yes/no question on w and reads the answer from r.
//
// When isTTY is false the function returns defaultYes without prompting.
// Accepted affirmative inputs: "y", "yes". Accepted negative inputs: "n", "no".
// Empty input returns defaultYes. Any other input returns defaultYes (no error).
func Confirm(w io.Writer, r io.Reader, isTTY bool, question string, defaultYes bool) (bool, error) {
	if !isTTY {
		return defaultYes, nil
	}

	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}
	writef(w, "%s %s: ", question, hint)

	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && line == "" {
		return defaultYes, nil
	}
	line = strings.ToLower(strings.TrimSpace(line))
	switch line {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return defaultYes, nil
	}
}

// Text displays a free-text prompt on w and reads a line from r.
//
// When isTTY is false the function returns defaultVal without prompting.
// Empty input returns defaultVal.
func Text(w io.Writer, r io.Reader, isTTY bool, question string, defaultVal string) (string, error) {
	if !isTTY {
		return defaultVal, nil
	}

	if defaultVal != "" {
		writef(w, "%s [%s]: ", question, defaultVal)
	} else {
		writef(w, "%s: ", question)
	}

	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && line == "" {
		return defaultVal, nil
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}
