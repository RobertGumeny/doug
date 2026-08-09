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
	ToolReconcileLifecycle = "reconcile_lifecycle"
	ToolGetNextTask        = "get_next_task"
	ToolReportTaskComplete = "report_task_complete"
	ToolReportTaskBlocked  = "report_task_blocked"
)

const (
	dispatcherWorkerGuidance = "Dispatcher/worker context hygiene: use this MCP session only as a thin dispatcher; hand the canonical .doug/ACTIVE_TASK.md brief to a fresh worker context for the task; after the worker fills ## Agent Result, report completion through Doug; start a fresh dispatcher for each epic."
	dispatcherInstruction    = "Hand .doug/ACTIVE_TASK.md to a fresh worker, then report the filled Result block through Doug."
	terminalGuidance         = "This assignment is terminal for the current worker context: stop here, or renew context before requesting another task."
	manualReviewGuidance     = "Manual review required: open scoped maintenance or bugfix work for this lifecycle drift instead of editing Doug lifecycle files by hand."
)

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
	CurrentEpic        string       `json:"current_epic"`
	LifecyclePhase     string       `json:"lifecycle_phase"`
	ActiveAssignment   *Assignment  `json:"active_assignment,omitempty"`
	NextAssignment     *Assignment  `json:"next_assignment,omitempty"`
	BriefPath          string       `json:"brief_path"`
	AttemptCount       int          `json:"attempt_count"`
	Blocked            bool         `json:"blocked"`
	Completed          bool         `json:"completed"`
	AllowedNextActions []string     `json:"allowed_next_actions"`
	Health             StatusHealth `json:"health"`
}

type StatusHealth struct {
	Healthy  bool                `json:"healthy"`
	Findings []DiagnosticFinding `json:"findings,omitempty"`
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

type ChangedFile struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

type ChangedField struct {
	Path   string `json:"path"`
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type ReconcileResponse struct {
	DiagnosticsResponse
	Repaired      bool           `json:"repaired"`
	ManualReview  bool           `json:"manual_review"`
	ChangedFiles  []ChangedFile  `json:"changed_files"`
	ChangedFields []ChangedField `json:"changed_fields"`
	Message       string         `json:"message"`
}

type NextTaskResponse struct {
	StatusResponse
	Brief                 string `json:"brief"`
	AssignmentBriefPath   string `json:"assignment_brief_path"`
	DispatcherInstruction string `json:"dispatcher_instruction"`
	DispatcherGuidance    string `json:"dispatcher_worker_guidance"`
	AlreadyActive         bool   `json:"already_active"`
	Claimed               bool   `json:"claimed"`
}

type ReportResponse struct {
	StatusResponse
	Outcome           string `json:"outcome"`
	SuccessResultKind string `json:"success_result_kind,omitempty"`
	Message           string `json:"message"`
	TerminalGuidance  string `json:"terminal_guidance"`
}

// ToolDefinition describes a Doug MCP tool for tools/list self-discovery.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func (h ToolHandler) GetStatus() (StatusResponse, error) {
	diagnostics, err := lifecycle.DiagnoseLifecycle(h.lifecycleOptions())
	if err != nil {
		return StatusResponse{}, err
	}
	resp := h.statusResponse(diagnostics.Status)
	resp.Health = statusHealth(diagnostics.Findings)
	if len(diagnostics.Findings) > 0 {
		resp.AllowedNextActions = diagnostics.AllowedNextActions
	}
	return resp, nil
}

func (h ToolHandler) DiagnoseLifecycle() (DiagnosticsResponse, error) {
	diagnostics, err := lifecycle.DiagnoseLifecycle(h.lifecycleOptions())
	if err != nil {
		return DiagnosticsResponse{}, err
	}
	return h.diagnosticsResponse(diagnostics), nil
}

func (h ToolHandler) ReconcileLifecycle(mode string) (ReconcileResponse, error) {
	var resp ReconcileResponse
	err := h.withRunLock("mcp reconcile_lifecycle", func() error {
		result, err := lifecycle.ReconcileLifecycle(h.lifecycleOptions(), mode)
		if err != nil {
			return err
		}
		message := result.Message
		if result.ManualReview && !strings.Contains(message, manualReviewGuidance) {
			message = strings.TrimSpace(message) + "; " + manualReviewGuidance
		}
		resp = ReconcileResponse{DiagnosticsResponse: h.diagnosticsResponse(result.Diagnostics), Repaired: result.Repaired, ManualReview: result.ManualReview, Message: message}
		for _, file := range result.ChangedFiles {
			resp.ChangedFiles = append(resp.ChangedFiles, ChangedFile{Path: file.Path, Action: file.Action})
		}
		for _, field := range result.ChangedFields {
			resp.ChangedFields = append(resp.ChangedFields, ChangedField{Path: field.Path, Field: field.Field, Before: field.Before, After: field.After})
		}
		return nil
	})
	return resp, err
}

func (h ToolHandler) GetNextTask() (NextTaskResponse, error) {
	var resp NextTaskResponse
	err := h.withRunLock("mcp get_next_task", func() error {
		claim, err := lifecycle.ClaimNext(h.lifecycleOptions())
		if err != nil {
			return err
		}
		if claim.Kind == lifecycle.StatusComplete && claim.ActiveTask.ID == "" {
			lifecycleClaim, lifecycleErr := h.claimPostEpicLifecycleWork()
			if lifecycleErr != nil {
				return lifecycleErr
			}
			if lifecycleClaim.Claimed {
				claim = lifecycleClaim
			}
		}
		resp = NextTaskResponse{
			StatusResponse:        h.statusResponse(claim.Status),
			AssignmentBriefPath:   claim.ActiveTaskPath,
			DispatcherInstruction: dispatcherInstruction,
			DispatcherGuidance:    dispatcherWorkerGuidance,
			AlreadyActive:         claim.AlreadyActive,
			Claimed:               claim.Claimed,
		}
		data, err := os.ReadFile(claim.ActiveTaskPath)
		if err == nil {
			resp.Brief = string(data)
		} else if claim.Kind == lifecycle.StatusActiveTask {
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
		return ReportResponse{}, outcomeMismatchError(ToolReportTaskComplete, result.Outcome)
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
	message := reportTaskCompleteMessage(kind.Kind)
	if kind.Kind == handlers.EpicComplete {
		if err := handlers.HandleEpicComplete(ctx); err != nil {
			return ReportResponse{}, err
		}
	}
	st, err := lifecycle.DiscoverStatus(h.lifecycleOptions())
	if err != nil {
		return ReportResponse{}, fmt.Errorf("discover status after completion: %w", err)
	}
	return ReportResponse{StatusResponse: h.statusResponse(st), Outcome: string(result.Outcome), SuccessResultKind: successResultKindName(kind.Kind), Message: message, TerminalGuidance: terminalGuidance}, nil
}

func successResultKindName(kind handlers.SuccessResultKind) string {
	switch kind {
	case handlers.Continue:
		return "continue"
	case handlers.Retry:
		return "retry"
	case handlers.EpicComplete:
		return "epic_complete"
	case handlers.BuildFailure:
		return "build_failure"
	default:
		return fmt.Sprintf("unknown_%d", kind)
	}
}

func reportTaskCompleteMessage(kind handlers.SuccessResultKind) string {
	switch kind {
	case handlers.EpicComplete:
		return "terminal task completed and epic finalized"
	case handlers.Continue:
		return "task completed and lifecycle advanced"
	case handlers.Retry:
		return "verified success path requested retry"
	case handlers.BuildFailure:
		return "verified success path paused on build failure"
	default:
		return fmt.Sprintf("verified success path returned %s", successResultKindName(kind))
	}
}

func outcomeMismatchError(tool string, outcome types.Outcome) error {
	switch tool {
	case ToolReportTaskComplete:
		if outcome == types.OutcomeFailure {
			return fmt.Errorf("report_task_complete requires SUCCESS or EPIC_COMPLETE result, got %q; use report_task_blocked for FAILURE results", outcome)
		}
		return fmt.Errorf("report_task_complete requires SUCCESS or EPIC_COMPLETE result, got %q; interactive MCP completion does not accept this outcome", outcome)
	case ToolReportTaskBlocked:
		if outcome == types.OutcomeSuccess || outcome == types.OutcomeEpicComplete {
			return fmt.Errorf("report_task_blocked requires FAILURE result, got %q; use report_task_complete for SUCCESS or EPIC_COMPLETE results", outcome)
		}
		return fmt.Errorf("report_task_blocked requires FAILURE result, got %q; interactive MCP blockage does not accept this outcome", outcome)
	default:
		return fmt.Errorf("%s cannot report outcome %q", tool, outcome)
	}
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
		return ReportResponse{}, outcomeMismatchError(ToolReportTaskBlocked, result.Outcome)
	}
	failure, err := lifecycle.RecordTaskFailure(h.lifecycleOptions(), taskID)
	if err != nil {
		return ReportResponse{}, err
	}
	msg := "task failure recorded; retry remains allowed"
	if failure.Blocked {
		msg = "task blocked for manual review"
	}
	return ReportResponse{StatusResponse: h.statusResponse(failure.Status), Outcome: string(result.Outcome), Message: msg, TerminalGuidance: terminalGuidance}, nil
}

func (h ToolHandler) withRunLock(owner string, fn func() error) error {
	paths := h.paths()
	lock, err := runlock.TryAcquire(paths.DougDir, owner)
	if err != nil {
		if errors.Is(err, runlock.ErrHeld) {
			return lockHeldError(paths.DougDir, err)
		}
		return err
	}
	defer func() { _ = lock.Close() }()
	return fn()
}

func lockHeldError(dougDir string, err error) error {
	message := fmt.Sprintf("another Doug lifecycle driver is already active (%s)", runlock.Path(dougDir))
	if details := runlock.HeldDetails(dougDir); details != "" {
		message += "; lock holder " + details
	}
	return fmt.Errorf("%s: %w", message, err)
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

func (h ToolHandler) diagnosticsResponse(diagnostics lifecycle.Diagnostics) DiagnosticsResponse {
	resp := DiagnosticsResponse{StatusResponse: h.statusResponse(diagnostics.Status)}
	resp.AllowedNextActions = diagnostics.AllowedNextActions
	resp.Health = statusHealth(diagnostics.Findings)
	for _, finding := range diagnostics.Findings {
		resp.Findings = append(resp.Findings, diagnosticFindingResponse(finding))
	}
	return resp
}

func statusHealth(findings []lifecycle.DiagnosticFinding) StatusHealth {
	health := StatusHealth{Healthy: len(findings) == 0}
	for _, finding := range findings {
		health.Findings = append(health.Findings, diagnosticFindingResponse(finding))
	}
	return health
}

func diagnosticFindingResponse(finding lifecycle.DiagnosticFinding) DiagnosticFinding {
	message := finding.Message
	if finding.RequiresManualReview && !strings.Contains(message, manualReviewGuidance) {
		message = strings.TrimSpace(message) + "; " + manualReviewGuidance
	}
	return DiagnosticFinding{Code: finding.Code, Severity: finding.Severity, Message: message, Path: finding.Path, RequiresManualReview: finding.RequiresManualReview}
}

func (h ToolHandler) statusResponse(st lifecycle.Status) StatusResponse {
	resp := StatusResponse{CurrentEpic: st.EpicID, LifecyclePhase: string(st.Kind), BriefPath: st.ActiveTaskPath, AttemptCount: st.ActiveTask.Attempts, Completed: st.AllTasksDone, Health: StatusHealth{Healthy: true}}
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
	return []string{ToolGetStatus, ToolDiagnoseLifecycle, ToolReconcileLifecycle, ToolGetNextTask, ToolReportTaskComplete, ToolReportTaskBlocked}
}

// ToolDefinitions returns self-describing MCP tool metadata in ToolNames order.
func ToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        ToolGetStatus,
			Description: "Read Doug's current lifecycle status, including active/next assignment pointers and allowed next actions. This tool is read-only and does not acquire the run lock.",
			InputSchema: noArgInputSchema(),
		},
		{
			Name:        ToolDiagnoseLifecycle,
			Description: "Inspect lifecycle drift and recovery guidance without mutating Doug state. Use this when status is unclear, a brief is missing or stale, or manual review may be required.",
			InputSchema: noArgInputSchema(),
		},
		{
			Name:        ToolReconcileLifecycle,
			Description: "Repair only supported Doug-owned lifecycle drift. Call with mode=repair after diagnose_lifecycle identifies repairable drift; unsupported drift returns manual-review guidance without changing files.",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"mode"},
				"properties": map[string]any{
					"mode": map[string]any{
						"type":        "string",
						"enum":        []string{lifecycle.ReconcileModeRepair},
						"description": "Required explicit repair mode. Doug refuses omitted or unsupported modes before mutating lifecycle files.",
					},
				},
			},
		},
		{
			Name:        ToolGetNextTask,
			Description: "Claim the next Doug-authored assignment, write .doug/ACTIVE_TASK.md, and return the worker-ready brief plus dispatcher/worker context guidance. This mutates lifecycle state under the run lock.",
			InputSchema: noArgInputSchema(),
		},
		{
			Name:        ToolReportTaskComplete,
			Description: "Report that the current worker filled .doug/ACTIVE_TASK.md with a SUCCESS or EPIC_COMPLETE result. Doug parses the result, runs verified success handling, advances lifecycle state, and returns terminal guidance.",
			InputSchema: taskIDInputSchema(),
		},
		{
			Name:        ToolReportTaskBlocked,
			Description: "Report that the current worker filled .doug/ACTIVE_TASK.md with a FAILURE result. Doug records retry or blocked/manual-review lifecycle state under the run lock.",
			InputSchema: taskIDInputSchema(),
		},
	}
}

func noArgInputSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}}
}

func taskIDInputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"task_id": map[string]any{
				"type":        "string",
				"description": "Optional task ID to validate against Doug's active assignment. When omitted, Doug uses project-state.yaml active_task.id.",
			},
		},
	}
}

func IsTool(name string) bool {
	for _, n := range ToolNames() {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return false
}
