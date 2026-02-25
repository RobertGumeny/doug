package templates_test

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/templates"
)

func TestRuntimeFS_ContainsSessionResult(t *testing.T) {
	f, err := templates.Runtime.Open("runtime/session_result.md")
	if err != nil {
		t.Fatalf("runtime/session_result.md not found in embedded Runtime FS: %v", err)
	}
	f.Close()
}

func TestInitFS_ContainsExpectedFiles(t *testing.T) {
	expectedFiles := []string{
		"init/CLAUDE.md",
		"init/AGENTS.md",
		"init/SESSION_RESULTS_TEMPLATE.md",
		"init/BUG_REPORT_TEMPLATE.md",
		"init/FAILURE_REPORT_TEMPLATE.md",
		"init/skills/implement-feature/SKILL.md",
		"init/skills/implement-bugfix/SKILL.md",
		"init/skills/implement-documentation/SKILL.md",
	}
	for _, path := range expectedFiles {
		f, err := templates.Init.Open(path)
		if err != nil {
			t.Errorf("expected file %q not found in embedded Init FS: %v", path, err)
			continue
		}
		f.Close()
	}
}

func TestSessionResult_ThreeFrontmatterFieldsOnly(t *testing.T) {
	content := templates.SessionResult

	// Must have exactly the three required fields.
	for _, want := range []string{`outcome: ""`, `changelog_entry: ""`, "dependencies_added: []"} {
		if !strings.Contains(content, want) {
			t.Errorf("runtime/session_result.md missing required frontmatter field %q", want)
		}
	}

	// Must NOT have any of the removed fields.
	for _, forbidden := range []string{"task_id:", "timestamp:", "files_modified:", "tests_run:", "build_successful:"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("runtime/session_result.md must not contain frontmatter field %q", forbidden)
		}
	}
}
