// Package config provides OrchestratorConfig loading and build system detection.
// Config is read from doug.yaml in the project root. A missing file returns
// sane defaults without error. CLI flags (bound via cobra) override config file
// values at the highest precedence by mutating the returned struct after loading.
package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Default values for OrchestratorConfig fields.
const (
	DefaultAgentCommand  = "claude"
	DefaultBuildSystem   = "go"
	DefaultMaxRetries    = 5
	DefaultMaxIterations = 20
	DefaultKBEnabled     = true
)

// OrchestratorConfig holds all configuration for the doug orchestrator.
// It is read from doug.yaml in the project root. CLI flags override it at the
// highest precedence by being applied after LoadConfig returns.
type OrchestratorConfig struct {
	AgentCommand  string `yaml:"agent_command"`
	BuildSystem   string `yaml:"build_system"`
	MaxRetries    int    `yaml:"max_retries"`
	MaxIterations int    `yaml:"max_iterations"`
	KBEnabled     bool   `yaml:"kb_enabled"`
}

// defaults returns an OrchestratorConfig populated with sane defaults.
func defaults() OrchestratorConfig {
	return OrchestratorConfig{
		AgentCommand:  DefaultAgentCommand,
		BuildSystem:   DefaultBuildSystem,
		MaxRetries:    DefaultMaxRetries,
		MaxIterations: DefaultMaxIterations,
		KBEnabled:     DefaultKBEnabled,
	}
}

// partialConfig is used during YAML parsing to distinguish between a field
// being absent (nil pointer) and a field being explicitly set to its zero value.
type partialConfig struct {
	AgentCommand  *string `yaml:"agent_command"`
	BuildSystem   *string `yaml:"build_system"`
	MaxRetries    *int    `yaml:"max_retries"`
	MaxIterations *int    `yaml:"max_iterations"`
	KBEnabled     *bool   `yaml:"kb_enabled"`
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
		return nil, err
	}

	var partial partialConfig
	if err := yaml.Unmarshal(data, &partial); err != nil {
		return nil, err
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

	return &cfg, nil
}

// DetectBuildSystem returns the build system identifier based on marker files
// found in dir. Rules (highest precedence first):
//   - "go"  if go.mod exists
//   - "npm" if package.json exists (and go.mod does not)
//   - "go"  if neither file exists (safe default)
func DetectBuildSystem(dir string) string {
	_, goModErr := os.Stat(filepath.Join(dir, "go.mod"))
	if goModErr == nil {
		return "go"
	}

	_, pkgJSONErr := os.Stat(filepath.Join(dir, "package.json"))
	if pkgJSONErr == nil {
		return "npm"
	}

	return "go"
}
