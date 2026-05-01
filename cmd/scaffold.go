package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/build"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/handlers"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

var (
	scaffoldLoadConfig                  = config.LoadConfig
	scaffoldCheckDeps                   = orchestrator.CheckDependencies
	scaffoldNewBuild                    = build.NewBuildSystem
	scaffoldRunAgent      agent.Backend = agent.DefaultBackend{}
	scaffoldParseResult                 = agent.ParseSessionResult
	scaffoldHandleSuccess               = handlers.HandleSuccess
	scaffoldHandleFailure               = handlers.HandleFailure
)

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Construct the scaffold task, invoke the agent once, and dispatch the outcome",
	Long:  "Validate scaffold preconditions, build the synthetic scaffold task from .doug/plan/manifest.yaml, invoke the scaffold agent exactly once, and dispatch the result through the existing outcome handlers.",
	RunE:  runScaffold,
}

func runScaffold(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	return scaffoldProjectContext(cmd.Context(), projectRoot)
}

func scaffoldProject(projectRoot string) error {
	return scaffoldProjectContext(context.Background(), projectRoot)
}

func scaffoldProjectContext(ctx context.Context, projectRoot string) error {
	paths := orchestrator.NewPaths(projectRoot)
	logger := log.New()

	if _, err := os.Stat(paths.StatePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s not found — run doug init first", paths.StatePath)
		}
		return fmt.Errorf("stat %s: %w", paths.StatePath, err)
	}

	manifest, err := types.LoadManifest(paths.ManifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s not found — generate .doug/plan/manifest.yaml before running doug scaffold", paths.ManifestPath)
		}
		return err
	}

	cfg, err := scaffoldLoadConfig(paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg.BuildSystem = resolveScaffoldBuildSystem(manifest)
	cfg.MaxRetries = 1
	cfg.KBEnabled = false

	if err := scaffoldCheckDeps(cfg); err != nil {
		return fmt.Errorf("%w — install the missing tools and add them to PATH, then retry", err)
	}

	buildSystem, err := scaffoldNewBuild(cfg.BuildSystem, projectRoot)
	if err != nil {
		return fmt.Errorf("build system: %w", err)
	}

	task, err := buildScaffoldTask(manifest)
	if err != nil {
		return err
	}

	prep, err := agent.PrepareExecution(string(agent.RunPhaseScaffold), string(task.Type), task.ID, cfg.ScaffoldAgentCommand, cfg.Policy)
	if err != nil {
		return fmt.Errorf("prepare scaffold execution: %w", err)
	}

	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:             task.ID,
		TaskType:           task.Type,
		DougDir:            paths.DougDir,
		Description:        task.Description,
		AcceptanceCriteria: task.AcceptanceCriteria,
		Attempts:           1,
		MaxRetries:         1,
		BuildSystem:        cfg.BuildSystem,
		ContextSections: []agent.ActiveTaskSection{
			{
				Heading: "Manifest Context",
				Body:    manifestContextBody(manifest),
			},
		},
	}, logger); err != nil {
		return fmt.Errorf("write scaffold active task: %w", err)
	}

	projectState, err := state.LoadProjectState(paths.StatePath)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return fmt.Errorf("%s not found — run doug init first", paths.StatePath)
		}
		return fmt.Errorf("load project state: %w", err)
	}
	projectState.ActiveTask = types.TaskPointer{
		Type:     task.Type,
		ID:       task.ID,
		Attempts: 1,
	}

	loopCtx := &types.LoopContext{
		TaskID:        task.ID,
		TaskType:      task.Type,
		Attempts:      1,
		CurrentEpic:   projectState.CurrentEpic,
		Config:        cfg,
		BuildSystem:   buildSystem,
		ProjectRoot:   projectRoot,
		TaskStartTime: time.Now(),
		State:         projectState,
		Tasks:         &types.Tasks{},
		StatePath:     filepath.Join(os.TempDir(), fmt.Sprintf("doug-scaffold-state-%d.yaml", time.Now().UnixNano())),
		TasksPath:     filepath.Join(os.TempDir(), fmt.Sprintf("doug-scaffold-tasks-%d.yaml", time.Now().UnixNano())),
		DougDir:       paths.DougDir,
		LogsDir:       paths.LogsDir,
		ChangelogPath: paths.ChangelogPath,
		Logger:        logger,
	}

	outputLogPath, err := scaffoldOutputLogPath(paths.LogsDir, projectState.CurrentEpic.ID, task.ID, loopCtx.Attempts)
	if err != nil {
		return err
	}
	outputLog, err := os.Create(outputLogPath)
	if err != nil {
		return fmt.Errorf("create agent output log: %w", err)
	}
	defer closeScaffoldOutputLog(outputLog, logger)

	logger.Info(fmt.Sprintf("invoking agent for task %s", task.ID))
	heartbeatEvery := time.Duration(cfg.AgentHeartbeatSeconds) * time.Second
	contract := agent.ScaffoldContract(projectRoot, paths.DougDir, paths.ManifestPath)
	activeTaskPath := contract.Brief.Path
	agentResp, agentErr := scaffoldRunAgent.Run(ctx, agent.RunRequest{
		Phase: agent.RunPhaseScaffold,
		Task: agent.TaskContext{
			ID:         task.ID,
			Type:       string(task.Type),
			Attempt:    loopCtx.Attempts,
			MaxRetries: cfg.MaxRetries,
			EpicID:     projectState.CurrentEpic.ID,
			EpicName:   projectState.CurrentEpic.Name,
		},
		Brief:            contract.Brief,
		ContextLoadOrder: contract.ContextLoadOrder,
		Artifacts:        contract.Artifacts,
		Routing: agent.RoutingInputs{
			Workflow:      "scaffold",
			SkillName:     prep.SkillName,
			ExecutionMode: prep.Exec.ExecutionMode,
		},
		Policy: agent.PolicyInputs{
			SessionPolicy:   prep.Exec.RoutingProfile,
			ToolPolicy:      prep.Exec.ToolPolicy,
			SessionDefaults: prep.Exec.SessionDefaults,
		},
		Restrictions:      contract.Restrictions,
		Command:           prep.ResolvedCommand,
		ProjectRoot:       projectRoot,
		HeartbeatInterval: heartbeatEvery,
		HeartbeatFn: func(elapsed time.Duration) {
			logger.Info(fmt.Sprintf("[%s] +%s", task.ID, elapsed.Round(time.Second)))
		},
		Output: outputLog,
	})
	if agentErr != nil {
		logger.Warning(fmt.Sprintf("agent exited with error: %v — reading session result anyway", agentErr))
	}
	if metaErr := agent.WriteRunMetadata(outputLogPath, agentResp, agentErr); metaErr != nil {
		logger.Warning(fmt.Sprintf("write agent run metadata: %v", metaErr))
	}

	result, err := scaffoldParseResult(activeTaskPath)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to parse session result from %s: %v — treating as FAILURE", activeTaskPath, err))
		result = &types.SessionResult{Outcome: types.OutcomeFailure}
	}

	agentDurationSeconds := int(agentResp.Duration.Seconds())
	switch result.Outcome {
	case types.OutcomeSuccess:
		successResult, err := scaffoldHandleSuccess(loopCtx, result, agentDurationSeconds)
		if err != nil {
			return fmt.Errorf("HandleSuccess: %w", err)
		}
		switch successResult.Kind {
		case handlers.Continue:
			return nil
		case handlers.BuildFailure:
			return fmt.Errorf("scaffold verification failed")
		case handlers.Retry:
			return fmt.Errorf("scaffold requested retry after a single invocation")
		case handlers.EpicComplete:
			return nil
		default:
			return fmt.Errorf("unsupported scaffold success result %d", successResult.Kind)
		}
	case types.OutcomeFailure:
		if err := scaffoldHandleFailure(loopCtx, agentDurationSeconds); err != nil {
			return err
		}
		return fmt.Errorf("scaffold failed without retry")
	default:
		return fmt.Errorf("unsupported scaffold outcome %q", result.Outcome)
	}
}

func closeScaffoldOutputLog(outputLog io.Closer, logger log.Logger) {
	if err := outputLog.Close(); err != nil {
		logger.Warning(fmt.Sprintf("close agent output log: %v", err))
	}
}

func scaffoldOutputLogPath(logsDir, epicID, taskID string, attempt int) (string, error) {
	outputLogDir := filepath.Join(logsDir, "output")
	if epicID != "" {
		outputLogDir = filepath.Join(outputLogDir, epicID)
	}
	if err := os.MkdirAll(outputLogDir, 0o755); err != nil {
		return "", fmt.Errorf("create output log directory: %w", err)
	}
	return filepath.Join(outputLogDir, fmt.Sprintf("output-%s_attempt-%d.log", taskID, attempt)), nil
}

func resolveScaffoldBuildSystem(manifest *types.Manifest) string {
	if manifest == nil {
		return config.DefaultBuildSystem
	}
	return config.ResolveManifestBuildSystem(
		manifest.Scaffold.BuildSystem,
		manifest.Scaffold.PackageManager,
		manifest.Scaffold.Runtime,
	)
}

func buildScaffoldTask(manifest *types.Manifest) (types.Task, error) {
	if manifest == nil {
		return types.Task{}, fmt.Errorf("build scaffold task: manifest is required")
	}

	descriptorParts := []string{
		manifest.Scaffold.Language,
		manifest.Scaffold.Runtime,
		manifest.Scaffold.Framework,
	}

	return types.Task{
		ID:     "SCAFFOLD",
		Type:   types.TaskTypeScaffold,
		Status: types.StatusInProgress,
		Description: fmt.Sprintf(
			"Materialize the initial %s project scaffold for %q using %s.",
			manifest.Project.Mode,
			manifest.Project.Name,
			strings.Join(descriptorParts, "/"),
		),
		AcceptanceCriteria: []string{
			"Create the day-0 project scaffold described by the manifest.",
			"Install the requested dependencies and package manager layout.",
			"Honor every manifest constraint provided in the structured context below.",
		},
	}, nil
}

func manifestContextBody(manifest *types.Manifest) string {
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Sprintf("Failed to render manifest context: %v\n", err)
	}

	return "The following manifest is the source of truth for this scaffold task.\n\n```yaml\n" + string(data) + "```\n"
}
