// Package changelog provides idempotent CHANGELOG.md update functionality
// using pure Go string manipulation — no sed, awk, or exec.Command.
package changelog

import (
	"fmt"
	"os"
	"strings"
)

// sectionHeader maps a task type to its CHANGELOG section header.
// Returns an empty string for unknown task types.
func sectionHeader(taskType string) string {
	switch taskType {
	case "feature":
		return "### Added"
	case "bugfix":
		return "### Fixed"
	case "documentation":
		return "### Changed"
	default:
		return ""
	}
}

// UpdateChangelog reads the CHANGELOG file at path, finds the section
// corresponding to taskType, and inserts entry as a bullet point immediately
// after the section header.
//
// Behavior:
//   - Returns a non-fatal error if the taskType is unknown or the section
//     header is not found in the file. Callers should log this as a warning
//     rather than failing the task.
//   - Is idempotent: if "- {entry}" already exists anywhere in the file,
//     the file is left unchanged and nil is returned.
//   - Uses pure Go string manipulation; no external commands are invoked.
func UpdateChangelog(path, entry, taskType string) error {
	header := sectionHeader(taskType)
	if header == "" {
		return fmt.Errorf("changelog: unknown task type %q; expected feature, bugfix, or documentation", taskType)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("changelog: read %q: %w", path, err)
	}
	content := string(data)

	// Deduplication: if the bullet already exists, return without modification.
	bullet := "- " + entry
	if strings.Contains(content, bullet) {
		return nil
	}

	// Locate the section header.
	headerIdx := strings.Index(content, header)
	if headerIdx == -1 {
		return fmt.Errorf("changelog: section %q not found in %q", header, path)
	}

	// Find the end of the header line (the newline character after the header).
	afterHeader := headerIdx + len(header)
	nlIdx := strings.Index(content[afterHeader:], "\n")

	var insertAt int
	if nlIdx == -1 {
		// Header is at the very end of the file with no trailing newline.
		content = content + "\n" + bullet + "\n"
		return os.WriteFile(path, []byte(content), 0644)
	}
	// Insert right after the newline that terminates the header line.
	insertAt = afterHeader + nlIdx + 1

	updated := content[:insertAt] + bullet + "\n" + content[insertAt:]
	return os.WriteFile(path, []byte(updated), 0644)
}
