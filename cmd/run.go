package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/build"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// runFlags holds CLI flag values that override doug.yaml config settings.
// Only flags explicitly changed by the user are applied (checked via cmd.Flags().Changed).
var runFlags struct {
	agentCommand  string
	buildSystem   string
	maxRetries    int
	maxIterations int
	kbEnabled     bool
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the orchestration loop",
	Long:  "Run the orchestration loop, executing tasks defined in tasks.yaml.",
	RunE:  runOrchestrate,
}

func init() {
	runCmd.Flags().StringVar(&runFlags.agentCommand, "agent", "", "override agent_command from doug.yaml")
	runCmd.Flags().StringVar(&runFlags.buildSystem, "build-system", "", "override build_system from doug.yaml (go|npm)")
	runCmd.Flags().IntVar(&runFlags.maxRetries, "max-retries", 0, "override max_retries from doug.yaml")
	runCmd.Flags().IntVar(&runFlags.maxIterations, "max-iterations", 0, "override max_iterations from doug.yaml")
	runCmd.Flags().BoolVar(&runFlags.kbEnabled, "kb-enabled", false, "override kb_enabled from doug.yaml")
}

// runOrchestrate implements the full orchestration loop for the "run" subcommand.
//
// Pre-loop sequence:
//  1. Load config from doug.yaml; apply any CLI flag overrides.
//  2. CheckDependencies — verify agent binary, git, and toolchain are on PATH.
//  3. Load project-state.yaml and tasks.yaml from the working directory.
//  4. BootstrapFromTasks — no-op if already bootstrapped; initializes state on first run.
//  5. IsEpicAlreadyComplete — exit 0 immediately if all work is done.
//  6. EnsureProjectReady — pre-flight build/test (skipped when project not initialized).
//  7. ValidateYAMLStructure — fail fast on structurally corrupt state.
//  8. EnsureEpicBranch — check out the feature branch (create if needed).
//  9. InitializeTaskPointers — align active/next task with task list status.
// 10. ValidateStateSync — catch state/task drift (skipped for synthetic tasks).
// 11. Persist state before entering the loop.
//
// Main loop (up to cfg.MaxIterations):
//   - IncrementAttempts at the START of each iteration (before agent invocation).
//   - CreateSessionFile → WriteActiveTask → RunAgent → ParseSessionResult.
//   - Dispatch to HandleSuccess / HandleFailure / HandleBug / HandleEpicComplete.
//   - Fatal errors (nested bug, blocked task, epic commit failure) return non-nil
//     so cobra exits with code 1.
//   - Max iterations reached → exit code 0.
func runOrchestrate(cmd *cobra.Command, args []string) error {
	// Step 1: Determine project root from the current working directory.
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Step 2: Load config; a missing doug.yaml returns sane defaults without error.
	cfg, err := config.LoadConfig(filepath.Join(projectRoot, "doug.yaml"))
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Apply CLI flag overrides — only when the user explicitly set the flag.
	if cmd.Flags().Changed("agent") {
		cfg.AgentCommand = runFlags.agentCommand
	}
	if cmd.Flags().Changed("build-system") {
		cfg.BuildSystem = runFlags.buildSystem
	}
	if cmd.Flags().Changed("max-retries") {
		cfg.MaxRetries = runFlags.maxRetries
	}
	if cmd.Flags().Changed("max-iterations") {
		cfg.MaxIterations = runFlags.maxIterations
	}
	if cmd.Flags().Changed("kb-enabled") {
		cfg.KBEnabled = runFlags.kbEnabled
	}

	// Step 3: Verify all required binaries are available before doing any work.
	if err := orchestrator.CheckDependencies(cfg); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	// Step 4: Load state and task files.
	statePath := filepath.Join(projectRoot, "project-state.yaml")
	tasksPath := filepath.Join(projectRoot, "tasks.yaml")

	projectState, err := state.LoadProjectState(statePath)
	if err != nil {
		return fmt.Errorf("load project state: %w", err)
	}
	tasks, err := state.LoadTasks(tasksPath)
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	// Step 5: Bootstrap state on first run (no-op if CurrentEpic.ID is already set).
	orchestrator.BootstrapFromTasks(projectState, tasks)

	// Step 6: Early exit if all tasks are already complete.
	if orchestrator.IsEpicAlreadyComplete(projectState, tasks) {
		log.Success("all tasks already DONE — nothing to do")
		return nil // exit code 0
	}

	// Step 7: Construct the build system implementation.
	buildSys, err := build.NewBuildSystem(cfg.BuildSystem, projectRoot)
	if err != nil {
		return fmt.Errorf("build system: %w", err)
	}

	// Step 8: Pre-flight build/test check (skipped when project is not yet initialized).
	if err := orchestrator.EnsureProjectReady(buildSys, cfg); err != nil {
		return fmt.Errorf("pre-flight check failed: %w", err)
	}

	// Step 9: Structural validation — fail fast on corrupt or missing required fields.
	if err := orchestrator.ValidateYAMLStructure(projectState, tasks); err != nil {
		return fmt.Errorf("YAML structure invalid: %w", err)
	}

	// Step 10: Ensure the working tree is on the correct epic feature branch.
	if err := git.EnsureEpicBranch(projectState.CurrentEpic.BranchName, projectRoot); err != nil {
		return fmt.Errorf("ensure epic branch: %w", err)
	}

	// Step 11: Align active and next task pointers with the current task list state.
	orchestrator.InitializeTaskPointers(projectState, tasks)

	// Step 12: Validate state/task consistency.
	// Synthetic tasks (bugfix, documentation) are never in tasks.yaml by design;
	// skip ValidateStateSync for them — the function would always return Fatal.
	if !projectState.ActiveTask.Type.IsSynthetic() {
		vResult, vErr := orchestrator.ValidateStateSync(projectState, tasks)
		if vErr != nil {
			return fmt.Errorf("state sync validation failed: %w", vErr)
		}
		if vResult.Kind == orchestrator.ValidationAutoCorrected {
			log.Warning(vResult.Description)
		}
	}

	// Persist bootstrapped / pointer-initialised state before the loop begins.
	if err := state.SaveProjectState(statePath, projectState); err != nil {
		return fmt.Errorf("save initial project state: %w", err)
	}

	// Fixed paths used by agent helpers and handlers throughout the loop.
	logsDir := filepath.Join(projectRoot, "logs")
	changelogPath := filepath.Join(projectRoot, "CHANGELOG.md")
	skillsConfigPath := filepath.Join(projectRoot, ".claude", "skills-config.yaml")

	// -------------------------------------------------------------------------
	// Main orchestration loop
	// -------------------------------------------------------------------------
	for iteration := 0; iteration < cfg.MaxIterations; iteration++ {
		log.Section(fmt.Sprintf("ITERATION %d — task %s", iteration+1, projectState.ActiveTask.ID))

		// IncrementAttempts at the START of each iteration, matching Bash orchestrator behavior.
		orchestrator.IncrementAttempts(projectState)

		// Snapshot per-iteration identity after the increment.
		taskID := projectState.ActiveTask.ID
		taskType := projectState.ActiveTask.Type
		attempts := projectState.ActiveTask.Attempts

		// Persist the incremented attempt counter before invoking the agent so that
		// a crash mid-run does not reset the counter on restart.
		if err := state.SaveProjectState(statePath, projectState); err != nil {
			return fmt.Errorf("save state before agent invocation: %w", err)
		}

		// Pre-create the session result file so the agent has a path to write to.
		sessionPath, err := agent.CreateSessionFile(
			logsDir,
			projectState.CurrentEpic.ID,
			taskID,
			attempts,
		)
		if err != nil {
			return fmt.Errorf("create session file: %w", err)
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

		// Write ACTIVE_TASK.md with task metadata and skill instructions.
		if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
			TaskID:             taskID,
			TaskType:           taskType,
			SessionFilePath:    sessionPath,
			LogsDir:            logsDir,
			SkillsConfigPath:   skillsConfigPath,
			Description:        taskDesc,
			AcceptanceCriteria: taskCriteria,
			Attempts:           attempts,
			MaxRetries:         cfg.MaxRetries,
		}); err != nil {
			return fmt.Errorf("write active task: %w", err)
		}

		// Build the loop context for handler dispatch.
		ctx := &orchestrator.LoopContext{
			TaskID:        taskID,
			TaskType:      taskType,
			Attempts:      attempts,
			CurrentEpic:   projectState.CurrentEpic,
			Config:        cfg,
			BuildSystem:   buildSys,
			ProjectRoot:   projectRoot,
			TaskStartTime: time.Now(),
			State:         projectState,
			Tasks:         tasks,
			StatePath:     statePath,
			TasksPath:     tasksPath,
			LogsDir:       logsDir,
			ChangelogPath: changelogPath,
		}

		// Invoke the agent; a non-zero exit is non-fatal — the session file is
		// the authoritative result regardless of the agent process exit code.
		log.Info(fmt.Sprintf("invoking agent for task %s (attempt %d)", taskID, attempts))
		if _, agentErr := agent.RunAgent(cfg.AgentCommand, projectRoot); agentErr != nil {
			log.Warning(fmt.Sprintf("agent exited with error: %v — reading session result anyway", agentErr))
		}

		// Parse the session result written by the agent.
		result, parseErr := agent.ParseSessionResult(sessionPath)
		if parseErr != nil {
			log.Error(fmt.Sprintf("failed to parse session result from %s: %v — treating as FAILURE", sessionPath, parseErr))
			result = &types.SessionResult{Outcome: types.OutcomeFailure}
		}
		ctx.SessionResult = result

		log.Info(fmt.Sprintf("session outcome: %s", result.Outcome))

		// Dispatch to the appropriate outcome handler.
		switch result.Outcome {

		case types.OutcomeSuccess:
			sr, err := handlers.HandleSuccess(ctx)
			if err != nil {
				// Fatal: rollback failed or state could not be persisted.
				return fmt.Errorf("HandleSuccess: %w", err)
			}
			switch sr.Kind {
			case handlers.EpicComplete:
				// KB synthesis documentation task completed; finalize the epic.
				if err := handlers.HandleEpicComplete(ctx); err != nil {
					// Tier 3: epic commit failure — surface as exit code 1 (CI-6).
					return fmt.Errorf("epic finalization failed: %w", err)
				}
				return nil // exit code 0

			case handlers.Continue:
				// Normal forward progress — state already updated in memory by handler.

			case handlers.Retry:
				// Non-fatal issue (build/test failure, git commit failure).
				// The handler rolled back changes; the loop retries on the next iteration.
			}

		case types.OutcomeFailure:
			if err := handlers.HandleFailure(ctx); err != nil {
				// Fatal: max retries reached, task blocked — exit code 1.
				return err
			}
			// Non-fatal: below max retries — loop retries on the next iteration.

		case types.OutcomeBug:
			if err := handlers.HandleBug(ctx); err != nil {
				// Fatal: nested bug detected in a bugfix task — exit code 1.
				return err
			}
			// Bug scheduled — bugfix task set as active; loop continues.

		case types.OutcomeEpicComplete:
			// Agent reported EPIC_COMPLETE directly (e.g., pre-flight check in docs task).
			if err := handlers.HandleEpicComplete(ctx); err != nil {
				// Tier 3: epic commit failure — exit code 1 (CI-6).
				return fmt.Errorf("epic finalization failed: %w", err)
			}
			return nil // exit code 0
		}
	}

	// Max iterations reached — this is a clean exit, not an error.
	log.Warning(fmt.Sprintf("max iterations (%d) reached — exiting", cfg.MaxIterations))
	return nil // exit code 0
}
