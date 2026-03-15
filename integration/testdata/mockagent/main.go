// mockagent is a minimal stub agent for integration testing.
//
// It reads .doug/ACTIVE_TASK.md from the working directory, fills in the
// Agent Result block with a canned SUCCESS outcome, and writes the updated
// content back to ACTIVE_TASK.md. Doug's ParseSessionResult then reads the
// result from that file directly.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	activeTaskPath := filepath.Join(".doug", "ACTIVE_TASK.md")
	data, err := os.ReadFile(activeTaskPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mockagent: read ACTIVE_TASK.md: %v\n", err)
		os.Exit(1)
	}

	content := string(data)

	// Replace the empty outcome field with SUCCESS and fill in changelog entry.
	content = strings.Replace(content, `outcome: ""`, `outcome: "SUCCESS"`, 1)
	content = strings.Replace(content, `changelog_entry: ""`, `changelog_entry: "smoke test task completed"`, 1)

	if err := os.WriteFile(activeTaskPath, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "mockagent: write ACTIVE_TASK.md: %v\n", err)
		os.Exit(1)
	}
}
