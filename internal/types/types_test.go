package types_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/types"
)

// strPtr is a test helper that returns a pointer to the given string.
func strPtr(s string) *string { return &s }

// TestProjectStateRoundTrip verifies that ProjectState survives a full
// marshal → unmarshal cycle with all fields preserved.
func TestProjectStateRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input types.ProjectState
	}{
		{
			name: "full state with completed epic",
			input: types.ProjectState{
				CurrentEpic: types.EpicState{
					ID:          "EPIC-1",
					Name:        "Scaffold & Core Types",
					BranchName:  "feature/EPIC-1",
					StartedAt:   "2026-02-24T20:01:28Z",
					CompletedAt: strPtr("2026-02-24T21:00:00Z"),
				},
				ActiveTask: types.TaskPointer{
					Type:     types.TaskTypeFeature,
					ID:       "EPIC-1-002",
					Attempts: 1,
				},
				NextTask: types.TaskPointer{
					Type: types.TaskTypeFeature,
					ID:   "EPIC-1-003",
				},
				KBEnabled: true,
				Metrics: types.Metrics{
					TotalTasksCompleted:  1,
					TotalDurationSeconds: 305,
					Tasks: []types.TaskMetric{
						{
							TaskID:          "EPIC-1-001",
							Outcome:         "success",
							DurationSeconds: 305,
							CompletedAt:     "2026-02-24T20:06:54Z",
						},
					},
				},
			},
		},
		{
			name: "state with null completed_at and no metrics tasks",
			input: types.ProjectState{
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
		{
			name: "synthetic documentation task",
			input: types.ProjectState{
				CurrentEpic: types.EpicState{
					ID:         "EPIC-3",
					Name:       "State Management",
					BranchName: "feature/EPIC-3",
					StartedAt:  "2026-04-01T00:00:00Z",
				},
				ActiveTask: types.TaskPointer{
					Type:     types.TaskTypeDocumentation,
					ID:       "KB_UPDATE",
					Attempts: 0,
				},
				KBEnabled: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yaml.Marshal(&tt.input)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var got types.ProjectState
			if err := yaml.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Compare fields explicitly to provide actionable failure messages.
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
			// completed_at: both nil or both equal string value
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
			if len(got.Metrics.Tasks) != len(tt.input.Metrics.Tasks) {
				t.Errorf("Metrics.Tasks length: got %d, want %d",
					len(got.Metrics.Tasks), len(tt.input.Metrics.Tasks))
			} else {
				for i, wantMetric := range tt.input.Metrics.Tasks {
					gotMetric := got.Metrics.Tasks[i]
					if gotMetric.TaskID != wantMetric.TaskID {
						t.Errorf("Metrics.Tasks[%d].TaskID: got %q, want %q", i, gotMetric.TaskID, wantMetric.TaskID)
					}
					if gotMetric.DurationSeconds != wantMetric.DurationSeconds {
						t.Errorf("Metrics.Tasks[%d].DurationSeconds: got %d, want %d",
							i, gotMetric.DurationSeconds, wantMetric.DurationSeconds)
					}
				}
			}
		})
	}
}

// TestTasksRoundTrip verifies that the Tasks (tasks.yaml) structure survives
// a full marshal → unmarshal cycle.
func TestTasksRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input types.Tasks
	}{
		{
			name: "single task epic",
			input: types.Tasks{
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
							UserDefined:        true, // not round-tripped (yaml:"-")
						},
					},
				},
			},
		},
		{
			name: "multi-task epic with all statuses",
			input: types.Tasks{
				Epic: types.EpicDefinition{
					ID:   "EPIC-2",
					Name: "Infrastructure",
					Tasks: []types.Task{
						{
							ID:          "EPIC-2-001",
							Type:        types.TaskTypeFeature,
							Status:      types.StatusDone,
							Description: "First task",
							UserDefined: true,
						},
						{
							ID:          "EPIC-2-002",
							Type:        types.TaskTypeFeature,
							Status:      types.StatusInProgress,
							Description: "Second task",
							UserDefined: true,
						},
						{
							ID:          "EPIC-2-003",
							Type:        types.TaskTypeManualReview,
							Status:      types.StatusTODO,
							Description: "Third task",
							UserDefined: true,
						},
						{
							ID:          "EPIC-2-004",
							Type:        types.TaskTypeFeature,
							Status:      types.StatusBlocked,
							Description: "Blocked task",
							UserDefined: true,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yaml.Marshal(&tt.input)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var got types.Tasks
			if err := yaml.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if got.Epic.ID != tt.input.Epic.ID {
				t.Errorf("Epic.ID: got %q, want %q", got.Epic.ID, tt.input.Epic.ID)
			}
			if got.Epic.Name != tt.input.Epic.Name {
				t.Errorf("Epic.Name: got %q, want %q", got.Epic.Name, tt.input.Epic.Name)
			}
			if len(got.Epic.Tasks) != len(tt.input.Epic.Tasks) {
				t.Fatalf("Epic.Tasks length: got %d, want %d",
					len(got.Epic.Tasks), len(tt.input.Epic.Tasks))
			}

			for i, want := range tt.input.Epic.Tasks {
				got := got.Epic.Tasks[i]
				if got.ID != want.ID {
					t.Errorf("Tasks[%d].ID: got %q, want %q", i, got.ID, want.ID)
				}
				if got.Type != want.Type {
					t.Errorf("Tasks[%d].Type: got %q, want %q", i, got.Type, want.Type)
				}
				if got.Status != want.Status {
					t.Errorf("Tasks[%d].Status: got %q, want %q", i, got.Status, want.Status)
				}
				if got.Description != want.Description {
					t.Errorf("Tasks[%d].Description: got %q, want %q", i, got.Description, want.Description)
				}
				// UserDefined is yaml:"-" so it must NOT be preserved.
				if got.UserDefined != false {
					t.Errorf("Tasks[%d].UserDefined should be false after unmarshal (yaml:\"-\"), got %v",
						i, got.UserDefined)
				}
			}
		})
	}
}

// TestIsSynthetic verifies the UserDefined vs Synthetic distinction helper.
func TestIsSynthetic(t *testing.T) {
	tests := []struct {
		taskType types.TaskType
		want     bool
	}{
		{types.TaskTypeFeature, false},
		{types.TaskTypeManualReview, false},
		{types.TaskTypeBugfix, true},
		{types.TaskTypeDocumentation, true},
	}

	for _, tt := range tests {
		if got := tt.taskType.IsSynthetic(); got != tt.want {
			t.Errorf("TaskType(%q).IsSynthetic() = %v, want %v", tt.taskType, got, tt.want)
		}
	}
}

// TestSessionResultRoundTrip verifies that SessionResult has exactly three
// fields and round-trips correctly.
func TestSessionResultRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input types.SessionResult
	}{
		{
			name: "success with changelog and no dependencies",
			input: types.SessionResult{
				Outcome:           types.OutcomeSuccess,
				ChangelogEntry:    "Added core type definitions for the orchestrator",
				DependenciesAdded: nil,
			},
		},
		{
			name: "success with dependencies added",
			input: types.SessionResult{
				Outcome:           types.OutcomeSuccess,
				ChangelogEntry:    "Added state loader with yaml.v3",
				DependenciesAdded: []string{"gopkg.in/yaml.v3"},
			},
		},
		{
			name: "bug outcome",
			input: types.SessionResult{
				Outcome:        types.OutcomeBug,
				ChangelogEntry: "",
			},
		},
		{
			name: "failure outcome",
			input: types.SessionResult{
				Outcome:        types.OutcomeFailure,
				ChangelogEntry: "",
			},
		},
		{
			name: "epic complete",
			input: types.SessionResult{
				Outcome:        types.OutcomeEpicComplete,
				ChangelogEntry: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yaml.Marshal(&tt.input)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var got types.SessionResult
			if err := yaml.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if got.Outcome != tt.input.Outcome {
				t.Errorf("Outcome: got %q, want %q", got.Outcome, tt.input.Outcome)
			}
			if got.ChangelogEntry != tt.input.ChangelogEntry {
				t.Errorf("ChangelogEntry: got %q, want %q", got.ChangelogEntry, tt.input.ChangelogEntry)
			}
			if len(got.DependenciesAdded) != len(tt.input.DependenciesAdded) {
				t.Errorf("DependenciesAdded length: got %d, want %d",
					len(got.DependenciesAdded), len(tt.input.DependenciesAdded))
			} else {
				for i, dep := range tt.input.DependenciesAdded {
					if got.DependenciesAdded[i] != dep {
						t.Errorf("DependenciesAdded[%d]: got %q, want %q",
							i, got.DependenciesAdded[i], dep)
					}
				}
			}
		})
	}
}
