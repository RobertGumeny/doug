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
)

// TaskType classifies a task as user-defined or orchestrator-injected (synthetic).
type TaskType string

const (
	TaskTypeFeature       TaskType = "feature"
	TaskTypeBugfix        TaskType = "bugfix"
	TaskTypeDocumentation TaskType = "documentation"
	TaskTypeManualReview  TaskType = "manual_review"
)

// IsSynthetic reports whether this task type is orchestrator-injected.
// Synthetic tasks (bugfix, documentation) are never written to tasks.yaml;
// they exist only in project-state.yaml.active_task as transient state.
func (t TaskType) IsSynthetic() bool {
	return t == TaskTypeBugfix || t == TaskTypeDocumentation
}

// ---------------------------------------------------------------------------
// project-state.yaml types
// ---------------------------------------------------------------------------

// ProjectState mirrors the full structure of project-state.yaml.
type ProjectState struct {
	CurrentEpic EpicState   `yaml:"current_epic"`
	ActiveTask  TaskPointer `yaml:"active_task"`
	NextTask    TaskPointer `yaml:"next_task"`
	KBEnabled   bool        `yaml:"kb_enabled"`
	Metrics     Metrics     `yaml:"metrics"`
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
	Type     TaskType `yaml:"type"`
	ID       string   `yaml:"id"`
	Attempts int      `yaml:"attempts,omitempty"`
}

// Metrics is the metrics block in project-state.yaml.
type Metrics struct {
	TotalTasksCompleted  int          `yaml:"total_tasks_completed"`
	TotalDurationSeconds int          `yaml:"total_duration_seconds"`
	Tasks                []TaskMetric `yaml:"tasks"`
}

// TaskMetric records the outcome of a single completed task.
type TaskMetric struct {
	TaskID          string `yaml:"task_id"`
	Outcome         string `yaml:"outcome"`
	DurationSeconds int    `yaml:"duration_seconds"`
	CompletedAt     string `yaml:"completed_at"`
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
// loader for every task read from tasks.yaml, establishing the UserDefined vs
// Synthetic distinction at the type level. Synthetic tasks (bugfix,
// documentation) are orchestrator-injected; they never appear as Task values.
type Task struct {
	ID                 string   `yaml:"id"`
	Type               TaskType `yaml:"type"`
	Status             Status   `yaml:"status"`
	Description        string   `yaml:"description"`
	AcceptanceCriteria []string `yaml:"acceptance_criteria"`
	UserDefined        bool     `yaml:"-"`
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
