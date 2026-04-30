package orchestrator

import (
	"path/filepath"

	"github.com/robertgumeny/doug/internal/config"
)

// Paths consolidates all .doug/ directory and file path values that are
// derived from the project root. Constructing them once in NewPaths avoids
// scattered filepath.Join calls throughout the orchestration loop.
type Paths struct {
	ProjectRoot      string // absolute path to the project root
	DougDir          string // <root>/.doug
	ConfigPath       string // <root>/.doug/doug.yaml
	StatePath        string // <root>/.doug/project-state.yaml
	TasksPath        string // <root>/.doug/tasks.yaml
	ManifestPath     string // <root>/.doug/plan/manifest.yaml
	LogsDir          string // <root>/.doug/logs
	ChangelogPath    string // <root>/CHANGELOG.md
	// Deprecated: SkillsConfigPath is the legacy path for skills-config.yaml. Skill
	// selection is now resolved by PolicyConfig.ResolveSkill from .doug/doug.yaml.
	// Remove this field together with DefaultSkillsConfigPath and the skillsConfigPath
	// parameter in PrepareExecution during final rollout.
	SkillsConfigPath string // <root>/.doug/skills-config.yaml
}

// NewPaths derives all orchestrator paths from the given project root.
func NewPaths(projectRoot string) Paths {
	dougDir := filepath.Join(projectRoot, ".doug")
	return Paths{
		ProjectRoot:      projectRoot,
		DougDir:          dougDir,
		ConfigPath:       filepath.Join(dougDir, "doug.yaml"),
		StatePath:        filepath.Join(dougDir, "project-state.yaml"),
		TasksPath:        filepath.Join(dougDir, "tasks.yaml"),
		ManifestPath:     filepath.Join(dougDir, "plan", "manifest.yaml"),
		LogsDir:          filepath.Join(dougDir, "logs"),
		ChangelogPath:    filepath.Join(projectRoot, "CHANGELOG.md"),
		SkillsConfigPath: filepath.Join(projectRoot, config.DefaultSkillsConfigPath),
	}
}
