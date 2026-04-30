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
const postEpicKBEntrypoint = "docs/kb/README.md"

// runPostEpicKB executes best-effort KB synthesis after epic finalization.
// It never mutates runtime task pointers and never reopens the epic on failure.
func (o *Orchestrator) runPostEpicKB(ctx context.Context, state *types.ProjectState) error {
	if !o.cfg.KBEnabled {
		return nil
	}

	o.logger.Section(fmt.Sprintf("POST-EPIC KB — %s", state.CurrentEpic.ID))

	contextBody := strings.Join([]string{
		fmt.Sprintf("The epic `%s` has already been completed and finalized.", state.CurrentEpic.ID),
		"Use the documentation workflow for this post-epic knowledge base synthesis pass.",
		fmt.Sprintf("Start at `%s` to locate the relevant repository knowledge-base surface before editing.", postEpicKBEntrypoint),
		"Synthesize or update knowledge base content from the archived runtime snapshot and session logs.",
		fmt.Sprintf("Runtime archive: `%s`", filepath.Join(o.paths.DougDir, "logs", "archives", state.CurrentEpic.ID)),
		fmt.Sprintf("Session logs: `%s`", filepath.Join(o.paths.DougDir, "logs", "sessions", state.CurrentEpic.ID)),
		"Write KB output only under `docs/kb/`. Do not create or modify KB artifacts anywhere else in the repository, including under `.doug/`.",
		"Do not reopen or modify epic runtime state. Report `SUCCESS` when KB synthesis is done.",
	}, "\n")

	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:      postEpicKBTaskID,
		TaskType:    types.TaskTypeDocumentation,
		DougDir:     o.paths.DougDir,
		Description: "Synthesize or update repository KB documentation for the completed epic.",
		AcceptanceCriteria: []string{
			"Use the documentation workflow and start from `docs/kb/README.md` as the KB entrypoint.",
			"Write KB output only under `docs/kb/`.",
			"Do not reopen or modify epic runtime state while synthesizing KB updates.",
		},
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
	activeTaskPath := filepath.Join(o.paths.DougDir, "ACTIVE_TASK.md")
	kbRoot := filepath.Join(o.paths.ProjectRoot, "docs", "kb")
	_, agentErr := o.execBackend().Run(ctx, agent.RunRequest{
		Phase: agent.RunPhasePostEpicKB,
		Task: agent.TaskContext{
			ID:         postEpicKBTaskID,
			Type:       string(types.TaskTypeDocumentation),
			Attempt:    1,
			MaxRetries: 1,
			EpicID:     state.CurrentEpic.ID,
			EpicName:   state.CurrentEpic.Name,
		},
		Brief: agent.CanonicalBrief{
			Path:      activeTaskPath,
			Format:    agent.BriefFormatMarkdown,
			Authority: "doug",
		},
		ContextLoadOrder: []agent.ContextInput{
			{Kind: agent.ContextInputProjectInstructions, Path: filepath.Join(o.paths.ProjectRoot, "AGENTS.md"), Required: false},
			{Kind: agent.ContextInputProductContext, Path: filepath.Join(o.paths.DougDir, "PRD.md"), Required: false},
			{Kind: agent.ContextInputCanonicalBrief, Path: activeTaskPath, Required: true},
		},
		Routing: agent.RoutingInputs{
			Workflow:  "post_epic_kb",
			SkillName: skillName,
		},
		Policy: agent.PolicyInputs{},
		Restrictions: agent.RestrictionHooks{
			Read:  agent.RestrictionHook{Mode: agent.RestrictionModeInherit},
			Write: agent.RestrictionHook{Mode: agent.RestrictionModeAllowList, Paths: []string{kbRoot, activeTaskPath}},
		},
		Command:           resolvedCmd,
		ProjectRoot:       o.paths.ProjectRoot,
		HeartbeatInterval: heartbeatEvery,
		HeartbeatFn: func(elapsed time.Duration) {
			o.logger.Info(fmt.Sprintf("[%s] +%s", postEpicKBTaskID, elapsed.Round(time.Second)))
		},
		Output: outputLog,
	})
	if closeErr := outputLog.Close(); closeErr != nil {
		o.logger.Warning(fmt.Sprintf("close post-epic KB output log: %v", closeErr))
	}
	if agentErr != nil {
		o.logger.Warning(fmt.Sprintf("post-epic KB agent exited with error: %v — reading session result anyway", agentErr))
	}

	result, parseErr := agent.ParseSessionResult(activeTaskPath)
	if parseErr != nil {
		return fmt.Errorf("parse post-epic KB session result: %w", parseErr)
	}

	if err := agent.ArchiveActiveTask(o.paths.DougDir, o.paths.LogsDir, state.CurrentEpic.ID, postEpicKBTaskID, 1); err != nil {
		o.logger.Warning(fmt.Sprintf("post-epic KB session archive failed: %v", err))
	}
	if err := validatePostEpicKBPaths(o.paths.ProjectRoot); err != nil {
		return err
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

func validatePostEpicKBPaths(projectRoot string) error {
	paths, err := git.PendingPaths(projectRoot)
	if err != nil {
		return fmt.Errorf("inspect post-epic KB changes: %w", err)
	}

	for _, path := range paths {
		if strings.HasPrefix(filepath.ToSlash(path), "docs/kb/") {
			continue
		}
		return fmt.Errorf("post-epic KB produced changes outside docs/kb/: %q", path)
	}
	return nil
}
