// Package config provides OrchestratorConfig loading, build system detection,
// and the BuildSystems registry. Config is read from doug.yaml in the project
// root. A missing file returns sane defaults without error. CLI flags (bound
// via cobra) override config file values at the highest precedence by mutating
// the returned struct after loading.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Default values for OrchestratorConfig fields.
const (
	DefaultAgentCommand     = "claude"
	DefaultBuildSystem      = "go"
	DefaultMaxRetries       = 5
	DefaultMaxIterations    = 20
	DefaultKBEnabled        = true
	DefaultAgentHeartbeat   = 30
	DefaultSkillsConfigPath = ".doug/skills-config.yaml"
)

// OrchestratorConfig holds all configuration for the doug orchestrator.
// It is read from .doug/doug.yaml. CLI flags override it at the highest
// precedence by being applied after LoadConfig returns.
type OrchestratorConfig struct {
	AgentCommand          string `yaml:"agent_command"`
	BuildSystem           string `yaml:"build_system"`
	MaxRetries            int    `yaml:"max_retries"`
	MaxIterations         int    `yaml:"max_iterations"`
	KBEnabled             bool   `yaml:"kb_enabled"`
	AgentHeartbeatSeconds int    `yaml:"agent_heartbeat_seconds"`
}

// defaults returns an OrchestratorConfig populated with sane defaults.
func defaults() OrchestratorConfig {
	return OrchestratorConfig{
		AgentCommand:          DefaultAgentCommand,
		BuildSystem:           DefaultBuildSystem,
		MaxRetries:            DefaultMaxRetries,
		MaxIterations:         DefaultMaxIterations,
		KBEnabled:             DefaultKBEnabled,
		AgentHeartbeatSeconds: DefaultAgentHeartbeat,
	}
}

// partialConfig is used during YAML parsing to distinguish between a field
// being absent (nil pointer) and a field being explicitly set to its zero value.
type partialConfig struct {
	AgentCommand          *string `yaml:"agent_command"`
	BuildSystem           *string `yaml:"build_system"`
	MaxRetries            *int    `yaml:"max_retries"`
	MaxIterations         *int    `yaml:"max_iterations"`
	KBEnabled             *bool   `yaml:"kb_enabled"`
	AgentHeartbeatSeconds *int    `yaml:"agent_heartbeat_seconds"`
}

// LoadConfig reads doug.yaml at path and returns an OrchestratorConfig.
// If the file does not exist, defaults are returned without error.
// Fields absent from the file are filled with their default values.
// Fields present in the file override the corresponding default.
//
// CLI flag override pattern: cobra binds flags to the returned *OrchestratorConfig
// after this call, giving flags the highest precedence automatically.
func LoadConfig(path string) (*OrchestratorConfig, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &cfg, nil
		}
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var partial partialConfig
	if err := yaml.Unmarshal(data, &partial); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	if partial.AgentCommand != nil {
		cfg.AgentCommand = *partial.AgentCommand
	}
	if partial.BuildSystem != nil {
		cfg.BuildSystem = *partial.BuildSystem
	}
	if partial.MaxRetries != nil {
		cfg.MaxRetries = *partial.MaxRetries
	}
	if partial.MaxIterations != nil {
		cfg.MaxIterations = *partial.MaxIterations
	}
	if partial.KBEnabled != nil {
		cfg.KBEnabled = *partial.KBEnabled
	}
	if partial.AgentHeartbeatSeconds != nil {
		cfg.AgentHeartbeatSeconds = *partial.AgentHeartbeatSeconds
	}

	return &cfg, nil
}

// BuildSystemInfo contains all metadata doug needs for a supported build system.
// To add a new build system: add one entry to the BuildSystems map below.
type BuildSystemInfo struct {
	// Permissions is the list of Claude Code Bash permissions to allow.
	Permissions []string
	// InstallCmd is the human-readable install command shown in ACTIVE_TASK.md.
	InstallCmd string
	// VerifyCommands are shown in ACTIVE_TASK.md as verification steps.
	VerifyCommands []string
	// InitMarkers are files that indicate this build system is in use (for detection).
	InitMarkers []string
	// CommonPitfalls are injected into ACTIVE_TASK.md as agent guidance.
	CommonPitfalls []string
}

// BuildSystems is the registry of supported build systems.
// DetectBuildSystem returns keys from this map.
// Adding a new build system: add one entry here + add detection logic to
// DetectBuildSystem + update --build-system flag validation in cmd/init.go.
var BuildSystems = map[string]BuildSystemInfo{
	"go": {
		Permissions: []string{
			"Bash(go build *)", "Bash(go test *)", "Bash(go vet *)",
			"Bash(go mod tidy)", "Bash(go mod download)",
		},
		InstallCmd:     "go mod download",
		VerifyCommands: []string{"go build ./...", "go test ./...", "go vet ./..."},
		InitMarkers:    []string{"go.mod"},
		CommonPitfalls: []string{
			"Run `go mod tidy` after adding or removing imports",
			"Do not use `sh -c` or shell string concatenation in exec.Command calls",
		},
	},
	"npm": {
		Permissions: []string{
			"Bash(npm install *)", "Bash(npm run build)",
			"Bash(npm run test)", "Bash(npm run test:*)",
			"Bash(npm run lint)", "Bash(npm run lint:*)", "Bash(npm ci)",
		},
		InstallCmd:     "npm ci",
		VerifyCommands: []string{"npm run build", "npm run test", "npm run lint"},
		InitMarkers:    []string{"package.json"},
		CommonPitfalls: []string{
			"Prefer `npm ci` over `npm install` in CI for reproducible installs",
			"Check package-lock.json into version control",
		},
	},
	"pnpm": {
		Permissions: []string{
			"Bash(pnpm install *)", "Bash(pnpm run build)",
			"Bash(pnpm run test)", "Bash(pnpm run test:*)",
			"Bash(pnpm run lint)", "Bash(pnpm run lint:*)", "Bash(pnpm add *)",
		},
		InstallCmd:     "pnpm install",
		VerifyCommands: []string{"pnpm run build", "pnpm run test", "pnpm run lint"},
		InitMarkers:    []string{"pnpm-workspace.yaml", "package.json"},
		CommonPitfalls: []string{
			"Use `pnpm add` not `npm install` to add packages",
			"Check pnpm-lock.yaml into version control",
		},
	},
	"static": {
		Permissions:    []string{},
		InstallCmd:     "",
		VerifyCommands: []string{},
		InitMarkers:    []string{"index.html"},
		CommonPitfalls: []string{},
	},
}

// ResolveManifestBuildSystem maps manifest scaffold fields to an internal build
// system identifier (a key in BuildSystems). Resolution order:
//  1. buildSystem is already a known key in BuildSystems → return it directly
//  2. packageManager is a known key in BuildSystems → return it
//  3. runtime "node" → "npm", runtime "go" → "go"
//  4. DefaultBuildSystem
func ResolveManifestBuildSystem(buildSystem, packageManager, runtime string) string {
	if _, ok := BuildSystems[buildSystem]; ok {
		return buildSystem
	}
	if _, ok := BuildSystems[packageManager]; ok {
		return packageManager
	}
	switch runtime {
	case "node":
		return "npm"
	case "go":
		return "go"
	default:
		return DefaultBuildSystem
	}
}

// DetectBuildSystem returns the build system identifier based on marker files
// found in dir. Rules (highest precedence first):
//   - "go"     if go.mod exists
//   - "pnpm"   if pnpm-workspace.yaml exists (and go.mod does not)
//   - "npm"    if package.json exists (and neither go.mod nor pnpm-workspace.yaml)
//   - "static" if index.html exists (lowest priority; no compiled assets)
//   - ""       if no marker file exists (caller decides the fallback)
func DetectBuildSystem(dir string) string {
	_, goModErr := os.Stat(filepath.Join(dir, "go.mod"))
	if goModErr == nil {
		return "go"
	}

	_, pnpmWorkspaceErr := os.Stat(filepath.Join(dir, "pnpm-workspace.yaml"))
	if pnpmWorkspaceErr == nil {
		return "pnpm"
	}

	_, pkgJSONErr := os.Stat(filepath.Join(dir, "package.json"))
	if pkgJSONErr == nil {
		return "npm"
	}

	_, indexHTMLErr := os.Stat(filepath.Join(dir, "index.html"))
	if indexHTMLErr == nil {
		return "static"
	}

	return ""
}
