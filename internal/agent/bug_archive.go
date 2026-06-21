package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/types"
)

// ErrUnknownBugSeverity is returned when a BugPayload carries a severity value
// that is not one of the four accepted values (critical, high, medium, low).
type ErrUnknownBugSeverity struct{ Value types.BugSeverity }

func (e *ErrUnknownBugSeverity) Error() string {
	return fmt.Sprintf("unknown bug severity %q: must be one of critical, high, medium, low", e.Value)
}

// ErrUnknownBugStatus is returned when a BugPayload carries a status value
// that is not one of the four accepted values (open, investigating, fixed, wont_fix).
type ErrUnknownBugStatus struct{ Value types.BugStatus }

func (e *ErrUnknownBugStatus) Error() string {
	return fmt.Sprintf("unknown bug status %q: must be one of open, investigating, fixed, wont_fix", e.Value)
}

// WriteBugArchive writes a structured bug archive under
//
//	<logsDir>/bugs/<epicID>/bug-<discoveredByTask>.md
//
// The file begins with a YAML frontmatter block that contains bug_id,
// discovered_by_task, timestamp, severity, and status, followed by the raw
// markdown body from payload.Body.
//
// Validation is performed before any file is written:
//   - Severity must be one of: critical, high, medium, low.
//   - Status must be one of: open, investigating, fixed, wont_fix.
//
// If payload.Timestamp is empty, the current UTC time in RFC3339 format is
// stamped automatically.
//
// Versioned filenames: when the canonical archive path already exists,
// successive writes produce sibling files: bug-<taskID>-v2.md, -v3.md, etc.
// This preserves history without overwriting previous archives.
//
// The returned string is the absolute path of the archive file that was written.
func WriteBugArchive(logsDir, epicID string, payload types.BugPayload) (string, error) {
	if err := validateBugSeverity(payload.Severity); err != nil {
		return "", err
	}
	if err := validateBugStatus(payload.Status); err != nil {
		return "", err
	}

	if payload.Timestamp == "" {
		payload.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	content, err := renderBugArchive(payload)
	if err != nil {
		return "", fmt.Errorf("render bug archive frontmatter: %w", err)
	}

	archiveDir := filepath.Join(logsDir, "bugs", epicID)
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return "", fmt.Errorf("create bug archive directory %s: %w", archiveDir, err)
	}

	dst, err := nextBugArchivePath(archiveDir, payload.DiscoveredByTask)
	if err != nil {
		return "", fmt.Errorf("resolve bug archive path: %w", err)
	}

	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write bug archive temp file: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", fmt.Errorf("rename bug archive temp file: %w", err)
	}

	return dst, nil
}

// validateBugSeverity returns ErrUnknownBugSeverity for any value outside the
// canonical set. Canonicalization (lowercase) is applied before comparison.
func validateBugSeverity(s types.BugSeverity) error {
	switch types.BugSeverity(strings.ToLower(string(s))) {
	case types.BugSeverityCritical, types.BugSeverityHigh, types.BugSeverityMedium, types.BugSeverityLow:
		return nil
	default:
		return &ErrUnknownBugSeverity{Value: s}
	}
}

// validateBugStatus returns ErrUnknownBugStatus for any value outside the
// canonical set. Canonicalization (lowercase) is applied before comparison.
func validateBugStatus(s types.BugStatus) error {
	switch types.BugStatus(strings.ToLower(string(s))) {
	case types.BugStatusOpen, types.BugStatusInvestigating, types.BugStatusFixed, types.BugStatusWontFix:
		return nil
	default:
		return &ErrUnknownBugStatus{Value: s}
	}
}

// renderBugArchive produces the on-disk content of a bug archive file:
// a YAML frontmatter block followed by the raw body.
func renderBugArchive(payload types.BugPayload) (string, error) {
	// Marshal using a field-ordered struct so the frontmatter key order is
	// deterministic: bug_id → discovered_by_task → timestamp → severity → status.
	type frontmatter struct {
		BugID            string           `yaml:"bug_id"`
		DiscoveredByTask string           `yaml:"discovered_by_task"`
		Timestamp        string           `yaml:"timestamp"`
		Severity         types.BugSeverity `yaml:"severity"`
		Status           types.BugStatus   `yaml:"status"`
	}

	fm := frontmatter{
		BugID:            payload.BugID,
		DiscoveredByTask: payload.DiscoveredByTask,
		Timestamp:        payload.Timestamp,
		Severity:         payload.Severity,
		Status:           payload.Status,
	}

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n")
	if payload.Body != "" {
		sb.WriteString("\n")
		sb.WriteString(payload.Body)
	}

	return sb.String(), nil
}

// nextBugArchivePath returns the path for the next bug archive file for the
// given taskID under archiveDir. The first write uses bug-<taskID>.md; repeated
// writes use bug-<taskID>-v2.md, bug-<taskID>-v3.md, etc.
func nextBugArchivePath(archiveDir, taskID string) (string, error) {
	first := filepath.Join(archiveDir, "bug-"+taskID+".md")
	if _, err := os.Stat(first); errors.Is(err, os.ErrNotExist) {
		return first, nil
	} else if err != nil {
		return "", err
	}
	for version := 2; ; version++ {
		candidate := filepath.Join(archiveDir, fmt.Sprintf("bug-%s-v%d.md", taskID, version))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
}
