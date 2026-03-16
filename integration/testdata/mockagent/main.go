// mockagent is a configurable stub agent for integration testing.
//
// Default behaviour (no script file): reads .doug/ACTIVE_TASK.md from the
// working directory, fills in the Agent Result block with a SUCCESS outcome,
// and writes the updated content back. This is backward-compatible with the
// original smoke test.
//
// Scenario-driven behaviour: if .doug/mockagent-script exists, each line
// specifies the action for the corresponding call (0-indexed). Supported
// actions:
//
//	SUCCESS              — write SUCCESS outcome (default)
//	FAILURE              — write FAILURE outcome
//	BUG                  — write BUG outcome and create .doug/ACTIVE_BUG.md
//	SUCCESS+BREAK_BUILD  — write SUCCESS outcome and corrupt main.go so the
//	                       subsequent build verification fails
//
// The call counter is persisted in .doug/mockagent-calls so it survives across
// separate doug run invocations.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	dougDir := ".doug"

	// Determine which call this is (0-indexed) and increment the counter.
	callCount := readInt(filepath.Join(dougDir, "mockagent-calls"))
	writeInt(filepath.Join(dougDir, "mockagent-calls"), callCount+1)

	// Load the script; default to SUCCESS when absent or exhausted.
	script := readLines(filepath.Join(dougDir, "mockagent-script"))
	action := "SUCCESS"
	if callCount < len(script) {
		action = script[callCount]
	}

	// Split "OUTCOME+MODIFIER" into its parts.
	parts := strings.SplitN(action, "+", 2)
	outcome := strings.TrimSpace(parts[0])
	modifier := ""
	if len(parts) > 1 {
		modifier = strings.TrimSpace(parts[1])
	}

	// Write the outcome into the Agent Result block of ACTIVE_TASK.md.
	activeTaskPath := filepath.Join(dougDir, "ACTIVE_TASK.md")
	data, err := os.ReadFile(activeTaskPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mockagent: read ACTIVE_TASK.md: %v\n", err)
		os.Exit(1)
	}
	content := string(data)
	content = strings.Replace(content, `outcome: ""`, `outcome: "`+outcome+`"`, 1)
	content = strings.Replace(content, `changelog_entry: ""`, `changelog_entry: "smoke test task completed"`, 1)
	if err := os.WriteFile(activeTaskPath, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "mockagent: write ACTIVE_TASK.md: %v\n", err)
		os.Exit(1)
	}

	// Outcome-specific side effects.
	if outcome == "BUG" {
		bugContent := "# Active Bug Report\n\nBug discovered during smoke test.\n"
		if err := os.WriteFile(filepath.Join(dougDir, "ACTIVE_BUG.md"), []byte(bugContent), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "mockagent: write ACTIVE_BUG.md: %v\n", err)
			os.Exit(1)
		}
	}

	// Modifier side effects.
	if modifier == "BREAK_BUILD" {
		// Write syntactically invalid Go to main.go so the subsequent
		// build verification step in HandleSuccess fails.
		broken := "package main\n\nfunc main() { INVALID_GO_SYNTAX\n"
		if err := os.WriteFile("main.go", []byte(broken), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "mockagent: write invalid main.go: %v\n", err)
			os.Exit(1)
		}
	}
}

// readInt reads an integer from path; returns 0 on any error.
func readInt(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return n
}

// writeInt writes n as a decimal string to path.
func writeInt(path string, n int) {
	_ = os.WriteFile(path, []byte(strconv.Itoa(n)), 0o644)
}

// readLines reads non-empty trimmed lines from path; returns nil on any error.
func readLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
