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

// UpdateChangelog reads the CHANGELOG file at path, finds the ## [Unreleased]
// block, locates the subsection corresponding to taskType within it, and
// inserts entry as a bullet point immediately after the section header.
//
// Behavior:
//   - Returns an error if ## [Unreleased] is absent from the file.
//   - Subsection search (### Added, ### Fixed, ### Changed) is scoped to the
//     ## [Unreleased] block only; matching headers in released version sections
//     are ignored.
//   - Returns an error if the target subsection is not found within
//     ## [Unreleased].
//   - Is idempotent: if "- {entry}" already exists within ## [Unreleased],
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

	// Locate ## [Unreleased] block.
	const unreleasedHeader = "## [Unreleased]"
	unreleasedIdx := strings.Index(content, unreleasedHeader)
	if unreleasedIdx == -1 {
		return fmt.Errorf("changelog: %q section not found in %q", unreleasedHeader, path)
	}

	// Bound the Unreleased block: from the header line to the next "## " section
	// (or end of file). We search from the character after the header to avoid
	// matching the header itself.
	afterUnreleased := unreleasedIdx + len(unreleasedHeader)
	nextSectionRel := strings.Index(content[afterUnreleased:], "\n## ")
	var unreleasedEnd int
	if nextSectionRel == -1 {
		unreleasedEnd = len(content)
	} else {
		// +1 to include the leading newline before "## " as part of the boundary.
		unreleasedEnd = afterUnreleased + nextSectionRel + 1
	}
	unreleasedBlock := content[unreleasedIdx:unreleasedEnd]

	// Deduplication: if the bullet already exists within ## [Unreleased], skip.
	bullet := "- " + entry
	if strings.Contains(unreleasedBlock, bullet) {
		return nil
	}

	// Locate the subsection header within the Unreleased block.
	headerRelIdx := strings.Index(unreleasedBlock, header)
	if headerRelIdx == -1 {
		return fmt.Errorf("changelog: section %q not found within ## [Unreleased] in %q", header, path)
	}

	// Convert to absolute index in the full file content.
	absHeaderIdx := unreleasedIdx + headerRelIdx
	afterHeader := absHeaderIdx + len(header)
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
