// Package mcp implements Doug's local stdio MCP tool handlers.
package mcp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/build"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/lifecycle"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/runlock"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

const (
	ToolGetStatus          = "get_status"
	ToolDiagnoseLifecycle  = "diagnose_lifecycle"
	ToolGetNextTask        = "get_next_task"
	ToolReportTaskComplete = "report_task_complete"
	ToolReportTaskBlocked  = "report_task_blocked"
)

const dispatcherWorkerGuidance = "Dispatcher/worker context hygiene: use this MCP session only as a thin dispatcher; hand the canonical .doug/ACTIVE_TASK.md brief to a fresh worker context for the task; after the worker fills ## Agent Result, report completion through Doug; start a fresh dispatcher for each epic."

// ToolHandler owns testable tool semantics independently from JSON-RPC stdio glue.
type ToolHandler struct {
	ProjectRoot   string
	Config        *config.OrchestratorConfig
	Logger        log.Logger
	BuildSystem   build.BuildSystem
	HandleSuccess func(ctx *types.LoopContext, result *types.SessionResult, agentDurationSeconds int) (handlers.SuccessResult, error)
}

type Assignment struct {
	ID       string         `json:"id,omitempty"`
	Type     types.TaskType `json:"type,omitempty"`
	Attempts int            `json:"attempts,omitempty"`
}

type StatusResponse struct {
	CurrentEpic        string      `json:"current_epic"`
	LifecyclePhase     string      `json:"lifecycle_phase"`
	ActiveAssignment   *Assignment `json:"active_assignment,omitempty"`
	NextAssignment     *Assignment `json:"next_assignment,omitempty"`
	BriefPath          string      `json:"brief_path"`
	AttemptCount       int         `json:"attempt_count"`
	Blocked            bool        `json:"blocked"`
	Completed          bool        `json:"completed"`
	AllowedNextActions []string    `json:"allowed_next_actions"`
}

type DiagnosticFinding struct {
	Code                 string `json:"code"`
	Severity             string `json:"severity"`
	Message              string `json:"message"`
	Path                 string `json:"path,omitempty"`
	RequiresManualReview bool   `json:"requires_manual_review"`
}

type DiagnosticsResponse struct {
	StatusResponse
	Findings []DiagnosticFinding `json:"findings"`
}

type NextTaskResponse struct {
	StatusResponse
	Brief              string `json:"brief"`
	DispatcherGuidance string `json:"dispatcher_worker_guidance"`
	AlreadyActive      bool   `json:"already_active"`
	Claimed            bool   `json:"claimed"`
}

type ReportResponse struct {
	StatusResponse
	Outcome string `json:"outcome"`
	Message string `json:"message"`
}

func (h ToolHandler) GetStatus() (StatusResponse, error) {
	st, err := lifecycle.DiscoverStatus(h.lifecycleOptions())
	if err != nil {
		return StatusResponse{}, err
	}
	return h.statusResponse(st), nil
}

func (h ToolHandler) DiagnoseLifecycle() (DiagnosticsResponse, error) {
	diagnostics, err := lifecycle.DiagnoseLifecycle(h.lifecycleOptions())
	if err != nil {
		return DiagnosticsResponse{}, err
	}
	resp := DiagnosticsResponse{StatusResponse: h.statusResponse(diagnostics.Status)}
	resp.AllowedNextActions = diagnostics.AllowedNextActions
	for _, finding := range diagnostics.Findings {
		resp.Findings = append(resp.Findings, DiagnosticFinding{Code: finding.Code, Severity: finding.Severity, Message: finding.Message, Path: finding.Path, RequiresManualReview: finding.RequiresManualReview})
	}
	return resp, nil
}

func (h ToolHandler) GetNextTask() (NextTaskResponse, error) {
	var resp NextTaskResponse
	err := h.withRunLock("mcp get_next_task", func() error {
		claim, err := lifecycle.ClaimNext(h.lifecycleOptions())
		if err != nil {
			return err
		}
		if claim.Status.Kind == lifecycle.StatusComplete && claim.Status.ActiveTask.ID == "" {
			lifecycleClaim, lifecycleErr := h.claimPostEpicLifecycleWork()
			if lifecycleErr != nil {
				return lifecycleErr
			}
			if lifecycleClaim.Claimed {
				claim = lifecycleClaim
			}
		}
		resp = NextTaskResponse{
			StatusResponse:     h.statusResponse(claim.Status),
			DispatcherGuidance: dispatcherWorkerGuidance,
			AlreadyActive:      claim.AlreadyActive,
			Claimed:            claim.Claimed,
		}
		data, err := os.ReadFile(claim.ActiveTaskPath)
		if err == nil {
			resp.Brief = string(data)
		} else if claim.Status.Kind == lifecycle.StatusActiveTask {
			return fmt.Errorf("read active task brief: %w", err)
		}
		return nil
	})
	return resp, err
}

func (h ToolHandler) ReportTaskComplete(taskID string) (ReportResponse, error) {
	var resp ReportResponse
	err := h.withRunLock("mcp report_task_complete", func() error {
		var err error
		resp, err = h.reportTaskCompleteLocked(taskID)
		return err
	})
	return resp, err
}

func (h ToolHandler) reportTaskCompleteLocked(taskID string) (ReportResponse, error) {
	paths := h.paths()
	result, err := agent.ParseSessionResult(filepath.Join(paths.DougDir, "ACTIVE_TASK.md"))
	if err != nil {
		return ReportResponse{}, fmt.Errorf("parse ACTIVE_TASK.md result: %w", err)
	}
	if result.Outcome != types.OutcomeSuccess && result.Outcome != types.OutcomeEpicComplete {
		return ReportResponse{}, fmt.Errorf("report_task_complete requires SUCCESS or EPIC_COMPLETE result, got %q", result.Outcome)
	}
	projectState, tasks, err := loadStateAndTasks(paths)
	if err != nil {
		return ReportResponse{}, err
	}
	if taskID == "" {
		taskID = projectState.ActiveTask.ID
	}
	if taskID == "" || taskID != projectState.ActiveTask.ID {
		return ReportResponse{}, fmt.Errorf("task id %q does not match active task %q", taskID, projectState.ActiveTask.ID)
	}
	bs, err := h.buildSystem()
	if err != nil {
		return ReportResponse{}, err
	}
	ctx := &types.LoopContext{
		TaskID:        taskID,
		TaskType:      projectState.ActiveTask.Type,
		Attempts:      projectState.ActiveTask.Attempts,
		CurrentEpic:   projectState.CurrentEpic,
		Config:        h.config(),
		BuildSystem:   bs,
		ProjectRoot:   paths.ProjectRoot,
		TaskStartTime: time.Now(),
		State:         projectState,
		Tasks:         tasks,
		StatePath:     paths.StatePath,
		TasksPath:     paths.TasksPath,
		DougDir:       paths.DougDir,
		LogsDir:       filepath.Join(paths.DougDir, "logs"),
		ChangelogPath: filepath.Join(paths.ProjectRoot, "CHANGELOG.md"),
		Logger:        h.logger(),
	}
	success := h.HandleSuccess
	if success == nil {
		success = handlers.HandleSuccess
	}
	kind, err := success(ctx, result, 0)
	if err != nil {
		return ReportResponse{}, fmt.Errorf("verified success handler: %w", err)
	}
	st := lifecycle.Status{EpicID: projectState.CurrentEpic.ID, ActiveTask: projectState.ActiveTask, NextTask: projectState.NextTask, ActiveTaskPath: filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), AllTasksDone: types.AreAllUserTasksComplete(tasks)}
	if st.AllTasksDone {
		st.Kind = lifecycle.StatusComplete
	} else if projectState.ActiveTask.ID != "" {
		st.Kind = lifecycle.StatusNoActiveTask
	}
	return ReportResponse{StatusResponse: h.statusResponse(st), Outcome: string(result.Outcome), Message: fmt.Sprintf("verified success path returned %v", kind.Kind)}, nil
}

func (h ToolHandler) claimPostEpicLifecycleWork() (lifecycle.ClaimResult, error) {
	cfg := h.config()
	if !cfg.ReviewEnabled && !cfg.KBEnabled {
		return lifecycle.ClaimResult{}, nil
	}
	paths := h.paths()
	projectState, tasks, err := loadStateAndTasks(paths)
	if err != nil {
		return lifecycle.ClaimResult{}, err
	}
	if !types.AreAllUserTasksComplete(tasks) {
		return lifecycle.ClaimResult{}, nil
	}
	taskID := "POST_EPIC_REVIEW"
	description := "Review the completed epic for acceptance-criteria faithfulness, likely regressions, implementation coherence, and release readiness."
	criteria := []string{"Fill in the post-epic review result in `.doug/ACTIVE_TASK.md`.", "Do not modify project code while performing the advisory review.", "Report `SUCCESS` in the result block when the review is complete."}
	if !cfg.ReviewEnabled && cfg.KBEnabled {
		taskID = "POST_EPIC_KB"
		description = "Synthesize post-epic KB and changelog updates from Doug-owned artifacts."
		criteria = []string{"Update relevant `docs/kb/` articles and `CHANGELOG.md` from completed epic evidence.", "Keep the result flowing through `.doug/ACTIVE_TASK.md`.", "Report `SUCCESS` in the result block when synthesis is complete."}
	}
	projectState.ActiveTask = types.TaskPointer{Type: types.TaskTypeDocumentation, ID: taskID, Attempts: 1}
	projectState.NextTask = types.TaskPointer{}
	if err := state.SaveProjectState(paths.StatePath, projectState); err != nil {
		return lifecycle.ClaimResult{}, fmt.Errorf("save post-epic lifecycle claim: %w", err)
	}
	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{TaskID: taskID, TaskType: types.TaskTypeDocumentation, DougDir: paths.DougDir, ProjectRoot: paths.ProjectRoot, Description: description, AcceptanceCriteria: criteria, Attempts: 1, MaxRetries: 1, BuildSystem: cfg.BuildSystem, ContextSections: []agent.ActiveTaskSection{{Heading: "Interactive Lifecycle Work", Body: dispatcherWorkerGuidance}}}, h.logger()); err != nil {
		return lifecycle.ClaimResult{}, fmt.Errorf("write post-epic lifecycle active task: %w", err)
	}
	st := lifecycle.Status{Kind: lifecycle.StatusActiveTask, EpicID: projectState.CurrentEpic.ID, ActiveTask: projectState.ActiveTask, ActiveTaskPath: filepath.Join(paths.DougDir, "ACTIVE_TASK.md"), AllTasksDone: true}
	return lifecycle.ClaimResult{Status: st, Claimed: true, Attempt: 1}, nil
}

func (h ToolHandler) ReportTaskBlocked(taskID string) (ReportResponse, error) {
	var resp ReportResponse
	err := h.withRunLock("mcp report_task_blocked", func() error {
		var err error
		resp, err = h.reportTaskBlockedLocked(taskID)
		return err
	})
	return resp, err
}

func (h ToolHandler) reportTaskBlockedLocked(taskID string) (ReportResponse, error) {
	paths := h.paths()
	result, err := agent.ParseSessionResult(filepath.Join(paths.DougDir, "ACTIVE_TASK.md"))
	if err != nil {
		return ReportResponse{}, fmt.Errorf("parse ACTIVE_TASK.md result: %w", err)
	}
	if result.Outcome != types.OutcomeFailure {
		return ReportResponse{}, fmt.Errorf("report_task_blocked requires FAILURE result, got %q", result.Outcome)
	}
	failure, err := lifecycle.RecordTaskFailure(h.lifecycleOptions(), taskID)
	if err != nil {
		return ReportResponse{}, err
	}
	msg := "task failure recorded; retry remains allowed"
	if failure.Blocked {
		msg = "task blocked for manual review"
	}
	return ReportResponse{StatusResponse: h.statusResponse(failure.Status), Outcome: string(result.Outcome), Message: msg}, nil
}

func (h ToolHandler) withRunLock(owner string, fn func() error) error {
	paths := h.paths()
	lock, err := runlock.TryAcquire(paths.DougDir, owner)
	if err != nil {
		if errors.Is(err, runlock.ErrHeld) {
			return fmt.Errorf("another Doug lifecycle driver is already active: %w (%s)", err, runlock.Path(paths.DougDir))
		}
		return err
	}
	defer func() { _ = lock.Close() }()
	return fn()
}

func (h ToolHandler) lifecycleOptions() lifecycle.Options {
	cfg := h.config()
	return lifecycle.Options{Paths: h.paths(), MaxRetries: cfg.MaxRetries, BuildSystem: cfg.BuildSystem, Logger: h.logger()}
}

func (h ToolHandler) paths() lifecycle.Paths {
	root := h.ProjectRoot
	if root == "" {
		root, _ = os.Getwd()
	}
	return lifecycle.DefaultPaths(root)
}

func (h ToolHandler) config() *config.OrchestratorConfig {
	if h.Config != nil {
		return h.Config
	}
	return &config.OrchestratorConfig{BuildSystem: config.DefaultBuildSystem, MaxRetries: config.DefaultMaxRetries}
}

func (h ToolHandler) logger() log.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return log.Discard()
}

func (h ToolHandler) buildSystem() (build.BuildSystem, error) {
	if h.BuildSystem != nil {
		return h.BuildSystem, nil
	}
	cfg := h.config()
	return build.NewBuildSystem(cfg.BuildSystem, filepath.Join(h.paths().ProjectRoot, cfg.ModuleRoot))
}

func (h ToolHandler) statusResponse(st lifecycle.Status) StatusResponse {
	resp := StatusResponse{CurrentEpic: st.EpicID, LifecyclePhase: string(st.Kind), BriefPath: st.ActiveTaskPath, AttemptCount: st.ActiveTask.Attempts, Completed: st.AllTasksDone}
	if st.ActiveTask.ID != "" {
		resp.ActiveAssignment = &Assignment{ID: st.ActiveTask.ID, Type: st.ActiveTask.Type, Attempts: st.ActiveTask.Attempts}
	}
	if st.NextTask.ID != "" {
		resp.NextAssignment = &Assignment{ID: st.NextTask.ID, Type: st.NextTask.Type, Attempts: st.NextTask.Attempts}
	}
	resp.Blocked = activeTaskBlocked(h.paths(), st.ActiveTask.ID)
	switch {
	case st.Kind == lifecycle.StatusActiveTask:
		resp.AllowedNextActions = []string{ToolReportTaskComplete, ToolReportTaskBlocked}
	case resp.Completed:
		resp.AllowedNextActions = []string{}
	default:
		resp.AllowedNextActions = []string{ToolGetNextTask}
	}
	return resp
}

func activeTaskBlocked(paths lifecycle.Paths, id string) bool {
	if id == "" {
		return false
	}
	tasks, err := state.LoadTasks(paths.TasksPath)
	if err != nil {
		return false
	}
	for _, task := range tasks.Epic.Tasks {
		if task.ID == id {
			return task.Status == types.StatusBlocked
		}
	}
	return false
}

func loadStateAndTasks(paths lifecycle.Paths) (*types.ProjectState, *types.Tasks, error) {
	projectState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		return nil, nil, fmt.Errorf("load project state: %w", err)
	}
	tasks, err := state.LoadTasks(paths.TasksPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load tasks: %w", err)
	}
	return projectState, tasks, nil
}

func ToolNames() []string {
	return []string{ToolGetStatus, ToolDiagnoseLifecycle, ToolGetNextTask, ToolReportTaskComplete, ToolReportTaskBlocked}
}

func IsTool(name string) bool {
	for _, n := range ToolNames() {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return false
}
