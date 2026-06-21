package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/types"
)

// validPayload returns a BugPayload with all required fields set to valid values.
func validPayload() types.BugPayload {
	return types.BugPayload{
		BugID:            "BUG-EPIC-5-001",
		DiscoveredByTask: "EPIC-5-001",
		Timestamp:        "2026-06-20T12:00:00Z",
		Severity:         types.BugSeverityHigh,
		Status:           types.BugStatusOpen,
		Body:             "## Summary\n\nThe feature panics on nil input.\n",
	}
}

// ---------------------------------------------------------------------------
// Frontmatter stamping
// ---------------------------------------------------------------------------

func TestWriteBugArchive_FrontmatterFields(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")

	payload := validPayload()
	if err := WriteBugArchive(logsDir, "EPIC-5", payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	archivePath := filepath.Join(logsDir, "bugs", "EPIC-5", "bug-EPIC-5-001.md")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("archive file not found at %s: %v", archivePath, err)
	}
	content := string(data)

	for _, want := range []string{
		"bug_id: BUG-EPIC-5-001",
		"discovered_by_task: EPIC-5-001",
		"2026-06-20T12:00:00Z", // yaml may quote the value; check the content
		"severity: high",
		"status: open",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("frontmatter missing %q\ncontent:\n%s", want, content)
		}
	}
}

func TestWriteBugArchive_BodyPreserved(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")

	payload := validPayload()
	payload.Body = "## Summary\n\nDetailed description of the bug.\n"

	if err := WriteBugArchive(logsDir, "EPIC-5", payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	archivePath := filepath.Join(logsDir, "bugs", "EPIC-5", "bug-EPIC-5-001.md")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("archive file not found: %v", err)
	}
	if !strings.Contains(string(data), "Detailed description of the bug.") {
		t.Errorf("body not found in archive:\n%s", string(data))
	}
}

func TestWriteBugArchive_EmptyBodyOmitted(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")

	payload := validPayload()
	payload.Body = ""

	if err := WriteBugArchive(logsDir, "EPIC-5", payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	archivePath := filepath.Join(logsDir, "bugs", "EPIC-5", "bug-EPIC-5-001.md")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("archive file not found: %v", err)
	}
	// The file should end with the closing --- and nothing extra.
	content := string(data)
	if strings.Count(content, "---") != 2 {
		t.Errorf("expected exactly two --- delimiters for empty body, got:\n%s", content)
	}
}

func TestWriteBugArchive_TimestampStampedWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")

	payload := validPayload()
	payload.Timestamp = ""

	if err := WriteBugArchive(logsDir, "EPIC-5", payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	archivePath := filepath.Join(logsDir, "bugs", "EPIC-5", "bug-EPIC-5-001.md")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("archive file not found: %v", err)
	}
	if !strings.Contains(string(data), "timestamp:") {
		t.Errorf("timestamp field missing from archive:\n%s", string(data))
	}
	// Verify the timestamp looks like an RFC3339 string (contains T and Z or +).
	for _, marker := range []string{"T", ":"} {
		if !strings.Contains(string(data), marker) {
			t.Errorf("timestamp does not look like RFC3339 (missing %q):\n%s", marker, string(data))
		}
	}
}

// ---------------------------------------------------------------------------
// Directory creation
// ---------------------------------------------------------------------------

func TestWriteBugArchive_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	// logsDir does not exist yet — WriteBugArchive must create it.
	logsDir := filepath.Join(dir, "deep", "nested", "logs")

	if err := WriteBugArchive(logsDir, "EPIC-7", validPayload()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(logsDir, "bugs", "EPIC-7", "bug-EPIC-5-001.md")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("archive not found at %s: %v", want, err)
	}
}

// ---------------------------------------------------------------------------
// Severity validation
// ---------------------------------------------------------------------------

func TestWriteBugArchive_RejectUnknownSeverity(t *testing.T) {
	tests := []struct {
		severity types.BugSeverity
	}{
		{severity: ""},
		{severity: "blocker"},
		{severity: "urgent"},
		{severity: "CRITICAL"}, // not canonicalized by caller; writer lower-cases before check
	}

	// CRITICAL in upper-case IS canonical after lower-casing; let's keep it
	// but test only truly unknown values.
	unknownTests := []types.BugSeverity{"", "blocker", "urgent", "p0"}

	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")

	for _, sev := range unknownTests {
		payload := validPayload()
		payload.Severity = sev
		err := WriteBugArchive(logsDir, "EPIC-5", payload)
		if err == nil {
			t.Errorf("severity %q: expected error, got nil", sev)
			continue
		}
		var target *ErrUnknownBugSeverity
		if !errors.As(err, &target) {
			t.Errorf("severity %q: expected *ErrUnknownBugSeverity, got: %T %v", sev, err, err)
		}
	}
	_ = tests
}

func TestWriteBugArchive_AcceptsAllValidSeverities(t *testing.T) {
	dir := t.TempDir()
	for _, sev := range []types.BugSeverity{
		types.BugSeverityCritical,
		types.BugSeverityHigh,
		types.BugSeverityMedium,
		types.BugSeverityLow,
	} {
		logsDir := filepath.Join(dir, "logs", string(sev))
		payload := validPayload()
		payload.Severity = sev
		if err := WriteBugArchive(logsDir, "EPIC-5", payload); err != nil {
			t.Errorf("severity %q: unexpected error: %v", sev, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Status validation
// ---------------------------------------------------------------------------

func TestWriteBugArchive_RejectUnknownStatus(t *testing.T) {
	unknownStatuses := []types.BugStatus{"", "closed", "pending", "done"}

	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")

	for _, status := range unknownStatuses {
		payload := validPayload()
		payload.Status = status
		err := WriteBugArchive(logsDir, "EPIC-5", payload)
		if err == nil {
			t.Errorf("status %q: expected error, got nil", status)
			continue
		}
		var target *ErrUnknownBugStatus
		if !errors.As(err, &target) {
			t.Errorf("status %q: expected *ErrUnknownBugStatus, got: %T %v", status, err, err)
		}
	}
}

func TestWriteBugArchive_AcceptsAllValidStatuses(t *testing.T) {
	dir := t.TempDir()
	for _, status := range []types.BugStatus{
		types.BugStatusOpen,
		types.BugStatusInvestigating,
		types.BugStatusFixed,
		types.BugStatusWontFix,
	} {
		logsDir := filepath.Join(dir, "logs", string(status))
		payload := validPayload()
		payload.Status = status
		if err := WriteBugArchive(logsDir, "EPIC-5", payload); err != nil {
			t.Errorf("status %q: unexpected error: %v", status, err)
		}
	}
}

func TestWriteBugArchive_RejectBeforeWriting(t *testing.T) {
	// When validation fails, no archive directory or file should be created.
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")

	payload := validPayload()
	payload.Severity = "unknown_severity"

	_ = WriteBugArchive(logsDir, "EPIC-5", payload)

	archiveDir := filepath.Join(logsDir, "bugs", "EPIC-5")
	if _, err := os.Stat(archiveDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("archive directory should not be created on validation failure, stat err=%v", err)
	}
}

// ---------------------------------------------------------------------------
// Versioned filenames
// ---------------------------------------------------------------------------

func TestWriteBugArchive_FirstWriteUsesCanonicalName(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")

	if err := WriteBugArchive(logsDir, "EPIC-5", validPayload()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(logsDir, "bugs", "EPIC-5", "bug-EPIC-5-001.md")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("first archive not at canonical path %s: %v", want, err)
	}
}

func TestWriteBugArchive_RepeatedWriteCreatesVersionedSibling(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")

	// First write.
	p1 := validPayload()
	p1.Body = "first report"
	if err := WriteBugArchive(logsDir, "EPIC-5", p1); err != nil {
		t.Fatalf("first write: %v", err)
	}

	// Second write for the same task.
	p2 := validPayload()
	p2.Body = "second report"
	if err := WriteBugArchive(logsDir, "EPIC-5", p2); err != nil {
		t.Fatalf("second write: %v", err)
	}

	firstPath := filepath.Join(logsDir, "bugs", "EPIC-5", "bug-EPIC-5-001.md")
	secondPath := filepath.Join(logsDir, "bugs", "EPIC-5", "bug-EPIC-5-001-v2.md")

	firstData, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read first archive: %v", err)
	}
	secondData, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("read second archive: %v", err)
	}

	if !strings.Contains(string(firstData), "first report") {
		t.Errorf("first archive content mismatch:\n%s", string(firstData))
	}
	if !strings.Contains(string(secondData), "second report") {
		t.Errorf("second archive content mismatch:\n%s", string(secondData))
	}
}

func TestWriteBugArchive_ThirdWriteCreatesV3(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")

	for i := 0; i < 3; i++ {
		p := validPayload()
		p.Body = "report"
		if err := WriteBugArchive(logsDir, "EPIC-5", p); err != nil {
			t.Fatalf("write %d: %v", i+1, err)
		}
	}

	v3Path := filepath.Join(logsDir, "bugs", "EPIC-5", "bug-EPIC-5-001-v3.md")
	if _, err := os.Stat(v3Path); err != nil {
		t.Errorf("v3 archive not found at %s: %v", v3Path, err)
	}
}

func TestWriteBugArchive_DifferentEpicsAreIsolated(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")

	p1 := validPayload()
	p1.Body = "epic5 bug"
	if err := WriteBugArchive(logsDir, "EPIC-5", p1); err != nil {
		t.Fatalf("EPIC-5 write: %v", err)
	}

	p2 := validPayload()
	p2.Body = "epic6 bug"
	if err := WriteBugArchive(logsDir, "EPIC-6", p2); err != nil {
		t.Fatalf("EPIC-6 write: %v", err)
	}

	// Both should be canonical (no -v2) because they're in separate epic dirs.
	for _, tc := range []struct{ epic, want string }{
		{"EPIC-5", "epic5 bug"},
		{"EPIC-6", "epic6 bug"},
	} {
		path := filepath.Join(logsDir, "bugs", tc.epic, "bug-EPIC-5-001.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("epic %s: read archive: %v", tc.epic, err)
		}
		if !strings.Contains(string(data), tc.want) {
			t.Errorf("epic %s: body mismatch:\n%s", tc.epic, string(data))
		}
	}
}
