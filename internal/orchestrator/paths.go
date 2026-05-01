package orchestrator

import (
	"path/filepath"
)

// Paths consolidates all .doug/ directory and file path values that are
// derived from the project root. Constructing them once in NewPaths avoids
// scattered filepath.Join calls throughout the orchestration loop.
type Paths struct {
	ProjectRoot   string // absolute path to the project root
	DougDir       string // <root>/.doug
	ConfigPath    string // <root>/.doug/doug.yaml
	StatePath     string // <root>/.doug/project-state.yaml
	TasksPath     string // <root>/.doug/tasks.yaml
	ManifestPath  string // <root>/.doug/plan/manifest.yaml
	LogsDir       string // <root>/.doug/logs
	ChangelogPath string // <root>/CHANGELOG.md
}

// NewPaths derives all orchestrator paths from the given project root.
func NewPaths(projectRoot string) Paths {
	dougDir := filepath.Join(projectRoot, ".doug")
	return Paths{
		ProjectRoot:   projectRoot,
		DougDir:       dougDir,
		ConfigPath:    filepath.Join(dougDir, "doug.yaml"),
		StatePath:     filepath.Join(dougDir, "project-state.yaml"),
		TasksPath:     filepath.Join(dougDir, "tasks.yaml"),
		ManifestPath:  filepath.Join(dougDir, "plan", "manifest.yaml"),
		LogsDir:       filepath.Join(dougDir, "logs"),
		ChangelogPath: filepath.Join(projectRoot, "CHANGELOG.md"),
	}
}
