// Package types defines all shared structs and typed constants used by the
// doug orchestrator. YAML struct tags match the existing Bash orchestrator
// schema (snake_case field names).
package types

// ---------------------------------------------------------------------------
// Typed constants
// ---------------------------------------------------------------------------

// Status represents the lifecycle state of a user-defined task.
type Status string

const (
	StatusTODO       Status = "TODO"
	StatusInProgress Status = "IN_PROGRESS"
	StatusDone       Status = "DONE"
	StatusBlocked    Status = "BLOCKED"
)

// Outcome represents the result reported by an agent after completing a task.
type Outcome string

const (
	OutcomeSuccess      Outcome = "SUCCESS"
	OutcomeBug          Outcome = "BUG"
	OutcomeFailure      Outcome = "FAILURE"
	OutcomeEpicComplete Outcome = "EPIC_COMPLETE"
	OutcomeBuildFailure Outcome = "BUILD_FAILURE"
)

// ProjectStatus represents the overall lifecycle state of the orchestration loop.
type ProjectStatus string

const (
	// ProjectStatusPaused indicates that the loop is paused after build or test
	// verification failed following an agent SUCCESS. The working tree is
	// preserved for manual inspection. Clear this field in project-state.yaml
	// and run `doug run` to resume.
	ProjectStatusPaused ProjectStatus = "PAUSED"
)

// EpicLifecycleStatus represents the lifecycle state of a backlog epic package.
type EpicLifecycleStatus string

const (
	EpicStatusPlanned   EpicLifecycleStatus = "PLANNED"
	EpicStatusActive    EpicLifecycleStatus = "ACTIVE"
	EpicStatusCompleted EpicLifecycleStatus = "COMPLETED"
)

// IsValid reports whether the status is one of the supported backlog states.
func (s EpicLifecycleStatus) IsValid() bool {
	return s == EpicStatusPlanned || s == EpicStatusActive || s == EpicStatusCompleted
}

// TaskType classifies a task for skill dispatch and backlog persistence.
type TaskType string

const (
	TaskTypeFeature       TaskType = "feature"
	TaskTypeBugfix        TaskType = "bugfix"
	TaskTypeDocumentation TaskType = "documentation"
	TaskTypeManualReview  TaskType = "manual_review"
	TaskTypeScaffold      TaskType = "scaffold"
)

// IsSynthetic reports whether this task type is runtime-only and can never
// appear in user-authored tasks.yaml or PLAN.md backlog files.
//
// Only scaffold is runtime-only: it is used exclusively by the doug scaffold
// command, never by the doug run loop, and never written to tasks.yaml.
//
// feature, bugfix, documentation, and manual_review are all user-authorable:
// they can appear in PLAN.md handoff data and tasks.yaml. Handler-injected
// bugfix tasks (BUG-xxx IDs) are the same type as user-authored bugfix tasks;
// the distinction is at the task-ID level, not the type level.
func (t TaskType) IsSynthetic() bool {
	return t == TaskTypeScaffold
}

// ---------------------------------------------------------------------------
// project-state.yaml types
// ---------------------------------------------------------------------------

// ProjectState mirrors the full structure of project-state.yaml.
type ProjectState struct {
	Status      ProjectStatus `yaml:"status,omitempty"`
	CurrentEpic EpicState     `yaml:"current_epic"`
	ActiveTask  TaskPointer   `yaml:"active_task"`
	NextTask    TaskPointer   `yaml:"next_task"`
	Metrics     Metrics       `yaml:"metrics"`
}

// EpicState is the current_epic block in project-state.yaml.
type EpicState struct {
	ID          string  `yaml:"id"`
	Name        string  `yaml:"name"`
	BranchName  string  `yaml:"branch_name"`
	StartedAt   string  `yaml:"started_at"`
	CompletedAt *string `yaml:"completed_at"`
}

// TaskPointer is a lightweight reference to the active or next task.
// It is used for both active_task and next_task in project-state.yaml.
// Attempts is present only on active_task; omitempty suppresses it for next_task.
type TaskPointer struct {
	Type                    TaskType `yaml:"type"`
	ID                      string   `yaml:"id"`
	Attempts                int      `yaml:"attempts,omitempty"`
	ConsecutiveTestFailures int      `yaml:"consecutive_test_failures,omitempty"`
	TestFailureOutput       string   `yaml:"test_failure_output,omitempty"`
}

// Metrics is the metrics block in project-state.yaml.
type Metrics struct {
	TotalTasksCompleted  int          `yaml:"total_tasks_completed"`
	TotalDurationSeconds int          `yaml:"total_duration_seconds"`
	Tasks                []TaskMetric `yaml:"tasks"`
}

// TaskMetric records the outcome of a single completed task.
type TaskMetric struct {
	TaskID               string `yaml:"task_id"`
	Outcome              string `yaml:"outcome"`
	DurationSeconds      int    `yaml:"duration_seconds"`
	CompletedAt          string `yaml:"completed_at"`
	CommitSHA            string `yaml:"commit_sha,omitempty"`
	Attempts             int    `yaml:"attempts,omitempty"`
	TaskType             string `yaml:"task_type,omitempty"`
	AgentDurationSeconds int    `yaml:"agent_duration_seconds,omitempty"`
}

// ---------------------------------------------------------------------------
// tasks.yaml types
// ---------------------------------------------------------------------------

// Tasks mirrors the full structure of tasks.yaml.
type Tasks struct {
	Epic EpicDefinition `yaml:"epic"`
}

// EpicDefinition is the epic block in tasks.yaml.
type EpicDefinition struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Tasks []Task `yaml:"tasks"`
}

// Task is a single entry in the epic task list (tasks.yaml).
//
// UserDefined is not persisted to YAML (yaml:"-"). It is set to true by the
// loader for every task read from tasks.yaml. Scaffold is the only runtime-only
// type that never appears as a Task value in tasks.yaml; all other task types
// (feature, bugfix, documentation, manual_review) are user-authorable.
type Task struct {
	ID                 string   `yaml:"id"`
	Type               TaskType `yaml:"type"`
	Status             Status   `yaml:"status"`
	Description        string   `yaml:"description"`
	AcceptanceCriteria []string `yaml:"acceptance_criteria"`
	UserDefined        bool     `yaml:"-"`
}

// EpicMetadata mirrors .doug/plan/epics/<EPIC-ID>/metadata.yaml.
type EpicMetadata struct {
	EpicID         string              `yaml:"epic_id"`
	Status         EpicLifecycleStatus `yaml:"status"`
	CreatedAt      string              `yaml:"created_at"`
	SourcePlanPath string              `yaml:"source_plan_path"`
	ActivatedAt    *string             `yaml:"activated_at,omitempty"`
	CompletedAt    *string             `yaml:"completed_at,omitempty"`
}

// ---------------------------------------------------------------------------
// Agent session result types
// ---------------------------------------------------------------------------

// SessionResult is parsed from the YAML front-matter of the agent's session
// file. The orchestrator requires exactly these three fields; all other session
// metadata (timestamps, file lists, test counts, etc.) is managed by the
// orchestrator itself and is not part of the Go type contract.
type SessionResult struct {
	Outcome           Outcome  `yaml:"outcome"`
	ChangelogEntry    string   `yaml:"changelog_entry"`
	DependenciesAdded []string `yaml:"dependencies_added"`
}
