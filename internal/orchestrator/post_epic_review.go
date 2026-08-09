package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/git"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/status"
	"github.com/robertgumeny/doug/internal/types"
)

const (
	postEpicReviewTaskID = "POST_EPIC_REVIEW"

	reviewDimensionAcceptanceFaithfulness  = "acceptance-criteria faithfulness"
	reviewDimensionLikelyRegressions       = "likely regressions"
	reviewDimensionImplementationCoherence = "implementation coherence"
	reviewDimensionReleaseReadiness        = "release-readiness"
)

type committedDiffLookup func(sha, projectRoot string) (string, error)

type postEpicReviewTaskInput struct {
	TaskID             string
	Description        string
	AcceptanceCriteria []string
	Outcome            string
	ChangelogEntry     string
	CommitSHA          string
	CommittedDiff      string
	Warnings           []string
}

type postEpicReviewInput struct {
	EpicID           string
	ReviewDimensions []string
	Tasks            []postEpicReviewTaskInput
	Warnings         []string
}

// runPostEpicReview executes the automatic advisory review pass for a completed epic.
// The pass is warning-only: agent and result-parse failures are reported to the
// operator but do not reopen finalized runtime state or fail the caller.
func (o *Orchestrator) runPostEpicReview(ctx context.Context, projectState *types.ProjectState, tasks *types.Tasks) error {
	if !o.cfg.ReviewEnabled {
		return nil
	}
	_, err := o.executePostEpicReview(ctx, projectState, tasks)
	return err
}

func postEpicReviewIncompleteWarning(epicID string, cause any) string {
	return fmt.Sprintf("advisory post-epic review did not complete for %s: %v — inspect the completed epic more carefully; retry with `doug review %s`", epicID, cause, epicID)
}

// ReviewCompletedEpic reruns the advisory review for an already-completed epic
// using only the finalized runtime archive and archived session logs. It ignores
// review_enabled because that flag only controls the automatic post-run pass.
func (o *Orchestrator) ReviewCompletedEpic(ctx context.Context, epicID string) (string, error) {
	projectState, tasks, err := o.loadCompletedEpicReviewArchive(epicID)
	if err != nil {
		return "", err
	}
	return o.executePostEpicReview(ctx, projectState, tasks)
}

func (o *Orchestrator) loadCompletedEpicReviewArchive(epicID string) (*types.ProjectState, *types.Tasks, error) {
	runtimeArchive := filepath.Join(o.paths.LogsDir, "epics", epicID)
	sessionArchive := filepath.Join(o.paths.LogsDir, "epics", epicID)
	if info, err := os.Stat(runtimeArchive); err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("completed runtime archive for epic %s is missing at %s; run the epic to completion before reviewing it", epicID, runtimeArchive)
		}
		return nil, nil, fmt.Errorf("inspect runtime archive %s: %w", runtimeArchive, err)
	} else if !info.IsDir() {
		return nil, nil, fmt.Errorf("completed runtime archive for epic %s is not a directory: %s", epicID, runtimeArchive)
	}
	if info, err := os.Stat(sessionArchive); err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("session archive for epic %s is missing at %s; archived task sessions are required for review", epicID, sessionArchive)
		}
		return nil, nil, fmt.Errorf("inspect session archive %s: %w", sessionArchive, err)
	} else if !info.IsDir() {
		return nil, nil, fmt.Errorf("session archive for epic %s is not a directory: %s", epicID, sessionArchive)
	}
	sessionMatches, err := filepath.Glob(filepath.Join(sessionArchive, "*", "attempt-*", "session.md"))
	if err != nil {
		return nil, nil, fmt.Errorf("inspect session archive for epic %s: %w", epicID, err)
	}
	if len(sessionMatches) == 0 {
		return nil, nil, fmt.Errorf("session archive for epic %s at %s has no archived task sessions; run the epic to completion before reviewing it", epicID, sessionArchive)
	}

	projectState, err := state.LoadProjectState(filepath.Join(runtimeArchive, "project-state.yaml"))
	if err != nil {
		return nil, nil, fmt.Errorf("load completed runtime archive state for epic %s: %w", epicID, err)
	}
	tasks, err := state.LoadTasks(filepath.Join(runtimeArchive, "tasks.yaml"))
	if err != nil {
		return nil, nil, fmt.Errorf("load completed runtime archive tasks for epic %s: %w", epicID, err)
	}
	if projectState.CurrentEpic.ID != epicID {
		return nil, nil, fmt.Errorf("runtime archive mismatch: requested epic %s but archived state is for %s", epicID, projectState.CurrentEpic.ID)
	}
	if tasks.Epic.ID != "" && tasks.Epic.ID != epicID {
		return nil, nil, fmt.Errorf("runtime archive mismatch: requested epic %s but archived tasks are for %s", epicID, tasks.Epic.ID)
	}
	if projectState.CurrentEpic.CompletedAt == nil || strings.TrimSpace(*projectState.CurrentEpic.CompletedAt) == "" {
		return nil, nil, fmt.Errorf("epic %s archive is not completed; active or in-progress epics cannot be reviewed until completed archives exist", epicID)
	}
	return projectState, tasks, nil
}

func (o *Orchestrator) executePostEpicReview(ctx context.Context, projectState *types.ProjectState, tasks *types.Tasks) (string, error) {
	epicID := projectState.CurrentEpic.ID
	o.logger.Section(fmt.Sprintf("POST-EPIC REVIEW — %s", epicID))

	reviewPath, err := nextPostEpicReviewArtifactPath(o.paths.LogsDir, epicID)
	if err != nil {
		return "", fmt.Errorf("allocate post-epic review artifact: %w", err)
	}
	if err := state.AtomicWrite(reviewPath, []byte(agent.PostEpicReviewArtifactSkeleton())); err != nil {
		return "", fmt.Errorf("write post-epic review artifact skeleton: %w", err)
	}

	reviewBrief := assemblePostEpicReviewBrief(o.paths.ProjectRoot, o.paths.LogsDir, epicID, tasks.Epic.Tasks, projectState.Metrics.Tasks)
	contextBody := strings.Join([]string{
		fmt.Sprintf("The epic `%s` has already been completed and finalized.", epicID),
		"Use the documentation workflow for this advisory post-epic review pass.",
		fmt.Sprintf("Review artifact: `%s`", reviewPath),
		"Doug already created that artifact as a markdown skeleton. Fill in the existing skeleton in place; do not create a git commit.",
		"Assess faithfulness to task acceptance criteria, likely regressions, implementation coherence, release readiness, and evidence reviewed.",
		"Treat the structured review input below as the primary evidence bundle. Use warnings in the input as traceability caveats, not as hard failures.",
		"Report `SUCCESS` when the review artifact has been filled in.",
	}, "\n")

	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:      postEpicReviewTaskID,
		TaskType:    types.TaskTypeDocumentation,
		ProjectRoot: o.paths.ProjectRoot,
		DougDir:     o.paths.DougDir,
		Description: "Review the completed epic for acceptance-criteria faithfulness, likely regressions, implementation coherence, and release readiness.",
		AcceptanceCriteria: []string{
			"Fill in the pre-created review artifact skeleton.",
			"Keep review output under `.doug/logs/epics/` and do not create a git commit.",
			"Base the review on the structured post-epic input and available evidence.",
		},
		Attempts:    1,
		MaxRetries:  1,
		BuildSystem: o.cfg.BuildSystem,
		ContextSections: []agent.ActiveTaskSection{
			{Heading: "Post-Epic Review Context", Body: contextBody},
			{Heading: "Structured Review Input", Body: reviewBrief},
		},
	}, o.logger); err != nil {
		return "", fmt.Errorf("write post-epic review task: %w", err)
	}
	defer func() {
		if err := agent.CleanupActiveTask(o.paths.DougDir); err != nil {
			o.logger.Warning(fmt.Sprintf("post-epic review cleanup failed: %v", err))
		}
	}()

	prep, prepErr := agent.PrepareExecution(string(agent.RunPhasePostEpicReview), string(types.TaskTypeDocumentation), postEpicReviewTaskID)
	if prepErr != nil {
		return "", fmt.Errorf("prepare post-epic review execution: %w", prepErr)
	}

	heartbeatEvery := time.Duration(o.cfg.AgentHeartbeatSeconds) * time.Second
	liveStatus := newAgentStatus(postEpicReviewTaskID, heartbeatEvery, o.logger)
	contract := agent.PostEpicReviewContract(o.paths.ProjectRoot, o.paths.DougDir, epicID)
	activeTaskPath := contract.Brief.Path
	reviewTaskContext := agent.TaskContext{
		ID:         postEpicReviewTaskID,
		Type:       string(types.TaskTypeDocumentation),
		Attempt:    1,
		MaxRetries: 1,
		EpicID:     epicID,
		EpicName:   projectState.CurrentEpic.Name,
	}
	if err := agent.WriteAttemptStart(o.paths.ProjectRoot, agent.RunPhasePostEpicReview, reviewTaskContext, time.Now()); err != nil {
		return "", fmt.Errorf("write post-epic review attempt-start marker: %w", err)
	}
	agentResp, agentErr := o.execBackend().Run(ctx, agent.RunRequest{
		Phase:            agent.RunPhasePostEpicReview,
		Task:             reviewTaskContext,
		Brief:            contract.Brief,
		ContextLoadOrder: contract.ContextLoadOrder,
		Artifacts:        contract.Artifacts,
		Routing: agent.RoutingInputs{
			Workflow:        "post_epic_review",
			SkillName:       prep.SkillName,
			InteractionMode: prep.InteractionMode,
		},
		Restrictions:      contract.Restrictions,
		InitialPrompt:     prep.InitialPrompt,
		ProjectRoot:       o.paths.ProjectRoot,
		HeartbeatInterval: heartbeatEvery,
		HeartbeatFn: func(elapsed time.Duration, activity string) {
			liveStatus.Heartbeat(elapsed, activity)
		},
	})
	liveStatus.Finish()
	o.logger.Info(status.FormatAgentEndSummary(agentResp.Duration, agentResp.FirstResponseMs, agentResp.ToolCallCount, agentResp.ProviderFailures))
	if agentErr != nil {
		o.logger.Warning(postEpicReviewIncompleteWarning(epicID, fmt.Sprintf("agent exited with error: %v", agentErr)))
	}

	artifactFilled, artifactErr := postEpicReviewArtifactFilled(reviewPath)
	if err := agent.ArchiveActiveTask(o.paths.DougDir, o.paths.LogsDir, epicID, postEpicReviewTaskID, 1); err != nil {
		o.logger.Warning(fmt.Sprintf("post-epic review session archive failed: %v", err))
	}
	if artifactErr != nil {
		o.logger.Warning(postEpicReviewIncompleteWarning(epicID, fmt.Sprintf("review artifact could not be inspected: %v", artifactErr)))
		return reviewPath, nil
	}
	if artifactFilled {
		o.logger.Success(fmt.Sprintf("post-epic review artifact written: %s", reviewPath))
		return reviewPath, nil
	}

	result, parseErr := agent.ParseSessionResult(activeTaskPath)
	if parseErr != nil {
		o.logger.Warning(postEpicReviewIncompleteWarning(epicID, fmt.Sprintf("result was not parseable: %v", parseErr)))
		return reviewPath, nil
	}
	if result.Outcome != types.OutcomeSuccess && result.Outcome != types.OutcomeEpicComplete {
		o.logger.Warning(postEpicReviewIncompleteWarning(epicID, fmt.Sprintf("reported outcome %s", result.Outcome)))
		return reviewPath, nil
	}

	o.logger.Success(fmt.Sprintf("post-epic review artifact written: %s", reviewPath))
	return reviewPath, nil
}

func postEpicReviewArtifactFilled(reviewPath string) (bool, error) {
	data, err := os.ReadFile(reviewPath)
	if err != nil {
		return false, err
	}
	return string(data) != agent.PostEpicReviewArtifactSkeleton(), nil
}

func nextPostEpicReviewArtifactPath(logsDir, epicID string) (string, error) {
	reviewDir := filepath.Join(logsDir, "epics", epicID)
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		return "", err
	}
	base := filepath.Join(reviewDir, "epic-review.md")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base, nil
	} else if err != nil {
		return "", err
	}
	for version := 2; ; version++ {
		candidate := filepath.Join(reviewDir, fmt.Sprintf("epic-review-v%d.md", version))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
}

// assemblePostEpicReviewBrief builds the deterministic, structured input used
// by the advisory post-epic review pass. Missing traceability data is recorded
// as task-local warnings/placeholders so one incomplete archive or unreachable
// commit does not prevent review of the rest of the epic.
func assemblePostEpicReviewBrief(projectRoot, logsDir, epicID string, tasks []types.Task, metrics []types.TaskMetric) string {
	input := assemblePostEpicReviewInput(projectRoot, logsDir, epicID, tasks, metrics, git.CommittedDiff)
	return renderPostEpicReviewBrief(input)
}

func assemblePostEpicReviewInput(projectRoot, logsDir, epicID string, tasks []types.Task, metrics []types.TaskMetric, diffLookup committedDiffLookup) postEpicReviewInput {
	input := postEpicReviewInput{
		EpicID: epicID,
		ReviewDimensions: []string{
			reviewDimensionAcceptanceFaithfulness,
			reviewDimensionLikelyRegressions,
			reviewDimensionImplementationCoherence,
			reviewDimensionReleaseReadiness,
		},
	}

	metricsByTask := latestMetricsByTaskID(metrics)
	for _, task := range tasks {
		if task.Type.IsSynthetic() {
			continue
		}

		assembled := postEpicReviewTaskInput{
			TaskID:             task.ID,
			Description:        task.Description,
			AcceptanceCriteria: append([]string(nil), task.AcceptanceCriteria...),
		}

		metric, ok := metricsByTask[task.ID]
		if !ok {
			assembled.Outcome = warningPlaceholder("missing task metric outcome")
			assembled.ChangelogEntry = warningPlaceholder("missing session changelog entry")
			assembled.CommitSHA = warningPlaceholder("missing commit SHA")
			assembled.CommittedDiff = warningPlaceholder("diff unavailable: missing commit SHA")
			assembled.Warnings = append(assembled.Warnings, "missing task metric; outcome, commit SHA, and diff are unavailable")
			input.Tasks = append(input.Tasks, assembled)
			input.Warnings = append(input.Warnings, fmt.Sprintf("%s: missing task metric", task.ID))
			continue
		}

		assembled.Outcome = metric.Outcome
		assembled.CommitSHA = strings.TrimSpace(metric.CommitSHA)

		if result, warnings := readArchivedTaskResult(logsDir, epicID, task.ID, metric.Attempts); len(warnings) == 0 {
			assembled.ChangelogEntry = result.ChangelogEntry
			if assembled.Outcome == "" {
				assembled.Outcome = string(result.Outcome)
			}
		} else {
			assembled.ChangelogEntry = warningPlaceholder("session changelog entry unavailable")
			assembled.Warnings = append(assembled.Warnings, warnings...)
			for _, warning := range warnings {
				input.Warnings = append(input.Warnings, fmt.Sprintf("%s: %s", task.ID, warning))
			}
		}

		if strings.TrimSpace(assembled.ChangelogEntry) == "" {
			assembled.ChangelogEntry = warningPlaceholder("empty changelog entry")
			assembled.Warnings = append(assembled.Warnings, "empty changelog entry")
		}

		if assembled.CommitSHA == "" {
			assembled.CommitSHA = warningPlaceholder("missing commit SHA")
			assembled.CommittedDiff = warningPlaceholder("diff unavailable: missing commit SHA")
			assembled.Warnings = append(assembled.Warnings, "missing commit SHA; committed diff unavailable")
			input.Warnings = append(input.Warnings, fmt.Sprintf("%s: missing commit SHA", task.ID))
			input.Tasks = append(input.Tasks, assembled)
			continue
		}

		diff, err := diffLookup(assembled.CommitSHA, projectRoot)
		if err != nil {
			assembled.CommittedDiff = warningPlaceholder(fmt.Sprintf("diff unavailable for %s: %v", assembled.CommitSHA, err))
			assembled.Warnings = append(assembled.Warnings, fmt.Sprintf("committed diff lookup failed for %s: %v", assembled.CommitSHA, err))
			input.Warnings = append(input.Warnings, fmt.Sprintf("%s: committed diff lookup failed for %s", task.ID, assembled.CommitSHA))
		} else if strings.TrimSpace(diff) == "" {
			assembled.CommittedDiff = warningPlaceholder(fmt.Sprintf("empty committed diff for %s", assembled.CommitSHA))
			assembled.Warnings = append(assembled.Warnings, fmt.Sprintf("empty committed diff for %s", assembled.CommitSHA))
			input.Warnings = append(input.Warnings, fmt.Sprintf("%s: empty committed diff for %s", task.ID, assembled.CommitSHA))
		} else {
			assembled.CommittedDiff = diff
		}

		input.Tasks = append(input.Tasks, assembled)
	}

	return input
}

func latestMetricsByTaskID(metrics []types.TaskMetric) map[string]types.TaskMetric {
	byID := make(map[string]types.TaskMetric, len(metrics))
	for _, metric := range metrics {
		if metric.TaskID == "" {
			continue
		}
		byID[metric.TaskID] = metric
	}
	return byID
}

func readArchivedTaskResult(logsDir, epicID, taskID string, attempts int) (types.SessionResult, []string) {
	paths := archivedTaskResultCandidates(logsDir, epicID, taskID, attempts)
	var parseWarnings []string
	for _, path := range paths {
		result, err := agent.ParseSessionResult(path)
		if err == nil {
			return *result, nil
		}
		parseWarnings = append(parseWarnings, fmt.Sprintf("parse session result %s: %v", path, err))
	}
	if len(paths) == 0 {
		return types.SessionResult{}, []string{"session archive unavailable"}
	}
	return types.SessionResult{}, parseWarnings
}

func archivedTaskResultCandidates(logsDir, epicID, taskID string, attempts int) []string {
	newSessionDir := filepath.Join(logsDir, "epics", epicID, taskID)
	legacySessionDir := filepath.Join(logsDir, "sessions", epicID)
	if attempts > 0 {
		return []string{
			filepath.Join(newSessionDir, fmt.Sprintf("attempt-%d", attempts), "session.md"),
			filepath.Join(legacySessionDir, fmt.Sprintf("session-%s_attempt-%d.md", taskID, attempts)),
		}
	}

	matches, err := filepath.Glob(filepath.Join(newSessionDir, "attempt-*", "session.md"))
	if err != nil || len(matches) == 0 {
		matches, err = filepath.Glob(filepath.Join(legacySessionDir, fmt.Sprintf("session-%s_attempt-*.md", taskID)))
	}
	if err != nil || len(matches) == 0 {
		return nil
	}
	sort.Strings(matches)
	return []string{matches[len(matches)-1]}
}

func renderPostEpicReviewBrief(input postEpicReviewInput) string {
	var sb strings.Builder
	sb.WriteString("# Post-Epic Review Input\n\n")
	fmt.Fprintf(&sb, "Epic: `%s`\n\n", input.EpicID)

	sb.WriteString("## Review Dimensions\n\n")
	for _, dimension := range input.ReviewDimensions {
		fmt.Fprintf(&sb, "- %s\n", dimension)
	}
	sb.WriteString("\n")

	if len(input.Warnings) > 0 {
		sb.WriteString("## Assembly Warnings\n\n")
		for _, warning := range input.Warnings {
			fmt.Fprintf(&sb, "- %s\n", warning)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Task Evidence\n")
	for _, task := range input.Tasks {
		fmt.Fprintf(&sb, "\n### %s\n\n", task.TaskID)
		fmt.Fprintf(&sb, "**Description:** %s\n\n", task.Description)
		sb.WriteString("**Acceptance Criteria:**\n")
		if len(task.AcceptanceCriteria) == 0 {
			sb.WriteString("- ⚠️ No acceptance criteria recorded.\n")
		} else {
			for _, criterion := range task.AcceptanceCriteria {
				fmt.Fprintf(&sb, "- %s\n", criterion)
			}
		}
		fmt.Fprintf(&sb, "\n**Outcome:** %s\n", task.Outcome)
		fmt.Fprintf(&sb, "**Changelog Entry:** %s\n", task.ChangelogEntry)
		fmt.Fprintf(&sb, "**Recorded Commit SHA:** %s\n", task.CommitSHA)

		if len(task.Warnings) > 0 {
			sb.WriteString("\n**Task Warnings:**\n")
			for _, warning := range task.Warnings {
				fmt.Fprintf(&sb, "- %s\n", warning)
			}
		}

		sb.WriteString("\n**Committed Diff:**\n\n")
		sb.WriteString("```diff\n")
		sb.WriteString(strings.TrimRight(task.CommittedDiff, "\n"))
		sb.WriteString("\n```\n")
	}

	return sb.String()
}

func warningPlaceholder(message string) string {
	return "⚠️ " + message
}
