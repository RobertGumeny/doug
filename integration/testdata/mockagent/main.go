// mockagent is a minimal stub agent for integration testing.
//
// It reads logs/ACTIVE_TASK.md from the working directory to find the session
// file path, writes a canned SUCCESS result there, and exits 0.
// It is compiled by integration/smoke_test.go TestMain and invoked by the real
// doug orchestrator in place of claude.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	data, err := os.ReadFile(filepath.Join("logs", "ACTIVE_TASK.md"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mockagent: read ACTIVE_TASK.md: %v\n", err)
		os.Exit(1)
	}

	var sessionPath string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		const prefix = "**Session File**: "
		if strings.HasPrefix(line, prefix) {
			sessionPath = strings.TrimPrefix(line, prefix)
			break
		}
	}

	if sessionPath == "" {
		fmt.Fprintln(os.Stderr, "mockagent: could not find Session File path in ACTIVE_TASK.md")
		os.Exit(1)
	}

	result := "---\noutcome: \"SUCCESS\"\nchangelog_entry: \"smoke test task completed\"\ndependencies_added: []\n---\n"
	if err := os.WriteFile(sessionPath, []byte(result), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "mockagent: write session file %s: %v\n", sessionPath, err)
		os.Exit(1)
	}
}
