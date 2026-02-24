package changelog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/changelog"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeTemp creates a temporary file with the given content and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return path
}

// readFile reads the file at path and returns its content as a string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile: %v", err)
	}
	return string(data)
}

const sampleChangelog = `# Changelog

## [Unreleased]

### Added

### Changed

### Fixed

### Removed
`

// ---------------------------------------------------------------------------
// UpdateChangelog — section header mapping
// ---------------------------------------------------------------------------

func TestUpdateChangelog_FeatureGoesToAdded(t *testing.T) {
	path := writeTemp(t, sampleChangelog)

	err := changelog.UpdateChangelog(path, "Added cool feature", "feature")
	if err != nil {
		t.Fatalf("UpdateChangelog: unexpected error: %v", err)
	}

	content := readFile(t, path)
	if !strings.Contains(content, "- Added cool feature") {
		t.Errorf("entry not found in changelog:\n%s", content)
	}

	// Verify it appears under ### Added (not under ### Changed or ### Fixed).
	addedIdx := strings.Index(content, "### Added")
	entryIdx := strings.Index(content, "- Added cool feature")
	changedIdx := strings.Index(content, "### Changed")
	if entryIdx <= addedIdx {
		t.Errorf("entry appears before ### Added section: addedIdx=%d, entryIdx=%d", addedIdx, entryIdx)
	}
	if entryIdx >= changedIdx {
		t.Errorf("entry appears after ### Changed section (should be under ### Added): entryIdx=%d, changedIdx=%d", entryIdx, changedIdx)
	}
}

func TestUpdateChangelog_BugfixGoesToFixed(t *testing.T) {
	path := writeTemp(t, sampleChangelog)

	err := changelog.UpdateChangelog(path, "Fixed null pointer exception", "bugfix")
	if err != nil {
		t.Fatalf("UpdateChangelog: unexpected error: %v", err)
	}

	content := readFile(t, path)
	if !strings.Contains(content, "- Fixed null pointer exception") {
		t.Errorf("entry not found in changelog:\n%s", content)
	}

	fixedIdx := strings.Index(content, "### Fixed")
	entryIdx := strings.Index(content, "- Fixed null pointer exception")
	removedIdx := strings.Index(content, "### Removed")
	if entryIdx <= fixedIdx {
		t.Errorf("entry appears before ### Fixed section")
	}
	if entryIdx >= removedIdx {
		t.Errorf("entry appears after ### Removed (should be under ### Fixed)")
	}
}

func TestUpdateChangelog_DocumentationGoesToChanged(t *testing.T) {
	path := writeTemp(t, sampleChangelog)

	err := changelog.UpdateChangelog(path, "Synthesized KB articles", "documentation")
	if err != nil {
		t.Fatalf("UpdateChangelog: unexpected error: %v", err)
	}

	content := readFile(t, path)
	if !strings.Contains(content, "- Synthesized KB articles") {
		t.Errorf("entry not found in changelog:\n%s", content)
	}

	changedIdx := strings.Index(content, "### Changed")
	entryIdx := strings.Index(content, "- Synthesized KB articles")
	fixedIdx := strings.Index(content, "### Fixed")
	if entryIdx <= changedIdx {
		t.Errorf("entry appears before ### Changed section")
	}
	if entryIdx >= fixedIdx {
		t.Errorf("entry appears after ### Fixed (should be under ### Changed)")
	}
}

// ---------------------------------------------------------------------------
// UpdateChangelog — idempotency
// ---------------------------------------------------------------------------

func TestUpdateChangelog_Idempotent_SameEntryTwice(t *testing.T) {
	path := writeTemp(t, sampleChangelog)

	err := changelog.UpdateChangelog(path, "My new feature", "feature")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	err = changelog.UpdateChangelog(path, "My new feature", "feature")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	content := readFile(t, path)
	count := strings.Count(content, "- My new feature")
	if count != 1 {
		t.Errorf("entry appears %d times (want exactly 1):\n%s", count, content)
	}
}

// ---------------------------------------------------------------------------
// UpdateChangelog — error cases
// ---------------------------------------------------------------------------

func TestUpdateChangelog_UnknownTaskType_ReturnsError(t *testing.T) {
	path := writeTemp(t, sampleChangelog)

	err := changelog.UpdateChangelog(path, "Something", "unknown_type")
	if err == nil {
		t.Fatal("UpdateChangelog: expected error for unknown task type, got nil")
	}
}

func TestUpdateChangelog_SectionNotFound_ReturnsError(t *testing.T) {
	// A changelog that has no ### Fixed section.
	content := "# Changelog\n\n## [Unreleased]\n\n### Added\n\n### Changed\n"
	path := writeTemp(t, content)

	err := changelog.UpdateChangelog(path, "Fixed something", "bugfix")
	if err == nil {
		t.Fatal("UpdateChangelog: expected error when ### Fixed section is missing, got nil")
	}
}

func TestUpdateChangelog_FileNotFound_ReturnsError(t *testing.T) {
	err := changelog.UpdateChangelog("/nonexistent/path/CHANGELOG.md", "entry", "feature")
	if err == nil {
		t.Fatal("UpdateChangelog: expected error for missing file, got nil")
	}
}

// ---------------------------------------------------------------------------
// UpdateChangelog — content integrity
// ---------------------------------------------------------------------------

func TestUpdateChangelog_ExistingEntriesPreserved(t *testing.T) {
	content := `# Changelog

## [Unreleased]

### Added
- Existing entry one
- Existing entry two

### Changed

### Fixed

### Removed
`
	path := writeTemp(t, content)

	err := changelog.UpdateChangelog(path, "Brand new entry", "feature")
	if err != nil {
		t.Fatalf("UpdateChangelog: %v", err)
	}

	result := readFile(t, path)
	if !strings.Contains(result, "- Existing entry one") {
		t.Error("existing entry one was removed")
	}
	if !strings.Contains(result, "- Existing entry two") {
		t.Error("existing entry two was removed")
	}
	if !strings.Contains(result, "- Brand new entry") {
		t.Error("new entry was not inserted")
	}
}

func TestUpdateChangelog_MultipleDistinctEntries(t *testing.T) {
	path := writeTemp(t, sampleChangelog)

	if err := changelog.UpdateChangelog(path, "First feature", "feature"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := changelog.UpdateChangelog(path, "Second feature", "feature"); err != nil {
		t.Fatalf("second insert: %v", err)
	}

	content := readFile(t, path)
	if !strings.Contains(content, "- First feature") {
		t.Error("first feature entry not found")
	}
	if !strings.Contains(content, "- Second feature") {
		t.Error("second feature entry not found")
	}
}
