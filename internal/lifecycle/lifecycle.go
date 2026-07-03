// Package lifecycle provides the shared internal task lifecycle core used by
// interactive control surfaces without exposing a public Doug API.
package lifecycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/plan"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// StatusKind describes Doug's current assignment lifecycle state.
type StatusKind string

const (
	StatusNoActiveTask StatusKind = "NO_ACTIVE_TASK"
	StatusActiveTask   StatusKind = "ACTIVE_TASK"
	StatusComplete     StatusKind = "COMPLETE"
)

const (
	DiagnosticActionGetStatus          = "get_status"
	DiagnosticActionGetNextTask        = "get_next_task"
	DiagnosticActionReportTaskComplete = "report_task_complete"
	DiagnosticActionReportTaskBlocked  = "report_task_blocked"
	DiagnosticActionReconcileRepair    = "reconcile_lifecycle(mode=repair)"
	DiagnosticActionManualReview       = "manual_review"
)

const ReconcileModeRepair = "repair"

// Paths identifies the lifecycle files owned by Doug.
type Paths struct {
	ProjectRoot string
	DougDir     string
	StatePath   string
	TasksPath   string
}

// DefaultPaths returns the conventional .doug lifecycle paths for projectRoot.
func DefaultPaths(projectRoot string) Paths {
	dougDir := filepath.Join(projectRoot, ".doug")
	return Paths{
		ProjectRoot: projectRoot,
		DougDir:     dougDir,
		StatePath:   filepath.Join(dougDir, "project-state.yaml"),
		TasksPath:   filepath.Join(dougDir, "tasks.yaml"),
	}
}

// Options configures status discovery and assignment claiming.
type Options struct {
	Paths       Paths
	MaxRetries  int
	BuildSystem string
	Logger      log.Logger
}

// Status reports the current lifecycle state without mutating any files.
type Status struct {
	Kind           StatusKind
	EpicID         string
	ActiveTask     types.TaskPointer
	NextTask       types.TaskPointer
	ActiveTaskPath string
	AllTasksDone   bool
}

// DiagnosticFinding describes lifecycle drift detected without mutating state.
type DiagnosticFinding struct {
	Code                 string
	Severity             string
	Message              string
	Path                 string
	RequiresManualReview bool
}

// Diagnostics reports lifecycle health, findings, and safe next actions.
type Diagnostics struct {
	Status             Status
	Findings           []DiagnosticFinding
	AllowedNextActions []string
}

// ChangedFile describes a file changed by lifecycle repair mode.
type ChangedFile struct {
	Path   string
	Action string
}

// ChangedField describes a lifecycle field changed by repair mode.
type ChangedField struct {
	Path   string
	Field  string
	Before string
	After  string
}

// ReconcileResult reports explicit lifecycle reconcile repair outcomes.
type ReconcileResult struct {
	Diagnostics
	Repaired      bool
	ManualReview  bool
	ChangedFiles  []ChangedFile
	ChangedFields []ChangedField
	Message       string
}

// ClaimResult reports the result of a mutating assignment claim.
type ClaimResult struct {
	Status
	AlreadyActive bool
	Claimed       bool
	Attempt       int
}

// CompletionResult reports persisted state changes from a verified successful
// task completion transition.
type CompletionResult struct {
	Status
	Terminal bool
	Advanced bool
}

// FailureResult reports whether a failed task remains retryable or has been
// moved into manual-review blockage.
type FailureResult struct {
	Status
	Retryable bool
	Blocked   bool
}

// FinalizationResult reports persisted state changes from epic finalization.
type FinalizationResult struct {
	Status
	ArchiveDir string
}

// DiscoverStatus loads Doug state and tasks and reports whether an assignment
// is currently active, available to claim, or complete. It never mutates state.
func DiscoverStatus(opts Options) (Status, error) {
	paths := normalizePaths(opts.Paths)
	projectState, tasks, err := load(paths)
	if err != nil {
		return Status{}, err
	}
	return discover(paths, projectState, tasks), nil
}

// DiagnoseLifecycle loads Doug lifecycle files and reports drift without
// claiming work, advancing pointers, rewriting the active brief, or otherwise
// mutating state.
func DiagnoseLifecycle(opts Options) (Diagnostics, error) {
	paths := normalizePaths(opts.Paths)
	projectState, tasks, err := load(paths)
	if err != nil {
		return Diagnostics{}, err
	}
	status := discover(paths, projectState, tasks)
	findings := diagnose(paths, projectState, tasks)
	return Diagnostics{Status: status, Findings: findings, AllowedNextActions: diagnosticActions(status, findings)}, nil
}

// ReconcileLifecycle applies explicit Doug-owned repairs only when mode is
// "repair" and every detected drift finding is supported and unambiguous. It
// refuses mixed or unsupported drift without mutating lifecycle files.
func ReconcileLifecycle(opts Options, mode string) (ReconcileResult, error) {
	if mode != ReconcileModeRepair {
		return ReconcileResult{}, fmt.Errorf("unsupported reconcile mode %q", mode)
	}
	paths := normalizePaths(opts.Paths)
	projectState, tasks, err := load(paths)
	if err != nil {
		return ReconcileResult{}, err
	}
	beforeStatus := discover(paths, projectState, tasks)
	findings := diagnose(paths, projectState, tasks)
	result := ReconcileResult{Diagnostics: Diagnostics{Status: beforeStatus, Findings: findings, AllowedNextActions: diagnosticActions(beforeStatus, findings)}}
	if len(findings) == 0 {
		result.Message = "no lifecycle drift detected"
		return result, nil
	}
	if !allRepairable(findings) {
		result.ManualReview = true
		result.Message = "lifecycle drift is unsupported or ambiguous; no files changed"
		return result, nil
	}

	changedFiles, changedFields, err := repairLifecycleDrift(paths, projectState, tasks, findings, opts)
	if err != nil {
		return ReconcileResult{}, err
	}
	if len(changedFiles) == 0 && len(changedFields) == 0 {
		result.ManualReview = true
		result.Message = "repairable drift could not be applied safely; no files changed"
		return result, nil
	}
	afterStatus := discover(paths, projectState, tasks)
	afterFindings := diagnose(paths, projectState, tasks)
	result.Diagnostics = Diagnostics{Status: afterStatus, Findings: afterFindings, AllowedNextActions: diagnosticActions(afterStatus, afterFindings)}
	result.Repaired = true
	result.ChangedFiles = changedFiles
	result.ChangedFields = changedFields
	result.Message = "lifecycle drift repaired"
	return result, nil
}

// ClaimNext claims the current TODO assignment for an interactive worker. The
// claim writes .doug/ACTIVE_TASK.md and persists the incremented attempt counter
// but deliberately does not write IN_PROGRESS into tasks.yaml, preserving the
// existing headless lifecycle semantics.
func ClaimNext(opts Options) (ClaimResult, error) {
	paths := normalizePaths(opts.Paths)
	projectState, tasks, err := load(paths)
	if err != nil {
		return ClaimResult{}, err
	}

	current := discover(paths, projectState, tasks)
	if current.Kind == StatusActiveTask {
		return ClaimResult{Status: current, AlreadyActive: true, Attempt: current.ActiveTask.Attempts}, nil
	}
	if current.Kind == StatusComplete {
		return ClaimResult{Status: current}, nil
	}

	task, ok := firstTODO(tasks)
	if !ok {
		current.Kind = StatusComplete
		current.AllTasksDone = true
		return ClaimResult{Status: current}, nil
	}

	projectState.ActiveTask = types.TaskPointer{Type: task.Type, ID: task.ID, Attempts: projectState.ActiveTask.Attempts}
	if projectState.ActiveTask.Attempts < 0 {
		projectState.ActiveTask.Attempts = 0
	}
	projectState.ActiveTask.Attempts++
	projectState.NextTask = nextTODOAfter(tasks, task.ID)

	if err := state.SaveProjectState(paths.StatePath, projectState); err != nil {
		return ClaimResult{}, fmt.Errorf("save project state after claim: %w", err)
	}

	logger := opts.Logger
	if logger == nil {
		logger = log.Discard()
	}
	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:             task.ID,
		TaskType:           task.Type,
		DougDir:            paths.DougDir,
		ProjectRoot:        paths.ProjectRoot,
		Description:        task.Description,
		AcceptanceCriteria: task.AcceptanceCriteria,
		Attempts:           projectState.ActiveTask.Attempts,
		MaxRetries:         opts.MaxRetries,
		BuildSystem:        opts.BuildSystem,
	}, logger); err != nil {
		return ClaimResult{}, fmt.Errorf("write active task: %w", err)
	}

	claimed := discover(paths, projectState, tasks)
	return ClaimResult{Status: claimed, Claimed: true, Attempt: projectState.ActiveTask.Attempts}, nil
}

// ApplyVerifiedCompletion mutates in-memory lifecycle state after Doug has
// independently verified a successful task outcome. When markTaskDone is true,
// the task must exist in tasks.yaml and is marked DONE before the coupled
// pointer transition. Handler-injected synthetic tasks can pass false to reuse
// the same pointer advancement without pretending they are backlog entries.
func ApplyVerifiedCompletion(projectState *types.ProjectState, tasks *types.Tasks, taskID string, markTaskDone bool, now time.Time) (CompletionResult, error) {
	if taskID == "" {
		taskID = projectState.ActiveTask.ID
	}
	if taskID == "" {
		return CompletionResult{}, fmt.Errorf("complete verified task: no task id supplied and no active task")
	}
	if markTaskDone {
		if err := types.UpdateTaskStatus(tasks, taskID, types.StatusDone); err != nil {
			return CompletionResult{}, fmt.Errorf("mark task %s done: %w", taskID, err)
		}
	}

	result := CompletionResult{}
	if types.AreAllUserTasksComplete(tasks) {
		completedAt := now.UTC().Format(time.RFC3339)
		projectState.CurrentEpic.CompletedAt = &completedAt
		result.Terminal = true
	} else {
		advanced := types.AdvanceToNextTask(projectState, tasks)
		if !advanced {
			return CompletionResult{}, fmt.Errorf("advance task pointers after %s: no next task but epic is not terminal", taskID)
		}
		result.Advanced = true
	}
	return result, nil
}

// CompleteVerifiedTask persists the coupled state transition after Doug has
// independently verified a successful task outcome: tasks.yaml is updated to
// DONE and project-state.yaml is advanced (or terminal completion is stamped).
func CompleteVerifiedTask(opts Options, taskID string) (CompletionResult, error) {
	paths := normalizePaths(opts.Paths)
	projectState, tasks, err := load(paths)
	if err != nil {
		return CompletionResult{}, err
	}

	if taskID == "" {
		taskID = projectState.ActiveTask.ID
	}
	result, err := ApplyVerifiedCompletion(projectState, tasks, taskID, true, time.Now())
	if err != nil {
		return CompletionResult{}, err
	}
	if err := state.SaveTasks(paths.TasksPath, tasks); err != nil {
		return CompletionResult{}, fmt.Errorf("save tasks after marking %s done: %w", taskID, err)
	}
	if err := state.SaveProjectState(paths.StatePath, projectState); err != nil {
		return CompletionResult{}, fmt.Errorf("save project state after completing %s: %w", taskID, err)
	}

	result.Status = discover(paths, projectState, tasks)
	return result, nil
}

// RecordTaskFailure persists a failed outcome without marking the task DONE. If
// maxRetries has not been reached, the active pointer (including attempts and
// retry diagnostics) is preserved for another attempt. Once maxRetries is
// reached, the task is marked BLOCKED and retained as active for manual review.
func RecordTaskFailure(opts Options, taskID string) (FailureResult, error) {
	paths := normalizePaths(opts.Paths)
	projectState, tasks, err := load(paths)
	if err != nil {
		return FailureResult{}, err
	}
	if taskID == "" {
		taskID = projectState.ActiveTask.ID
	}
	if taskID == "" {
		return FailureResult{}, fmt.Errorf("record task failure: no task id supplied and no active task")
	}

	if opts.MaxRetries <= 0 || projectState.ActiveTask.Attempts < opts.MaxRetries {
		if err := state.SaveProjectState(paths.StatePath, projectState); err != nil {
			return FailureResult{}, fmt.Errorf("save retryable failure state for %s: %w", taskID, err)
		}
		return FailureResult{Status: discover(paths, projectState, tasks), Retryable: true}, nil
	}

	blockedTask := types.TaskPointer{Type: projectState.ActiveTask.Type, ID: taskID}
	if blockedTask.Type == "" {
		for _, task := range tasks.Epic.Tasks {
			if task.ID == taskID {
				blockedTask.Type = task.Type
				break
			}
		}
	}
	if err := ApplyFailedTaskBlock(projectState, tasks, blockedTask); err != nil {
		return FailureResult{}, err
	}
	if err := state.SaveTasks(paths.TasksPath, tasks); err != nil {
		return FailureResult{}, fmt.Errorf("save tasks after blocking %s: %w", taskID, err)
	}
	if err := state.SaveProjectState(paths.StatePath, projectState); err != nil {
		return FailureResult{}, fmt.Errorf("save project state after blocking %s: %w", taskID, err)
	}
	return FailureResult{Status: discover(paths, projectState, tasks), Blocked: true}, nil
}

// ApplyFailedTaskBlock marks a backlog task BLOCKED and leaves that blocked
// task as the active pointer for manual review, clearing next_task.
func ApplyFailedTaskBlock(projectState *types.ProjectState, tasks *types.Tasks, blockedTask types.TaskPointer) error {
	if blockedTask.ID == "" {
		return fmt.Errorf("mark task blocked: no task id supplied")
	}
	if err := types.UpdateTaskStatus(tasks, blockedTask.ID, types.StatusBlocked); err != nil {
		return fmt.Errorf("mark task %s blocked: %w", blockedTask.ID, err)
	}
	active := projectState.ActiveTask
	active.Type = blockedTask.Type
	active.ID = blockedTask.ID
	projectState.ActiveTask = active
	projectState.NextTask = types.TaskPointer{}
	return nil
}

// ApplyEpicFinalized mutates in-memory lifecycle state for terminal epic
// finalization: completed_at is guaranteed and runtime task pointers are
// cleared together.
func ApplyEpicFinalized(projectState *types.ProjectState, now time.Time) {
	if projectState.CurrentEpic.CompletedAt == nil || *projectState.CurrentEpic.CompletedAt == "" {
		completedAt := now.UTC().Format(time.RFC3339)
		projectState.CurrentEpic.CompletedAt = &completedAt
	}
	projectState.ActiveTask = types.TaskPointer{}
	projectState.NextTask = types.TaskPointer{}
}

// FinalizeEpic persists the terminal epic finalization transition after all
// user tasks are DONE: backlog metadata is marked COMPLETED via the shared plan
// finalizer, the runtime snapshot is archived, and runtime task pointers are
// cleared together in project-state.yaml.
func FinalizeEpic(opts Options) (FinalizationResult, error) {
	paths := normalizePaths(opts.Paths)
	projectState, tasks, err := load(paths)
	if err != nil {
		return FinalizationResult{}, err
	}
	if !types.AreAllUserTasksComplete(tasks) {
		return FinalizationResult{}, fmt.Errorf("finalize epic %s: user tasks are not all DONE", projectState.CurrentEpic.ID)
	}
	if projectState.CurrentEpic.CompletedAt == nil || *projectState.CurrentEpic.CompletedAt == "" {
		completedAt := time.Now().UTC().Format(time.RFC3339)
		projectState.CurrentEpic.CompletedAt = &completedAt
		if err := state.SaveProjectState(paths.StatePath, projectState); err != nil {
			return FinalizationResult{}, fmt.Errorf("save completed_at for %s: %w", projectState.CurrentEpic.ID, err)
		}
	}

	archiveDir, err := plan.FinalizeEpicCompletion(paths.ProjectRoot, projectState.CurrentEpic, *projectState.CurrentEpic.CompletedAt)
	if err != nil {
		return FinalizationResult{}, fmt.Errorf("finalize epic %s: %w", projectState.CurrentEpic.ID, err)
	}
	ApplyEpicFinalized(projectState, time.Now())
	if err := state.SaveProjectState(paths.StatePath, projectState); err != nil {
		return FinalizationResult{}, fmt.Errorf("save finalized state for %s: %w", projectState.CurrentEpic.ID, err)
	}

	return FinalizationResult{Status: discover(paths, projectState, tasks), ArchiveDir: archiveDir}, nil
}

func normalizePaths(paths Paths) Paths {
	if paths.ProjectRoot == "" {
		paths.ProjectRoot = "."
	}
	if paths.DougDir == "" {
		paths.DougDir = filepath.Join(paths.ProjectRoot, ".doug")
	}
	if paths.StatePath == "" {
		paths.StatePath = filepath.Join(paths.DougDir, "project-state.yaml")
	}
	if paths.TasksPath == "" {
		paths.TasksPath = filepath.Join(paths.DougDir, "tasks.yaml")
	}
	return paths
}

func load(paths Paths) (*types.ProjectState, *types.Tasks, error) {
	projectState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return nil, nil, fmt.Errorf("project state not found at %s", paths.StatePath)
		}
		return nil, nil, fmt.Errorf("load project state: %w", err)
	}
	tasks, err := state.LoadTasks(paths.TasksPath)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return nil, nil, fmt.Errorf("tasks file not found at %s", paths.TasksPath)
		}
		return nil, nil, fmt.Errorf("load tasks: %w", err)
	}
	return projectState, tasks, nil
}

func discover(paths Paths, projectState *types.ProjectState, tasks *types.Tasks) Status {
	activePath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
	st := Status{
		EpicID:         projectState.CurrentEpic.ID,
		ActiveTask:     projectState.ActiveTask,
		NextTask:       projectState.NextTask,
		ActiveTaskPath: activePath,
		AllTasksDone:   types.AreAllUserTasksComplete(tasks),
	}
	if st.AllTasksDone {
		st.Kind = StatusComplete
		return st
	}
	if projectState.ActiveTask.ID != "" && fileExists(activePath) {
		st.Kind = StatusActiveTask
		return st
	}
	st.Kind = StatusNoActiveTask
	return st
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func diagnose(paths Paths, projectState *types.ProjectState, tasks *types.Tasks) []DiagnosticFinding {
	activePath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
	findings := []DiagnosticFinding{}
	taskByID := map[string]types.Task{}
	for _, task := range tasks.Epic.Tasks {
		taskByID[task.ID] = task
	}

	active := projectState.ActiveTask
	if active.ID != "" {
		task, ok := taskByID[active.ID]
		if !ok {
			findings = append(findings, DiagnosticFinding{Code: "ACTIVE_POINTER_TASK_MISSING", Severity: "error", Path: paths.StatePath, RequiresManualReview: true, Message: fmt.Sprintf("active_task points to %q, but that task is not present in tasks.yaml", active.ID)})
		} else if task.Status != types.StatusTODO {
			code := "ACTIVE_POINTER_STATUS_MISMATCH"
			message := fmt.Sprintf("active_task points to %q, but tasks.yaml marks it %q", active.ID, task.Status)
			if task.Status == types.StatusDone {
				code = "COMPLETED_TASK_POINTER_DRIFT"
				message = fmt.Sprintf("active_task still points to completed task %q", active.ID)
			}
			findings = append(findings, DiagnosticFinding{Code: code, Severity: "error", Path: paths.StatePath, RequiresManualReview: true, Message: message})
		}
	}

	if active.ID != "" && !fileExists(activePath) {
		findings = append(findings, DiagnosticFinding{Code: "ACTIVE_BRIEF_MISSING", Severity: "error", Path: activePath, RequiresManualReview: true, Message: fmt.Sprintf("active_task points to %q, but .doug/ACTIVE_TASK.md is missing", active.ID)})
	}

	if fileExists(activePath) {
		briefID, err := readActiveBriefTaskID(activePath)
		if err != nil {
			findings = append(findings, DiagnosticFinding{Code: "ACTIVE_BRIEF_UNREADABLE", Severity: "error", Path: activePath, RequiresManualReview: true, Message: err.Error()})
		} else if briefID == "" {
			findings = append(findings, DiagnosticFinding{Code: "ACTIVE_BRIEF_TASK_ID_MISSING", Severity: "error", Path: activePath, RequiresManualReview: true, Message: ".doug/ACTIVE_TASK.md does not contain a parseable **Task ID** line"})
		} else if active.ID == "" {
			findings = append(findings, DiagnosticFinding{Code: "ORPHANED_ACTIVE_BRIEF", Severity: "warning", Path: activePath, RequiresManualReview: true, Message: fmt.Sprintf(".doug/ACTIVE_TASK.md contains task %q, but project-state.yaml has no active_task", briefID)})
		} else if briefID != active.ID {
			briefTask, briefKnown := taskByID[briefID]
			activeTask, activeKnown := taskByID[active.ID]
			if briefKnown && activeKnown && briefTask.Status == types.StatusDone && activeTask.Status == types.StatusTODO {
				findings = append(findings, DiagnosticFinding{Code: "STALE_ACTIVE_BRIEF", Severity: "error", Path: activePath, RequiresManualReview: true, Message: fmt.Sprintf(".doug/ACTIVE_TASK.md still contains completed task %q while active_task has advanced to %q", briefID, active.ID)})
			} else {
				findings = append(findings, DiagnosticFinding{Code: "AMBIGUOUS_ACTIVE_BRIEF_DRIFT", Severity: "error", Path: activePath, RequiresManualReview: true, Message: fmt.Sprintf(".doug/ACTIVE_TASK.md contains task %q while active_task points to %q; Doug cannot infer the correct assignment safely", briefID, active.ID)})
			}
		}
	}

	if active.ID != "" {
		expectedNext := nextTODOAfter(tasks, active.ID)
		if projectState.NextTask.ID != expectedNext.ID {
			findings = append(findings, DiagnosticFinding{Code: "ACTIVE_NEXT_POINTER_MISMATCH", Severity: "error", Path: paths.StatePath, RequiresManualReview: true, Message: fmt.Sprintf("next_task is %q, but expected next TODO after %q is %q", projectState.NextTask.ID, active.ID, expectedNext.ID)})
		}
	}
	return findings
}

var activeBriefTaskIDPattern = regexp.MustCompile(`(?m)^\*\*Task ID\*\*:\s*([^\s]+)\s*$`)

func readActiveBriefTaskID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read active task brief: %w", err)
	}
	match := activeBriefTaskIDPattern.FindStringSubmatch(string(data))
	if len(match) < 2 {
		return "", nil
	}
	return strings.Trim(match[1], "`\"'"), nil
}

func allRepairable(findings []DiagnosticFinding) bool {
	if len(findings) == 0 {
		return false
	}
	for _, finding := range findings {
		switch finding.Code {
		case "ACTIVE_BRIEF_MISSING", "STALE_ACTIVE_BRIEF", "ACTIVE_NEXT_POINTER_MISMATCH":
			continue
		default:
			return false
		}
	}
	return true
}

func repairLifecycleDrift(paths Paths, projectState *types.ProjectState, tasks *types.Tasks, findings []DiagnosticFinding, opts Options) ([]ChangedFile, []ChangedField, error) {
	var changedFiles []ChangedFile
	var changedFields []ChangedField
	needsBriefRewrite := false
	needsNextRepair := false
	for _, finding := range findings {
		switch finding.Code {
		case "ACTIVE_BRIEF_MISSING", "STALE_ACTIVE_BRIEF":
			needsBriefRewrite = true
		case "ACTIVE_NEXT_POINTER_MISMATCH":
			needsNextRepair = true
		}
	}
	activeTask, ok := findTask(tasks, projectState.ActiveTask.ID)
	if !ok || activeTask.Status != types.StatusTODO {
		return nil, nil, nil
	}
	if needsNextRepair {
		expected := nextTODOAfter(tasks, projectState.ActiveTask.ID)
		before := projectState.NextTask
		if before.Type != expected.Type || before.ID != expected.ID || before.Attempts != expected.Attempts {
			projectState.NextTask = expected
			if err := state.SaveProjectState(paths.StatePath, projectState); err != nil {
				return nil, nil, fmt.Errorf("save project state after lifecycle repair: %w", err)
			}
			changedFiles = appendChangedFile(changedFiles, ChangedFile{Path: paths.StatePath, Action: "updated"})
			changedFields = append(changedFields, ChangedField{Path: paths.StatePath, Field: "next_task", Before: pointerString(before), After: pointerString(expected)})
		}
	}
	if needsBriefRewrite {
		logger := opts.Logger
		if logger == nil {
			logger = log.Discard()
		}
		if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
			TaskID:             activeTask.ID,
			TaskType:           activeTask.Type,
			DougDir:            paths.DougDir,
			ProjectRoot:        paths.ProjectRoot,
			Description:        activeTask.Description,
			AcceptanceCriteria: activeTask.AcceptanceCriteria,
			Attempts:           projectState.ActiveTask.Attempts,
			MaxRetries:         opts.MaxRetries,
			BuildSystem:        opts.BuildSystem,
		}, logger); err != nil {
			return nil, nil, fmt.Errorf("rewrite active task during lifecycle repair: %w", err)
		}
		changedFiles = appendChangedFile(changedFiles, ChangedFile{Path: filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), Action: "rewritten"})
		changedFields = append(changedFields, ChangedField{Path: filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), Field: "active_brief.task_id", Before: "stale-or-missing", After: activeTask.ID})
	}
	return changedFiles, changedFields, nil
}

func findTask(tasks *types.Tasks, id string) (types.Task, bool) {
	for _, task := range tasks.Epic.Tasks {
		if task.ID == id {
			return task, true
		}
	}
	return types.Task{}, false
}

func appendChangedFile(files []ChangedFile, file ChangedFile) []ChangedFile {
	for _, existing := range files {
		if existing.Path == file.Path {
			return files
		}
	}
	return append(files, file)
}

func pointerString(pointer types.TaskPointer) string {
	if pointer.ID == "" && pointer.Type == "" {
		return "<empty>"
	}
	return fmt.Sprintf("%s:%s", pointer.Type, pointer.ID)
}

func diagnosticActions(status Status, findings []DiagnosticFinding) []string {
	if len(findings) > 0 {
		if allRepairable(findings) {
			return []string{DiagnosticActionGetStatus, DiagnosticActionReconcileRepair}
		}
		return []string{DiagnosticActionGetStatus, DiagnosticActionManualReview}
	}
	if status.Kind == StatusActiveTask {
		return []string{DiagnosticActionReportTaskComplete, DiagnosticActionReportTaskBlocked}
	}
	if status.AllTasksDone {
		return []string{DiagnosticActionGetStatus}
	}
	return []string{DiagnosticActionGetNextTask}
}

func firstTODO(tasks *types.Tasks) (types.Task, bool) {
	for _, task := range tasks.Epic.Tasks {
		if task.Status == types.StatusTODO {
			return task, true
		}
	}
	return types.Task{}, false
}

func nextTODOAfter(tasks *types.Tasks, activeID string) types.TaskPointer {
	foundActive := false
	for _, task := range tasks.Epic.Tasks {
		if foundActive && task.Status == types.StatusTODO {
			return types.TaskPointer{Type: task.Type, ID: task.ID}
		}
		if task.ID == activeID {
			foundActive = true
		}
	}
	return types.TaskPointer{}
}
