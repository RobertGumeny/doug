package orchestrator

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/git"
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
	sessionDir := filepath.Join(logsDir, "sessions", epicID)
	if attempts > 0 {
		return []string{filepath.Join(sessionDir, fmt.Sprintf("session-%s_attempt-%d.md", taskID, attempts))}
	}

	matches, err := filepath.Glob(filepath.Join(sessionDir, fmt.Sprintf("session-%s_attempt-*.md", taskID)))
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
