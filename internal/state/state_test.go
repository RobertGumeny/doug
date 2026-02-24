package state_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// ---------------------------------------------------------------------------
// ProjectState tests
// ---------------------------------------------------------------------------

func TestLoadProjectStateNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := state.LoadProjectState(filepath.Join(dir, "missing.yaml"))
	if !errors.Is(err, state.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestLoadProjectStateParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")
	if err := os.WriteFile(path, []byte("key: [unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := state.LoadProjectState(path)
	var parseErr *state.ParseError
	if !errors.As(err, &parseErr) {
		t.Errorf("expected *ParseError, got %v (%T)", err, err)
	}
}

func TestProjectStateRoundTrip(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name  string
		input *types.ProjectState
	}{
		{
			name: "full state with completed_at",
			input: &types.ProjectState{
				CurrentEpic: types.EpicState{
					ID:          "EPIC-1",
					Name:        "Scaffold & Core Types",
					BranchName:  "feature/EPIC-1",
					StartedAt:   "2026-02-24T20:01:28Z",
					CompletedAt: strPtr("2026-02-24T21:00:00Z"),
				},
				ActiveTask: types.TaskPointer{
					Type:     types.TaskTypeFeature,
					ID:       "EPIC-1-003",
					Attempts: 1,
				},
				NextTask: types.TaskPointer{
					Type: types.TaskTypeFeature,
					ID:   "EPIC-1-004",
				},
				KBEnabled: true,
				Metrics: types.Metrics{
					TotalTasksCompleted:  2,
					TotalDurationSeconds: 583,
					Tasks: []types.TaskMetric{
						{TaskID: "EPIC-1-001", Outcome: "success", DurationSeconds: 305, CompletedAt: "2026-02-24T20:06:54Z"},
						{TaskID: "EPIC-1-002", Outcome: "success", DurationSeconds: 278, CompletedAt: "2026-02-24T20:11:36Z"},
					},
				},
			},
		},
		{
			name: "state with nil completed_at and empty metrics",
			input: &types.ProjectState{
				CurrentEpic: types.EpicState{
					ID:          "EPIC-2",
					Name:        "Infrastructure",
					BranchName:  "feature/EPIC-2",
					StartedAt:   "2026-03-01T09:00:00Z",
					CompletedAt: nil,
				},
				ActiveTask: types.TaskPointer{
					Type:     types.TaskTypeBugfix,
					ID:       "BUG-EPIC-2-001",
					Attempts: 2,
				},
				NextTask: types.TaskPointer{
					Type: types.TaskTypeFeature,
					ID:   "EPIC-2-002",
				},
				KBEnabled: false,
				Metrics: types.Metrics{
					TotalTasksCompleted:  0,
					TotalDurationSeconds: 0,
					Tasks:                nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "project-state.yaml")

			if err := state.SaveProjectState(path, tt.input); err != nil {
				t.Fatalf("SaveProjectState: %v", err)
			}

			// .tmp file must not remain after a successful save
			if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
				t.Error(".tmp file still exists after successful save")
			}

			got, err := state.LoadProjectState(path)
			if err != nil {
				t.Fatalf("LoadProjectState: %v", err)
			}

			if got.CurrentEpic.ID != tt.input.CurrentEpic.ID {
				t.Errorf("CurrentEpic.ID: got %q, want %q", got.CurrentEpic.ID, tt.input.CurrentEpic.ID)
			}
			if got.CurrentEpic.Name != tt.input.CurrentEpic.Name {
				t.Errorf("CurrentEpic.Name: got %q, want %q", got.CurrentEpic.Name, tt.input.CurrentEpic.Name)
			}
			if got.CurrentEpic.BranchName != tt.input.CurrentEpic.BranchName {
				t.Errorf("CurrentEpic.BranchName: got %q, want %q", got.CurrentEpic.BranchName, tt.input.CurrentEpic.BranchName)
			}
			if got.CurrentEpic.StartedAt != tt.input.CurrentEpic.StartedAt {
				t.Errorf("CurrentEpic.StartedAt: got %q, want %q", got.CurrentEpic.StartedAt, tt.input.CurrentEpic.StartedAt)
			}
			if (got.CurrentEpic.CompletedAt == nil) != (tt.input.CurrentEpic.CompletedAt == nil) {
				t.Errorf("CurrentEpic.CompletedAt nil mismatch: got %v, want %v",
					got.CurrentEpic.CompletedAt, tt.input.CurrentEpic.CompletedAt)
			} else if got.CurrentEpic.CompletedAt != nil && *got.CurrentEpic.CompletedAt != *tt.input.CurrentEpic.CompletedAt {
				t.Errorf("CurrentEpic.CompletedAt: got %q, want %q",
					*got.CurrentEpic.CompletedAt, *tt.input.CurrentEpic.CompletedAt)
			}
			if got.ActiveTask.Type != tt.input.ActiveTask.Type {
				t.Errorf("ActiveTask.Type: got %q, want %q", got.ActiveTask.Type, tt.input.ActiveTask.Type)
			}
			if got.ActiveTask.ID != tt.input.ActiveTask.ID {
				t.Errorf("ActiveTask.ID: got %q, want %q", got.ActiveTask.ID, tt.input.ActiveTask.ID)
			}
			if got.ActiveTask.Attempts != tt.input.ActiveTask.Attempts {
				t.Errorf("ActiveTask.Attempts: got %d, want %d", got.ActiveTask.Attempts, tt.input.ActiveTask.Attempts)
			}
			if got.NextTask.Type != tt.input.NextTask.Type {
				t.Errorf("NextTask.Type: got %q, want %q", got.NextTask.Type, tt.input.NextTask.Type)
			}
			if got.NextTask.ID != tt.input.NextTask.ID {
				t.Errorf("NextTask.ID: got %q, want %q", got.NextTask.ID, tt.input.NextTask.ID)
			}
			if got.KBEnabled != tt.input.KBEnabled {
				t.Errorf("KBEnabled: got %v, want %v", got.KBEnabled, tt.input.KBEnabled)
			}
			if got.Metrics.TotalTasksCompleted != tt.input.Metrics.TotalTasksCompleted {
				t.Errorf("Metrics.TotalTasksCompleted: got %d, want %d",
					got.Metrics.TotalTasksCompleted, tt.input.Metrics.TotalTasksCompleted)
			}
			if got.Metrics.TotalDurationSeconds != tt.input.Metrics.TotalDurationSeconds {
				t.Errorf("Metrics.TotalDurationSeconds: got %d, want %d",
					got.Metrics.TotalDurationSeconds, tt.input.Metrics.TotalDurationSeconds)
			}
			if len(got.Metrics.Tasks) != len(tt.input.Metrics.Tasks) {
				t.Fatalf("Metrics.Tasks len: got %d, want %d",
					len(got.Metrics.Tasks), len(tt.input.Metrics.Tasks))
			}
			for i, want := range tt.input.Metrics.Tasks {
				g := got.Metrics.Tasks[i]
				if g.TaskID != want.TaskID {
					t.Errorf("Metrics.Tasks[%d].TaskID: got %q, want %q", i, g.TaskID, want.TaskID)
				}
				if g.Outcome != want.Outcome {
					t.Errorf("Metrics.Tasks[%d].Outcome: got %q, want %q", i, g.Outcome, want.Outcome)
				}
				if g.DurationSeconds != want.DurationSeconds {
					t.Errorf("Metrics.Tasks[%d].DurationSeconds: got %d, want %d", i, g.DurationSeconds, want.DurationSeconds)
				}
				if g.CompletedAt != want.CompletedAt {
					t.Errorf("Metrics.Tasks[%d].CompletedAt: got %q, want %q", i, g.CompletedAt, want.CompletedAt)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tasks tests
// ---------------------------------------------------------------------------

func TestLoadTasksNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := state.LoadTasks(filepath.Join(dir, "missing.yaml"))
	if !errors.Is(err, state.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestLoadTasksParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.yaml")
	if err := os.WriteFile(path, []byte("key: [unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := state.LoadTasks(path)
	var parseErr *state.ParseError
	if !errors.As(err, &parseErr) {
		t.Errorf("expected *ParseError, got %v (%T)", err, err)
	}
}

func TestTasksRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input *types.Tasks
	}{
		{
			name: "single task epic",
			input: &types.Tasks{
				Epic: types.EpicDefinition{
					ID:   "EPIC-1",
					Name: "Scaffold & Core Types",
					Tasks: []types.Task{
						{
							ID:                 "EPIC-1-001",
							Type:               types.TaskTypeFeature,
							Status:             types.StatusDone,
							Description:        "Verify the project scaffold.",
							AcceptanceCriteria: []string{"go build ./... passes", "go test ./... passes"},
							UserDefined:        true,
						},
					},
				},
			},
		},
		{
			name: "multi-task epic with all statuses",
			input: &types.Tasks{
				Epic: types.EpicDefinition{
					ID:   "EPIC-2",
					Name: "Infrastructure",
					Tasks: []types.Task{
						{ID: "EPIC-2-001", Type: types.TaskTypeFeature, Status: types.StatusDone, Description: "First task"},
						{ID: "EPIC-2-002", Type: types.TaskTypeFeature, Status: types.StatusInProgress, Description: "Second task"},
						{ID: "EPIC-2-003", Type: types.TaskTypeManualReview, Status: types.StatusTODO, Description: "Third task"},
						{ID: "EPIC-2-004", Type: types.TaskTypeFeature, Status: types.StatusBlocked, Description: "Blocked task"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "tasks.yaml")

			if err := state.SaveTasks(path, tt.input); err != nil {
				t.Fatalf("SaveTasks: %v", err)
			}

			// .tmp file must not remain after a successful save
			if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
				t.Error(".tmp file still exists after successful save")
			}

			got, err := state.LoadTasks(path)
			if err != nil {
				t.Fatalf("LoadTasks: %v", err)
			}

			if got.Epic.ID != tt.input.Epic.ID {
				t.Errorf("Epic.ID: got %q, want %q", got.Epic.ID, tt.input.Epic.ID)
			}
			if got.Epic.Name != tt.input.Epic.Name {
				t.Errorf("Epic.Name: got %q, want %q", got.Epic.Name, tt.input.Epic.Name)
			}
			if len(got.Epic.Tasks) != len(tt.input.Epic.Tasks) {
				t.Fatalf("Epic.Tasks len: got %d, want %d", len(got.Epic.Tasks), len(tt.input.Epic.Tasks))
			}
			for i, want := range tt.input.Epic.Tasks {
				g := got.Epic.Tasks[i]
				if g.ID != want.ID {
					t.Errorf("Tasks[%d].ID: got %q, want %q", i, g.ID, want.ID)
				}
				if g.Type != want.Type {
					t.Errorf("Tasks[%d].Type: got %q, want %q", i, g.Type, want.Type)
				}
				if g.Status != want.Status {
					t.Errorf("Tasks[%d].Status: got %q, want %q", i, g.Status, want.Status)
				}
				if g.Description != want.Description {
					t.Errorf("Tasks[%d].Description: got %q, want %q", i, g.Description, want.Description)
				}
				// UserDefined is set by the loader, not preserved via YAML (yaml:"-")
				if !g.UserDefined {
					t.Errorf("Tasks[%d].UserDefined: LoadTasks should set this to true", i)
				}
			}
		})
	}
}
