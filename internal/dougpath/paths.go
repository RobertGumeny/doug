package dougpath

import (
	"fmt"
	"path/filepath"
)

const (
	DougDirName        = ".doug"
	IntakeDirName      = "intake"
	LogsDirName        = "logs"
	EpicsDirName       = "epics"
	BugsDirName        = "bugs"
	ResearchDirName    = "research"
	AttemptStartFile   = "attempt-start.json"
	SessionArchiveFile = "session.md"
	StatsFile          = "stats.json"
	InfraFailureFile   = "infra-failure.md"
)

// Paths centralizes Doug's durable storage contract under a project root.
// Production writers may still use legacy locations during the EPIC-53
// transition, but new-layout callers should derive paths from this type rather
// than reconstructing .doug/intake or .doug/logs/epics paths ad hoc.
type Paths struct {
	ProjectRoot string
	DougDir     string
	IntakeDir   string
	LogsDir     string
	EpicsDir    string
}

// New returns path helpers rooted at projectRoot.
func New(projectRoot string) Paths {
	dougDir := filepath.Join(projectRoot, DougDirName)
	logsDir := filepath.Join(dougDir, LogsDirName)
	return Paths{
		ProjectRoot: projectRoot,
		DougDir:     dougDir,
		IntakeDir:   filepath.Join(dougDir, IntakeDirName),
		LogsDir:     logsDir,
		EpicsDir:    filepath.Join(logsDir, EpicsDirName),
	}
}

func (p Paths) IntakeBugsDir() string {
	return filepath.Join(p.IntakeDir, BugsDirName)
}

func (p Paths) IntakeBugDir(epicID string) string {
	return filepath.Join(p.IntakeBugsDir(), epicID)
}

func (p Paths) IntakeBugPath(epicID, taskID string) string {
	return filepath.Join(p.IntakeBugDir(epicID), fmt.Sprintf("bug-%s.md", taskID))
}

func (p Paths) IntakeResearchDir() string {
	return filepath.Join(p.IntakeDir, ResearchDirName)
}

func (p Paths) IntakeResearchPath(filename string) string {
	return filepath.Join(p.IntakeResearchDir(), filename)
}

func (p Paths) EpicDir(epicID string) string {
	return filepath.Join(p.EpicsDir, epicID)
}

func (p Paths) EpicPRDPath(epicID string) string {
	return filepath.Join(p.EpicDir(epicID), "PRD.md")
}

func (p Paths) EpicTasksPath(epicID string) string {
	return filepath.Join(p.EpicDir(epicID), "tasks.yaml")
}

func (p Paths) EpicProjectStatePath(epicID string) string {
	return filepath.Join(p.EpicDir(epicID), "project-state.yaml")
}

func (p Paths) TaskAttemptDir(epicID, taskID string, attempt int) string {
	return filepath.Join(p.EpicDir(epicID), taskID, attemptDir(attempt))
}

func (p Paths) SessionArchivePath(epicID, taskID string, attempt int) string {
	return filepath.Join(p.TaskAttemptDir(epicID, taskID, attempt), SessionArchiveFile)
}

func (p Paths) StatsPath(epicID, taskID string, attempt int) string {
	return filepath.Join(p.TaskAttemptDir(epicID, taskID, attempt), StatsFile)
}

// PiSessionDir is Doug's stable retained session directory. Pi may create its
// native timestamped JSONL transcript files inside this directory; Doug does
// not require or perform transcript renames.
func (p Paths) PiSessionDir(epicID, taskID string, attempt int) string {
	return p.TaskAttemptDir(epicID, taskID, attempt)
}

func (p Paths) PiNativeSessionPath(epicID, taskID string, attempt int, filename string) string {
	return filepath.Join(p.PiSessionDir(epicID, taskID, attempt), filename)
}

func (p Paths) AttemptStartPath(epicID, taskID string, attempt int) string {
	return filepath.Join(p.TaskAttemptDir(epicID, taskID, attempt), AttemptStartFile)
}

func (p Paths) TransportFailurePath(epicID, taskID string, attempt int) string {
	return filepath.Join(p.TaskAttemptDir(epicID, taskID, attempt), InfraFailureFile)
}

func (p Paths) ReviewArtifactPath(epicID string, version int) string {
	name := "epic-review.md"
	if version > 1 {
		name = fmt.Sprintf("epic-review-v%d.md", version)
	}
	return filepath.Join(p.EpicDir(epicID), name)
}

func attemptDir(attempt int) string {
	return fmt.Sprintf("attempt-%d", attempt)
}
