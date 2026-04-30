package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

func parseAgentResult(activeTaskPath string) (*types.SessionResult, error) {
	return agent.ParseSessionResult(activeTaskPath)
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
	// Synthetic tasks (bugfix, documentation) are never in tasks.yaml by design;
	// skip ValidateStateSync for them — the function would always return Fatal.
	if !projectState.ActiveTask.Type.IsSynthetic() {
		vResult, vErr := ValidateStateSync(projectState, tasks)
		if vErr != nil {
			return fmt.Errorf("state sync validation failed: %w", vErr)
		}
		if vResult.Kind == ValidationAutoCorrected {
			o.logger.Warning(vResult.Description)
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

		o.logger.Section(fmt.Sprintf("[%s] attempt %d/%d (%s)", taskID, attempts, o.cfg.MaxRetries, taskType))

		// Safety net: catch any stuck loop regardless of outcome type.
		// HandleFailure blocks at attempts == MaxRetries; this fires at MaxRetries+1
		// as a backstop for tasks that always report SUCCESS but never advance.
		if attempts > o.cfg.MaxRetries {
			return fmt.Errorf("task %s has been attempted %d times without completing — max retries (%d) exceeded; manual review required",
				taskID, attempts, o.cfg.MaxRetries)
		}

		// Persist the incremented attempt counter before invoking the agent so that
		// a crash mid-run does not reset the counter on restart.
		if err := state.SaveProjectState(o.paths.StatePath, projectState); err != nil {
			return fmt.Errorf("save state before agent invocation: %w", err)
		}

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

		// Resolve {{skill_name}} and {{task_id}} in agent command before invocation.
		skillName, _ := agent.GetSkillForTaskType(string(taskType), o.paths.SkillsConfigPath)
		resolvedCmd := strings.ReplaceAll(o.cfg.RunAgentCommand, "{{skill_name}}", skillName)
		resolvedCmd = strings.ReplaceAll(resolvedCmd, "{{task_id}}", taskID)

		// Open a raw output log for the agent's stdout+stderr. This prevents
		// agents that unconditionally stream to the terminal (e.g. codex exec)
		// from blasting output during an automated run. Output is preserved on
		// disk alongside the session file for post-run inspection.
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
		contract := agent.RuntimeContract(o.paths.ProjectRoot, o.paths.DougDir)
		activeTaskPath := contract.Brief.Path
		agentResp, agentErr := o.execBackend().Run(ctx, agent.RunRequest{
			Phase: agent.RunPhaseRuntime,
			Task: agent.TaskContext{
				ID:         taskID,
				Type:       string(taskType),
				Attempt:    attempts,
				MaxRetries: o.cfg.MaxRetries,
				EpicID:     projectState.CurrentEpic.ID,
				EpicName:   projectState.CurrentEpic.Name,
			},
			Brief:            contract.Brief,
			ContextLoadOrder: contract.ContextLoadOrder,
			Artifacts:        contract.Artifacts,
			Routing: agent.RoutingInputs{
				Workflow:  "run",
				SkillName: skillName,
			},
			Policy:            agent.PolicyInputs{},
			Restrictions:      contract.Restrictions,
			Command:           resolvedCmd,
			ProjectRoot:       o.paths.ProjectRoot,
			HeartbeatInterval: heartbeatEvery,
			HeartbeatFn: func(elapsed time.Duration) {
				o.logger.Info(fmt.Sprintf("[%s] +%s", taskID, elapsed.Round(time.Second)))
			},
			Output: outputLog,
		})
		if closeErr := outputLog.Close(); closeErr != nil {
			o.logger.Warning(fmt.Sprintf("close agent output log: %v", closeErr))
		}
		if agentErr != nil {
			o.logger.Warning(fmt.Sprintf("agent exited with error: %v — reading session result anyway", agentErr))
		}

		// Capture agent result for explicit dispatch to outcome handlers.
		agentDurationSeconds := int(agentResp.Duration.Seconds())

		// Parse the result block written by the agent into ACTIVE_TASK.md.
		agentResult, parseErr := parseAgentResult(activeTaskPath)
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
