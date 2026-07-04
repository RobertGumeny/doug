package dougpath

import (
	"path/filepath"
	"testing"
)

func TestPaths_NewStorageContract(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "project")
	paths := New(root)
	epicID := "EPIC-53"
	taskID := "EPIC-53-001"
	attempt := 2

	assertPath(t, "intake bugs dir", paths.IntakeBugsDir(), root, ".doug", "intake", "bugs")
	assertPath(t, "intake bug report", paths.IntakeBugPath(epicID, taskID), root, ".doug", "intake", "bugs", epicID, "bug-EPIC-53-001.md")
	assertPath(t, "intake research dir", paths.IntakeResearchDir(), root, ".doug", "intake", "research")
	assertPath(t, "intake research report", paths.IntakeResearchPath("2026-06-27-path-contract.md"), root, ".doug", "intake", "research", "2026-06-27-path-contract.md")

	assertPath(t, "epic PRD snapshot", paths.EpicPRDPath(epicID), root, ".doug", "logs", "epics", epicID, "PRD.md")
	assertPath(t, "epic tasks snapshot", paths.EpicTasksPath(epicID), root, ".doug", "logs", "epics", epicID, "tasks.yaml")
	assertPath(t, "epic project-state snapshot", paths.EpicProjectStatePath(epicID), root, ".doug", "logs", "epics", epicID, "project-state.yaml")

	assertPath(t, "task attempt dir", paths.TaskAttemptDir(epicID, taskID, attempt), root, ".doug", "logs", "epics", epicID, taskID, "attempt-2")
	assertPath(t, "session archive", paths.SessionArchivePath(epicID, taskID, attempt), root, ".doug", "logs", "epics", epicID, taskID, "attempt-2", "session.md")
	assertPath(t, "stats", paths.StatsPath(epicID, taskID, attempt), root, ".doug", "logs", "epics", epicID, taskID, "attempt-2", "stats.json")
	assertPath(t, "attempt-start marker", paths.AttemptStartPath(epicID, taskID, attempt), root, ".doug", "logs", "epics", epicID, taskID, "attempt-2", "attempt-start.json")
	assertPath(t, "transport-failure record", paths.TransportFailurePath(epicID, taskID, attempt), root, ".doug", "logs", "epics", epicID, taskID, "attempt-2", "infra-failure.md")

	assertPath(t, "review artifact", paths.ReviewArtifactPath(epicID, 1), root, ".doug", "logs", "epics", epicID, "epic-review.md")
	assertPath(t, "versioned review artifact", paths.ReviewArtifactPath(epicID, 3), root, ".doug", "logs", "epics", epicID, "epic-review-v3.md")
}

func TestPaths_PiSessionContractStabilizesDirectoryButKeepsNativeJSONLName(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "project")
	paths := New(root)

	assertPath(t, "Pi session directory", paths.PiSessionDir("EPIC-53", "EPIC-53-001", 1), root, ".doug", "logs", "epics", "EPIC-53", "EPIC-53-001", "attempt-1")
	assertPath(t, "Pi native JSONL transcript", paths.PiNativeSessionPath("EPIC-53", "EPIC-53-001", 1, "2026-06-27T120102.123456789Z.jsonl"), root, ".doug", "logs", "epics", "EPIC-53", "EPIC-53-001", "attempt-1", "2026-06-27T120102.123456789Z.jsonl")
}

func assertPath(t *testing.T, name, got string, parts ...string) {
	t.Helper()
	want := filepath.Join(parts...)
	if got != want {
		t.Fatalf("%s: got %q, want %q", name, got, want)
	}
}
