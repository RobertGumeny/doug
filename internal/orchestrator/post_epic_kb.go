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
	"github.com/robertgumeny/doug/internal/types"
)

const postEpicKBTaskID = "POST_EPIC_KB"

// runPostEpicKB executes best-effort KB synthesis after epic finalization.
// It never mutates runtime task pointers and never reopens the epic on failure.
func (o *Orchestrator) runPostEpicKB(ctx context.Context, state *types.ProjectState) error {
	if !o.cfg.KBEnabled {
		return nil
	}

	o.logger.Section(fmt.Sprintf("POST-EPIC KB — %s", state.CurrentEpic.ID))

	contextBody := strings.Join([]string{
		fmt.Sprintf("The epic `%s` has already been completed and finalized.", state.CurrentEpic.ID),
		"Synthesize or update knowledge base content from the archived runtime snapshot and session logs.",
		fmt.Sprintf("Runtime archive: `%s`", filepath.Join(o.paths.DougDir, "logs", "archives", state.CurrentEpic.ID)),
		fmt.Sprintf("Session logs: `%s`", filepath.Join(o.paths.DougDir, "logs", "sessions", state.CurrentEpic.ID)),
		"Do not reopen or modify epic runtime state. Report `SUCCESS` when KB synthesis is done.",
	}, "\n")

	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:      postEpicKBTaskID,
		TaskType:    types.TaskTypeDocumentation,
		DougDir:     o.paths.DougDir,
		Attempts:    1,
		MaxRetries:  1,
		BuildSystem: o.cfg.BuildSystem,
		ContextSections: []agent.ActiveTaskSection{
			{Heading: "Post-Epic KB Context", Body: contextBody},
		},
	}, o.logger); err != nil {
		return fmt.Errorf("write post-epic KB task: %w", err)
	}
	defer func() {
		if err := agent.CleanupActiveTask(o.paths.DougDir); err != nil {
			o.logger.Warning(fmt.Sprintf("post-epic KB cleanup failed: %v", err))
		}
	}()

	skillName, _ := agent.GetSkillForTaskType(string(types.TaskTypeDocumentation), o.paths.SkillsConfigPath)
	resolvedCmd := strings.ReplaceAll(o.cfg.RunAgentCommand, "{{skill_name}}", skillName)
	resolvedCmd = strings.ReplaceAll(resolvedCmd, "{{task_id}}", postEpicKBTaskID)

	outputLogDir := filepath.Join(o.paths.LogsDir, "output", state.CurrentEpic.ID)
	if err := os.MkdirAll(outputLogDir, 0o755); err != nil {
		return fmt.Errorf("create post-epic KB output log dir: %w", err)
	}
	outputLogPath := filepath.Join(outputLogDir, "output-"+strings.ToLower(postEpicKBTaskID)+".log")
	outputLog, err := os.Create(outputLogPath)
	if err != nil {
		return fmt.Errorf("create post-epic KB output log: %w", err)
	}

	heartbeatEvery := time.Duration(o.cfg.AgentHeartbeatSeconds) * time.Second
	_, agentErr := agent.RunAgent(ctx, resolvedCmd, o.paths.ProjectRoot, heartbeatEvery, func(elapsed time.Duration) {
		o.logger.Info(fmt.Sprintf("[%s] +%s", postEpicKBTaskID, elapsed.Round(time.Second)))
	}, outputLog)
	if closeErr := outputLog.Close(); closeErr != nil {
		o.logger.Warning(fmt.Sprintf("close post-epic KB output log: %v", closeErr))
	}
	if agentErr != nil {
		o.logger.Warning(fmt.Sprintf("post-epic KB agent exited with error: %v — reading session result anyway", agentErr))
	}

	activeTaskPath := filepath.Join(o.paths.DougDir, "ACTIVE_TASK.md")
	result, parseErr := agent.ParseSessionResult(activeTaskPath)
	if parseErr != nil {
		return fmt.Errorf("parse post-epic KB session result: %w", parseErr)
	}

	if err := agent.ArchiveActiveTask(o.paths.DougDir, o.paths.LogsDir, state.CurrentEpic.ID, postEpicKBTaskID, 1); err != nil {
		o.logger.Warning(fmt.Sprintf("post-epic KB session archive failed: %v", err))
	}

	switch result.Outcome {
	case types.OutcomeSuccess, types.OutcomeEpicComplete:
		if err := git.Commit("docs: synthesize KB for "+state.CurrentEpic.ID, o.paths.ProjectRoot); err != nil {
			if !errors.Is(err, git.ErrNothingToCommit) {
				return fmt.Errorf("commit post-epic KB changes: %w", err)
			}
			o.logger.Info("post-epic KB produced no new changes to commit")
		}
		o.logger.Success(fmt.Sprintf("post-epic KB synthesis completed for %s", state.CurrentEpic.ID))
		return nil
	default:
		return fmt.Errorf("post-epic KB reported outcome %s", result.Outcome)
	}
}
