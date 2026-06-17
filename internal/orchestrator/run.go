package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/stats"
	"github.com/robertgumeny/doug/internal/types"
)

const taskDescriptionHeaderMaxRunes = 80

func formatAttemptHeader(taskID string, attempt, maxRetries int, description string) string {
	return fmt.Sprintf("[%s] attempt %d/%d — %s", taskID, attempt, maxRetries, truncateTaskDescription(description))
}

func truncateTaskDescription(description string) string {
	description = strings.TrimSpace(description)
	runes := []rune(description)
	if len(runes) <= taskDescriptionHeaderMaxRunes {
		return description
	}
	return string(runes[:taskDescriptionHeaderMaxRunes-3]) + "..."
}

func formatAgentEndSummary(resp agent.RunResponse) string {
	return fmt.Sprintf("agent finished in %s — first response +%s, %d tool calls, %d provider failures", formatMinutesSeconds(resp.Duration), formatSeconds(time.Duration(resp.FirstResponseMs)*time.Millisecond), resp.ToolCallCount, resp.ProviderFailures)
}

func formatMinutesSeconds(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int64(d.Round(time.Second) / time.Second)
	return fmt.Sprintf("%dm %ds", totalSeconds/60, totalSeconds%60)
}

func formatSeconds(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("%ds", int64(d.Round(time.Second)/time.Second))
}

func classifyAgentResultParseError(parseErr error) string {
	var invalidOutcome *agent.ErrInvalidOutcome
	switch {
	case errors.As(parseErr, &invalidOutcome):
		return fmt.Sprintf("agent result contract error: invalid outcome %q in `## Agent Result.outcome`", invalidOutcome.Value)
	case errors.Is(parseErr, agent.ErrMissingOutcome):
		return "agent result contract error: missing `## Agent Result.outcome`"
	case errors.Is(parseErr, agent.ErrNoFrontmatter):
		return "agent result contract error: missing YAML frontmatter in `## Agent Result` block"
	default:
		return "agent result parse error"
	}
}

func restoreAttemptsAfterAgentResultParseError(statePath string, projectState *types.ProjectState) error {
	if projectState.ActiveTask.Attempts > 0 {
		projectState.ActiveTask.Attempts--
	}
	return state.SaveProjectState(statePath, projectState)
}

func maxInfraRetries(cfg *config.OrchestratorConfig) int {
	if cfg != nil && cfg.MaxInfraRetries > 0 {
		return cfg.MaxInfraRetries
	}
	return config.DefaultMaxInfraRetries
}

func infraRetryBackoff(infraRetries int) time.Duration {
	if infraRetries < 1 {
		infraRetries = 1
	}
	backoff := time.Second
	for i := 1; i < infraRetries; i++ {
		backoff *= 2
		if backoff >= 30*time.Second {
			return 30 * time.Second
		}
	}
	return backoff
}

func sleepForInfraRetry(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (o *Orchestrator) waitForInfraRetry(ctx context.Context, d time.Duration) error {
	if o.infraRetrySleeper != nil {
		return o.infraRetrySleeper(ctx, d)
	}
	return sleepForInfraRetry(ctx, d)
}

func writeInfraRetryFailureReport(path, taskID string, attempts, infraRetries, cap int, transportErr error) error {
	message := fmt.Sprintf("# Transport Failure\n\nTask `%s` hit the infrastructure retry cap before Doug could read an agent workflow outcome.\n\n- Task attempts consumed: %d\n- Infrastructure retries: %d/%d\n", taskID, attempts, infraRetries, cap)
	if transportErr != nil {
		message += fmt.Sprintf("- Last transport error: `%v`\n", transportErr)
	}
	message += "\nDoug halted the run without decrementing the task attempt counter for these transport failures. Retry after the Pi/provider transport issue is resolved.\n"
	return os.WriteFile(path, []byte(message), 0o644)
}

func writeInfraFailureRecord(logsDir, epicID, taskID string, infraAttempt int, failedAt time.Time, class string, resp agent.RunResponse, transportErr error, outputLogPath string) (string, error) {
	recordDir := filepath.Join(logsDir, "failures", epicID)
	if err := os.MkdirAll(recordDir, 0o755); err != nil {
		return "", fmt.Errorf("create infra failure record directory: %w", err)
	}
	recordPath := filepath.Join(recordDir, fmt.Sprintf("infra-failure-%s-attempt-%d.md", taskID, infraAttempt))

	exitCode := ""
	if resp.ExitCode != nil {
		exitCode = fmt.Sprintf("%d", *resp.ExitCode)
	}
	errorText := ""
	if transportErr != nil {
		errorText = transportErr.Error()
	}

	message := fmt.Sprintf("---\ntask_id: %q\nattempt: %d\nfailed_at: %q\nclass: %q\nbackend_status: %q\nerror: %q\nexit_code: %q\noutput_log: %q\n---\n\n# Infrastructure Failure\n\nDoug recorded a transport failure before an agent workflow outcome was available.\n", taskID, infraAttempt, failedAt.UTC().Format(time.RFC3339), class, resp.Status, errorText, exitCode, outputLogPath)
	if err := state.AtomicWrite(recordPath, []byte(message)); err != nil {
		return "", err
	}
	return recordPath, nil
}

// Run executes the full orchestration lifecycle: pre-loop setup followed by
// the main iteration loop. The context is checked at the start of each
// iteration; cancellation exits the loop cleanly.
func (o *Orchestrator) Run(ctx context.Context) error {
	// Step 1: Verify all required binaries are available before doing any work.
	if err := CheckDependencies(o.cfg); err != nil {
		return fmt.Errorf("%w — install the missing tools and add them to PATH, then retry", err)
	}

	// Step 2: Load state and task files.
	projectState, err := state.LoadProjectState(o.paths.StatePath)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return fmt.Errorf("project state not found at %s — run `doug init` to initialise the project", o.paths.StatePath)
		}
		return fmt.Errorf("load project state: %w", err)
	}
	tasks, err := state.LoadTasks(o.paths.TasksPath)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return fmt.Errorf("tasks file not found at %s — create .doug/tasks.yaml with your task definitions", o.paths.TasksPath)
		}
		return fmt.Errorf("load tasks: %w", err)
	}

	// Step 3: Detect PAUSED state — resume via build verification in the loop.
	// The loop will skip agent invocation on the first iteration and run
	// build/test verification directly against the current working tree.
	resumeFromPause := false
	if projectState.Status == types.ProjectStatusPaused {
		o.logger.Info(fmt.Sprintf(
			"project is PAUSED for task %s — resuming via build verification",
			projectState.ActiveTask.ID,
		))
		projectState.Status = ""
		resumeFromPause = true
	}

	// Step 4: Detect epic rollover when tasks.yaml switched to a new epic.
	rolled, err := PrepareForEpicRollover(projectState, tasks)
	if err != nil {
		return fmt.Errorf("epic rollover blocked: %w", err)
	}
	if rolled {
		o.logger.Info(fmt.Sprintf("detected new epic %s in tasks.yaml — resetting runtime state for rollover", tasks.Epic.ID))
	}

	// Step 5: Bootstrap state on first run (no-op if CurrentEpic.ID is already set).
	BootstrapFromTasks(projectState, tasks)

	// Step 6: Early exit if all tasks are already complete.
	if IsEpicAlreadyComplete(projectState, tasks) {
		o.logger.Success("all tasks already DONE — nothing to do")
		return nil
	}

	// Step 7: Pre-flight build/test check (skipped on resume — verification runs
	// in the loop; also skipped when project is not yet initialized).
	if !resumeFromPause {
		if err := EnsureProjectReady(o.buildSystem, o.cfg.BuildSystem, o.logger); err != nil {
			return fmt.Errorf("pre-flight check failed: %w", err)
		}
	}

	if normalized, err := NormalizeLegacyManualReviewState(projectState, tasks); err != nil {
		return fmt.Errorf("legacy state normalization failed: %w", err)
	} else if normalized {
		o.logger.Warning("normalized legacy manual_review state to blocked-task model")
		if err := state.SaveTasks(o.paths.TasksPath, tasks); err != nil {
			return fmt.Errorf("save tasks after legacy state normalization: %w", err)
		}
		if err := state.SaveProjectState(o.paths.StatePath, projectState); err != nil {
			return fmt.Errorf("save project state after legacy state normalization: %w", err)
		}
	}

	// Step 8: Structural validation — fail fast on corrupt or missing required fields.
	if err := ValidateYAMLStructure(projectState, tasks); err != nil {
		return fmt.Errorf("YAML structure invalid: %w\nFix: edit the file indicated above and set the missing or invalid field", err)
	}
	if err := ValidateTaskTypes(tasks); err != nil {
		return fmt.Errorf("task type validation failed: %w", err)
	}

	// Step 9: Ensure the working tree is on the correct epic feature branch.
	if err := git.EnsureEpicBranch(projectState.CurrentEpic.BranchName, o.paths.ProjectRoot); err != nil {
		return fmt.Errorf("ensure epic branch: %w", err)
	}

	// Step 10: Align active and next task pointers with the current task list state.
	InitializeTaskPointers(projectState, tasks)

	// Step 11: Validate state/task consistency.
	// Skip ValidateStateSync for active tasks not in tasks.yaml (e.g.,
	// handler-injected BUG-xxx bugfix tasks or scaffold tasks). Only tasks
	// whose IDs are in the backlog can be meaningfully validated for sync.
	activeTaskInBacklog := false
	for _, t := range tasks.Epic.Tasks {
		if t.ID == projectState.ActiveTask.ID {
			activeTaskInBacklog = true
			break
		}
	}
	if activeTaskInBacklog {
		vResult, vErr := ValidateStateSync(projectState, tasks)
		if vErr != nil {
			return fmt.Errorf("state sync validation failed: %w", vErr)
		}
		if vResult.Kind == ValidationAutoCorrected {
			o.logger.Warning(vResult.Description)
		}
		if err := ValidateActiveTaskIsRunnable(projectState, tasks); err != nil {
			return err
		}
	}

	// Persist bootstrapped / pointer-initialised state before the loop begins.
	if err := state.SaveProjectState(o.paths.StatePath, projectState); err != nil {
		return fmt.Errorf("save initial project state: %w", err)
	}

	// If the previous run completed the final user task and saved completed_at
	// but exited before finalization, resume finalization now.
	if types.AreAllUserTasksComplete(tasks) && projectState.CurrentEpic.CompletedAt != nil && *projectState.CurrentEpic.CompletedAt != "" {
		finalizeCtx := &LoopContext{
			TaskID:        projectState.ActiveTask.ID,
			TaskType:      projectState.ActiveTask.Type,
			Attempts:      projectState.ActiveTask.Attempts,
			CurrentEpic:   projectState.CurrentEpic,
			Config:        o.cfg,
			BuildSystem:   o.buildSystem,
			ProjectRoot:   o.paths.ProjectRoot,
			TaskStartTime: time.Now(),
			State:         projectState,
			Tasks:         tasks,
			StatePath:     o.paths.StatePath,
			TasksPath:     o.paths.TasksPath,
			DougDir:       o.paths.DougDir,
			LogsDir:       o.paths.LogsDir,
			ChangelogPath: o.paths.ChangelogPath,
			Logger:        o.logger,
		}
		if err := handlers.HandleEpicComplete(finalizeCtx); err != nil {
			return fmt.Errorf("epic finalization failed: %w", err)
		}
		if err := o.runPostEpicKB(ctx, projectState); err != nil {
			o.logger.Warning(fmt.Sprintf("post-epic KB synthesis failed: %v", err))
		}
		return nil
	}

	// -------------------------------------------------------------------------
	// Main orchestration loop
	// -------------------------------------------------------------------------
	for iteration := 0; iteration < o.cfg.MaxIterations; iteration++ {
		// Respect context cancellation at the start of each iteration.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Resume path: skip agent invocation on the first iteration after a PAUSE.
		// Build verification runs directly; no attempt counter increment (BUILD_FAILURE
		// must not consume a retry).
		if resumeFromPause {
			resumeFromPause = false
			o.logger.Section(fmt.Sprintf("RESUME — task %s", projectState.ActiveTask.ID))
			resumeCtx := &LoopContext{
				TaskID:        projectState.ActiveTask.ID,
				TaskType:      projectState.ActiveTask.Type,
				Attempts:      projectState.ActiveTask.Attempts,
				CurrentEpic:   projectState.CurrentEpic,
				Config:        o.cfg,
				BuildSystem:   o.buildSystem,
				ProjectRoot:   o.paths.ProjectRoot,
				TaskStartTime: time.Now(),
				State:         projectState,
				Tasks:         tasks,
				StatePath:     o.paths.StatePath,
				TasksPath:     o.paths.TasksPath,
				DougDir:       o.paths.DougDir,
				LogsDir:       o.paths.LogsDir,
				ChangelogPath: o.paths.ChangelogPath,
				Logger:        o.logger,
			}
			sr, err := handlers.HandleResume(resumeCtx)
			if err != nil {
				return fmt.Errorf("HandleResume: %w", err)
			}
			switch sr.Kind {
			case handlers.EpicComplete:
				if err := handlers.HandleEpicComplete(resumeCtx); err != nil {
					return fmt.Errorf("epic finalization failed: %w", err)
				}
				return nil
			case handlers.Continue:
				continue
			case handlers.BuildFailure:
				return nil
			case handlers.Retry:
				continue
			}
		}

		// IncrementAttempts at the START of each iteration, matching Bash orchestrator behavior.
		IncrementAttempts(projectState)

		// Snapshot per-iteration identity after the increment.
		taskID := projectState.ActiveTask.ID
		taskType := projectState.ActiveTask.Type
		attempts := projectState.ActiveTask.Attempts

		// Look up description and acceptance criteria for user-defined tasks.
		// For synthetic tasks (bugfix, documentation) the task won't be found — empty values are fine.
		var taskDesc string
		var taskCriteria []string
		for _, t := range tasks.Epic.Tasks {
			if t.ID == taskID {
				taskDesc = t.Description
				taskCriteria = t.AcceptanceCriteria
				break
			}
		}

		o.logger.Section(formatAttemptHeader(taskID, attempts, o.cfg.MaxRetries, taskDesc))

		// Safety net: catch any stuck loop regardless of outcome type.
		// HandleFailure blocks at attempts == MaxRetries; this fires at MaxRetries+1
		// as a backstop for tasks that always report SUCCESS but never advance.
		if attempts > o.cfg.MaxRetries {
			return fmt.Errorf("task %s has been attempted %d times without completing — max retries (%d) exceeded; task must be reviewed or unblocked manually",
				taskID, attempts, o.cfg.MaxRetries)
		}

		// Persist the incremented attempt counter before invoking the agent so that
		// a crash mid-run does not reset the counter on restart.
		if err := state.SaveProjectState(o.paths.StatePath, projectState); err != nil {
			return fmt.Errorf("save state before agent invocation: %w", err)
		}

		// Resolve the skill name and source-owned interaction mode for this phase.
		prep, prepErr := agent.PrepareExecution(string(agent.RunPhaseRuntime), string(taskType), taskID)
		if prepErr != nil {
			return fmt.Errorf("prepare execution for task %s: %w", taskID, prepErr)
		}

		var extraSections []agent.ActiveTaskSection

		// Write ACTIVE_TASK.md with task metadata and briefing header.
		if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
			TaskID:             taskID,
			TaskType:           taskType,
			DougDir:            o.paths.DougDir,
			Description:        taskDesc,
			AcceptanceCriteria: taskCriteria,
			Attempts:           attempts,
			MaxRetries:         o.cfg.MaxRetries,
			BuildSystem:        o.cfg.BuildSystem,
			TestFailureOutput:  projectState.ActiveTask.TestFailureOutput,
			ContextSections:    extraSections,
		}, o.logger); err != nil {
			return fmt.Errorf("write active task: %w", err)
		}

		// Guard: bugfix tasks require ACTIVE_BUG.md to exist — without it the
		// agent has no bug report and will run blind, causing stuck loops.
		if taskType == types.TaskTypeBugfix {
			bugFile := filepath.Join(o.paths.DougDir, "ACTIVE_BUG.md")
			if _, err := os.Stat(bugFile); errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("task %s is type bugfix but .doug/ACTIVE_BUG.md is missing — cannot dispatch bugfix agent without a bug report", taskID)
			}
		}

		runTaskContext := agent.TaskContext{
			ID:         taskID,
			Type:       string(taskType),
			Attempt:    attempts,
			MaxRetries: o.cfg.MaxRetries,
			EpicID:     projectState.CurrentEpic.ID,
			EpicName:   projectState.CurrentEpic.Name,
		}

		// Build the loop context for handler dispatch.
		loopCtx := &LoopContext{
			TaskID:        taskID,
			TaskType:      taskType,
			Attempts:      attempts,
			CurrentEpic:   projectState.CurrentEpic,
			Config:        o.cfg,
			BuildSystem:   o.buildSystem,
			ProjectRoot:   o.paths.ProjectRoot,
			TaskStartTime: time.Now(),
			State:         projectState,
			Tasks:         tasks,
			StatePath:     o.paths.StatePath,
			TasksPath:     o.paths.TasksPath,
			DougDir:       o.paths.DougDir,
			LogsDir:       o.paths.LogsDir,
			ChangelogPath: o.paths.ChangelogPath,
			Logger:        o.logger,
		}

		if err := agent.WriteAttemptStart(o.paths.ProjectRoot, agent.RunPhaseRuntime, runTaskContext, time.Now()); err != nil {
			return fmt.Errorf("write attempt-start marker: %w", err)
		}

		// Open a raw output log for Pi/agent output. Output is preserved on disk
		// alongside the session file for post-run inspection.
		outputLogDir := filepath.Join(o.paths.LogsDir, "output", projectState.CurrentEpic.ID)
		if err := os.MkdirAll(outputLogDir, 0o755); err != nil {
			return fmt.Errorf("create output log directory: %w", err)
		}
		outputLogPath := filepath.Join(outputLogDir, fmt.Sprintf("output-%s_attempt-%d.log", taskID, attempts))
		outputLog, err := os.Create(outputLogPath)
		if err != nil {
			return fmt.Errorf("create agent output log: %w", err)
		}

		// Invoke the agent; a non-zero exit is non-fatal — the session file is
		// the authoritative result regardless of the agent process exit code.
		o.logger.Info(fmt.Sprintf("invoking agent for task %s (attempt %d)", taskID, attempts))
		heartbeatEvery := time.Duration(o.cfg.AgentHeartbeatSeconds) * time.Second
		firstResponseThreshold := time.Duration(o.cfg.FirstResponseThresholdSeconds) * time.Second
		var firstResponseSeen atomic.Bool
		var noResponseWarned atomic.Bool
		contract := agent.RuntimeContract(o.paths.ProjectRoot, o.paths.DougDir)
		activeTaskPath := contract.Brief.Path
		agentResp, agentErr := o.execBackend().Run(ctx, agent.RunRequest{
			Phase:            agent.RunPhaseRuntime,
			Task:             runTaskContext,
			Brief:            contract.Brief,
			ContextLoadOrder: contract.ContextLoadOrder,
			Artifacts:        contract.Artifacts,
			Routing: agent.RoutingInputs{
				Workflow:        "run",
				SkillName:       prep.SkillName,
				InteractionMode: prep.InteractionMode,
			},
			Restrictions:      contract.Restrictions,
			InitialPrompt:     prep.InitialPrompt,
			ProjectRoot:       o.paths.ProjectRoot,
			HeartbeatInterval: heartbeatEvery,
			HeartbeatFn: func(elapsed time.Duration, activity string) {
				elapsed = elapsed.Round(time.Second)
				if firstResponseThreshold > 0 && elapsed >= firstResponseThreshold && !firstResponseSeen.Load() && noResponseWarned.CompareAndSwap(false, true) {
					o.logger.Warning(fmt.Sprintf("⚠ no provider response yet (+%s)", elapsed))
				}
				o.logger.Info(fmt.Sprintf("[%s] +%s — %s", taskID, elapsed, activity))
			},
			FirstResponseFn: func(elapsed time.Duration) {
				firstResponseSeen.Store(true)
				o.logger.Info(fmt.Sprintf("► first response (+%s)", elapsed.Round(time.Second)))
			},
			Output: outputLog,
		})
		if closeErr := outputLog.Close(); closeErr != nil {
			o.logger.Warning(fmt.Sprintf("close agent output log: %v", closeErr))
		}
		if metaErr := agent.WriteRunMetadata(outputLogPath, agentResp, agentErr); metaErr != nil {
			o.logger.Warning(fmt.Sprintf("write agent run metadata: %v", metaErr))
		}
		statsRecord := stats.FromRunResponse(taskID, attempts, time.Now(), agentResp)
		if statsPath, statsErr := stats.WriteRunStats(o.paths.LogsDir, projectState.CurrentEpic.ID, statsRecord); statsErr != nil {
			o.logger.Warning(fmt.Sprintf("write agent run stats: %v", statsErr))
		} else {
			o.logger.Info(fmt.Sprintf("wrote run stats: %s", statsPath))
		}
		if agentResp.Status == agent.RunStatusTransportFailure {
			if projectState.ActiveTask.Attempts > 0 {
				projectState.ActiveTask.Attempts--
			}
			projectState.ActiveTask.InfraRetries++
			infraRetries := projectState.ActiveTask.InfraRetries
			infraCap := maxInfraRetries(o.cfg)
			failureClass := "transport_failure"
			if infraRetries >= infraCap {
				failureClass = "transport_failure_retry_cap"
			}
			failureRecordPath, err := writeInfraFailureRecord(o.paths.LogsDir, projectState.CurrentEpic.ID, taskID, infraRetries, time.Now(), failureClass, agentResp, agentErr, outputLogPath)
			if err != nil {
				return fmt.Errorf("write infra failure record: %w", err)
			}
			o.logger.Warning(fmt.Sprintf("wrote infra failure record: %s", failureRecordPath))
			if err := state.SaveProjectState(o.paths.StatePath, projectState); err != nil {
				return fmt.Errorf("save state after transport failure: %w", err)
			}
			if infraRetries >= infraCap {
				failurePath := filepath.Join(o.paths.DougDir, "ACTIVE_FAILURE.md")
				if err := writeInfraRetryFailureReport(failurePath, taskID, projectState.ActiveTask.Attempts, infraRetries, infraCap, agentErr); err != nil {
					return fmt.Errorf("write transport failure report: %w", err)
				}
				return fmt.Errorf("agent transport failed %d/%d times for task %s; wrote durable failure to %s", infraRetries, infraCap, taskID, failurePath)
			}
			backoff := infraRetryBackoff(infraRetries)
			o.logger.Warning(fmt.Sprintf("agent transport failed for task %s (infra retry %d/%d); retrying in %s without consuming a task attempt", taskID, infraRetries, infraCap, backoff))
			if err := o.waitForInfraRetry(ctx, backoff); err != nil {
				return err
			}
			continue
		}
		if projectState.ActiveTask.InfraRetries != 0 {
			projectState.ActiveTask.InfraRetries = 0
			if err := state.SaveProjectState(o.paths.StatePath, projectState); err != nil {
				return fmt.Errorf("clear infra retry counter after agent transport recovery: %w", err)
			}
		}
		if agentErr != nil {
			o.logger.Warning(fmt.Sprintf("agent exited with error: %v — reading session result anyway", agentErr))
		}

		// Capture agent result for explicit dispatch to outcome handlers.
		agentDurationSeconds := int(agentResp.Duration.Seconds())
		loopCtx.ProviderWaitMs = agentResp.FirstResponseMs
		loopCtx.ProviderFailures = agentResp.ProviderFailureDetails

		// Parse the result block written by the agent into ACTIVE_TASK.md.
		agentResult, parseErr := agent.ParseSessionResult(activeTaskPath)
		if parseErr != nil {
			parseSummary := classifyAgentResultParseError(parseErr)
			o.logger.Error(fmt.Sprintf("%s: %v", parseSummary, parseErr))
			if err := agent.ArchiveActiveTask(o.paths.DougDir, o.paths.LogsDir, projectState.CurrentEpic.ID, taskID, attempts); err != nil {
				o.logger.Warning(fmt.Sprintf("session archive failed after parse error: %v", err))
			}
			if err := restoreAttemptsAfterAgentResultParseError(o.paths.StatePath, projectState); err != nil {
				o.logger.Warning(fmt.Sprintf("could not restore attempt counter after parse error: %v", err))
			}
			return fmt.Errorf("%s in %s: %w", parseSummary, activeTaskPath, parseErr)
		}

		o.logger.Info(formatAgentEndSummary(agentResp))
		if agentResult.ChangelogEntry != "" {
			o.logger.Info(fmt.Sprintf("outcome: %s — %s", agentResult.Outcome, agentResult.ChangelogEntry))
		} else {
			o.logger.Info(fmt.Sprintf("outcome: %s", agentResult.Outcome))
		}

		// Dispatch to the appropriate outcome handler.
		switch agentResult.Outcome {

		case types.OutcomeSuccess:
			sr, err := handlers.HandleSuccess(loopCtx, agentResult, agentDurationSeconds)
			if err != nil {
				return fmt.Errorf("HandleSuccess: %w", err)
			}
			switch sr.Kind {
			case handlers.EpicComplete:
				if err := handlers.HandleEpicComplete(loopCtx); err != nil {
					return fmt.Errorf("epic finalization failed: %w", err)
				}
				if err := o.runPostEpicKB(ctx, projectState); err != nil {
					o.logger.Warning(fmt.Sprintf("post-epic KB synthesis failed: %v", err))
				}
				return nil

			case handlers.Continue:
				// Normal forward progress — state already updated in memory by handler.

			case handlers.BuildFailure:
				// Build/test verification failed after agent SUCCESS.
				// Project is PAUSED; working tree preserved. Exit cleanly.
				return nil

			case handlers.Retry:
				// Non-fatal issue (git commit failure).
				// The loop retries on the next iteration.
			}

		case types.OutcomeFailure:
			if err := handlers.HandleFailure(loopCtx, agentDurationSeconds); err != nil {
				return err
			}

		case types.OutcomeBug:
			if err := handlers.HandleBug(loopCtx, agentDurationSeconds); err != nil {
				return err
			}

		case types.OutcomeEpicComplete:
			if err := handlers.HandleEpicComplete(loopCtx); err != nil {
				return fmt.Errorf("epic finalization failed: %w", err)
			}
			if err := o.runPostEpicKB(ctx, projectState); err != nil {
				o.logger.Warning(fmt.Sprintf("post-epic KB synthesis failed: %v", err))
			}
			return nil
		}
	}

	// Max iterations reached — this is a clean exit, not an error.
	o.logger.Warning(fmt.Sprintf("max iterations (%d) reached — exiting", o.cfg.MaxIterations))
	return nil
}
