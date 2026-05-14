package agent

import (
	"path/filepath"
	"testing"
)

func TestPiSessionDir(t *testing.T) {
	projectRoot := "/home/user/project"

	tests := []struct {
		name    string
		req     RunRequest
		wantDir string
	}{
		{
			name: "runtime task scopes session to epicID/taskID/attempt",
			req: RunRequest{
				Phase:       RunPhaseRuntime,
				ProjectRoot: projectRoot,
				Task: TaskContext{
					EpicID:  "EPIC-33",
					ID:      "EPIC-33-001",
					Attempt: 1,
				},
			},
			wantDir: filepath.Join(projectRoot, ".doug", "logs", piSessionRootDir, "EPIC-33", "EPIC-33-001", "attempt-1"),
		},
		{
			name: "scaffold task with epic scopes session to epicID/SCAFFOLD/attempt",
			req: RunRequest{
				Phase:       RunPhaseScaffold,
				ProjectRoot: projectRoot,
				Task: TaskContext{
					EpicID:  "EPIC-33",
					ID:      "SCAFFOLD",
					Attempt: 1,
				},
			},
			wantDir: filepath.Join(projectRoot, ".doug", "logs", piSessionRootDir, "EPIC-33", "SCAFFOLD", "attempt-1"),
		},
		{
			name: "scaffold task without epic falls back to phase name",
			req: RunRequest{
				Phase:       RunPhaseScaffold,
				ProjectRoot: projectRoot,
				Task: TaskContext{
					EpicID:  "",
					ID:      "SCAFFOLD",
					Attempt: 1,
				},
			},
			wantDir: filepath.Join(projectRoot, ".doug", "logs", piSessionRootDir, "scaffold", "SCAFFOLD", "attempt-1"),
		},
		{
			name: "post-epic-KB task scopes session to epicID/POST_EPIC_KB/attempt",
			req: RunRequest{
				Phase:       RunPhasePostEpicKB,
				ProjectRoot: projectRoot,
				Task: TaskContext{
					EpicID:  "EPIC-33",
					ID:      "POST_EPIC_KB",
					Attempt: 1,
				},
			},
			wantDir: filepath.Join(projectRoot, ".doug", "logs", piSessionRootDir, "EPIC-33", "POST_EPIC_KB", "attempt-1"),
		},
		{
			name: "zero attempt uses attempt-0 to avoid empty directory name",
			req: RunRequest{
				Phase:       RunPhaseRuntime,
				ProjectRoot: projectRoot,
				Task: TaskContext{
					EpicID:  "EPIC-33",
					ID:      "EPIC-33-001",
					Attempt: 0,
				},
			},
			wantDir: filepath.Join(projectRoot, ".doug", "logs", piSessionRootDir, "EPIC-33", "EPIC-33-001", "attempt-0"),
		},
		{
			name: "missing task ID falls back to 'task'",
			req: RunRequest{
				Phase:       RunPhaseRuntime,
				ProjectRoot: projectRoot,
				Task: TaskContext{
					EpicID:  "EPIC-33",
					ID:      "",
					Attempt: 2,
				},
			},
			wantDir: filepath.Join(projectRoot, ".doug", "logs", piSessionRootDir, "EPIC-33", "task", "attempt-2"),
		},
		{
			name: "missing epic and task IDs both fall back to phase-derived and 'task'",
			req: RunRequest{
				Phase:       RunPhasePostEpicKB,
				ProjectRoot: projectRoot,
				Task: TaskContext{
					EpicID:  "",
					ID:      "",
					Attempt: 1,
				},
			},
			wantDir: filepath.Join(projectRoot, ".doug", "logs", piSessionRootDir, "post_epic_kb", "task", "attempt-1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := piSessionDir(tt.req)
			if got != tt.wantDir {
				t.Fatalf("piSessionDir() = %q, want %q", got, tt.wantDir)
			}
		})
	}
}
