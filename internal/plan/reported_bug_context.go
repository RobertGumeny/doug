package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/dougpath"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// reportedBugTerminalStatuses contains all status values that indicate a reported
// bug is no longer unresolved and should be excluded from planning intake.
var reportedBugTerminalStatuses = map[string]bool{
	"fixed":    true,
	"resolved": true,
	"done":     true,
	"closed":   true,
}

type ReportedBugContext struct {
	BugID          string
	SourceEpicID   string
	SourcePath     string
	Status         string
	Severity       string
	Summary        string
	EpicStatus     *types.EpicLifecycleStatus
	PlanningAction string
}

type reportedBugFrontmatter struct {
	BugID    string `yaml:"bug_id"`
	Status   string `yaml:"status"`
	Severity string `yaml:"severity"`
}

// LoadReportedBugContext returns unresolved reported bugs for use in the
// Doug-owned planning brief. warn is called (when non-nil) for each reported-bug
// file that is skipped due to a parse or validation problem; it receives a
// human-readable message that names the problematic path.
func LoadReportedBugContext(projectRoot string, warn func(string)) ([]ReportedBugContext, error) {
	paths := dougpath.New(projectRoot)
	bugsRoots := []string{
		paths.IntakeBugsDir(),
		filepath.Join(paths.LogsDir, dougpath.BugsDirName),
	}

	contexts := make([]ReportedBugContext, 0)
	for _, bugsRoot := range bugsRoots {
		rootContexts, err := loadReportedBugContextFromRoot(projectRoot, bugsRoot, warn)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, rootContexts...)
	}

	sort.Slice(contexts, func(i, j int) bool {
		if contexts[i].SourceEpicID != contexts[j].SourceEpicID {
			return contexts[i].SourceEpicID < contexts[j].SourceEpicID
		}
		return contexts[i].SourcePath < contexts[j].SourcePath
	})
	return contexts, nil
}

func loadReportedBugContextFromRoot(projectRoot, bugsRoot string, warn func(string)) ([]ReportedBugContext, error) {
	entries, err := os.ReadDir(bugsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read reported bug directory %q: %w", bugsRoot, err)
	}

	contexts := make([]ReportedBugContext, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		epicID := entry.Name()
		epicDir := filepath.Join(bugsRoot, epicID)
		files, err := os.ReadDir(epicDir)
		if err != nil {
			return nil, fmt.Errorf("read reported bug epic directory %q: %w", epicDir, err)
		}

		epicStatus, err := loadReportedBugEpicStatus(projectRoot, epicID)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".md" {
				continue
			}

			path := filepath.Join(epicDir, file.Name())
			ctx, err := loadReportedBugFile(path, epicID, epicStatus, projectRoot, warn)
			if err != nil {
				return nil, err
			}
			if ctx == nil {
				continue
			}
			contexts = append(contexts, *ctx)
		}
	}
	return contexts, nil
}

func (b ReportedBugContext) PlanningBullet() string {
	parts := []string{
		fmt.Sprintf("`%s` from epic `%s`", b.BugID, b.SourceEpicID),
		fmt.Sprintf("status `%s`", b.Status),
		fmt.Sprintf("severity `%s`", b.Severity),
	}
	if b.Summary != "" {
		parts = append(parts, fmt.Sprintf("summary: %s", b.Summary))
	}
	if b.EpicStatus != nil {
		parts = append(parts, fmt.Sprintf("source epic lifecycle `%s`", *b.EpicStatus))
	} else {
		parts = append(parts, "source epic lifecycle `not tracked in backlog metadata`")
	}
	parts = append(parts, b.PlanningAction)
	parts = append(parts, fmt.Sprintf("report: `%s`", filepath.ToSlash(b.SourcePath)))
	return strings.Join(parts, "; ")
}

func loadReportedBugEpicStatus(projectRoot, epicID string) (*types.EpicLifecycleStatus, error) {
	metadata, err := LoadEpicMetadata(NewEpicPackagePaths(projectRoot, epicID).MetadataPath)
	if err != nil {
		if err == state.ErrNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("load backlog metadata for reported bug epic %q: %w", epicID, err)
	}
	return &metadata.Status, nil
}

func loadReportedBugFile(path, epicID string, epicStatus *types.EpicLifecycleStatus, projectRoot string, warn func(string)) (*ReportedBugContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read reported bug file %q: %w", path, err)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	frontmatter, fmErr := extractFrontmatter(content)
	if fmErr != nil {
		if warn != nil {
			warn(fmt.Sprintf("skipping malformed reported bug file %q: %v", path, fmErr))
		}
		return nil, nil
	}

	var raw reportedBugFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &raw); err != nil {
		if warn != nil {
			warn(fmt.Sprintf("skipping malformed reported bug file %q: invalid YAML frontmatter: %v", path, err))
		}
		return nil, nil
	}

	status := strings.TrimSpace(strings.ToLower(raw.Status))
	if status == "" {
		if warn != nil {
			warn(fmt.Sprintf("skipping malformed reported bug file %q: missing required field %q", path, "status"))
		}
		return nil, nil
	}
	if reportedBugTerminalStatuses[status] {
		return nil, nil
	}

	bugID := strings.TrimSpace(raw.BugID)
	if bugID == "" {
		if warn != nil {
			warn(fmt.Sprintf("skipping malformed reported bug file %q: missing required field %q", path, "bug_id"))
		}
		return nil, nil
	}

	severity := strings.TrimSpace(strings.ToLower(raw.Severity))
	if severity == "" {
		if warn != nil {
			warn(fmt.Sprintf("skipping malformed reported bug file %q: missing required field %q", path, "severity"))
		}
		return nil, nil
	}

	relPath, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return nil, fmt.Errorf("compute relative reported bug path %q: %w", path, err)
	}

	return &ReportedBugContext{
		BugID:          bugID,
		SourceEpicID:   epicID,
		SourcePath:     relPath,
		Status:         status,
		Severity:       severity,
		Summary:        extractReportedBugSummary(content),
		EpicStatus:     epicStatus,
		PlanningAction: reportedBugPlanningAction(epicStatus),
	}, nil
}

func extractFrontmatter(content string) (string, error) {
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			start = i
			break
		}
	}
	if start == -1 {
		return "", fmt.Errorf("missing YAML frontmatter")
	}

	end := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return "", fmt.Errorf("missing YAML frontmatter terminator")
	}
	return strings.Join(lines[start+1:end], "\n"), nil
}

func extractReportedBugSummary(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "## Summary" {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "## ") {
				return ""
			}
			return trimmed
		}
	}
	return ""
}

func reportedBugPlanningAction(epicStatus *types.EpicLifecycleStatus) string {
	if epicStatus == nil {
		return "backlog metadata is not available; turn this into explicit new or updated `PLANNED` work during this planning cycle"
	}

	switch *epicStatus {
	case types.EpicStatusPlanned:
		return "update the existing `PLANNED` backlog work if the bug still fits that scope, or create a new `PLANNED` follow-up if it does not"
	case types.EpicStatusActive:
		return "treat follow-up as new planning work; do not reopen or mutate the `ACTIVE` backlog package"
	case types.EpicStatusCompleted:
		return "treat follow-up as new planning work; do not reopen the `COMPLETED` historical package"
	default:
		return "turn this into explicit new or updated `PLANNED` work during this planning cycle"
	}
}
