package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/robertgumeny/doug/internal/config"
)

// ---------------------------------------------------------------------------
// LoadConfig tests
// ---------------------------------------------------------------------------

func TestLoadConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.LoadConfig(filepath.Join(dir, "doug.yaml"))
	if err != nil {
		t.Fatalf("expected no error for missing config file, got %v", err)
	}
	if cfg.AgentCommand != config.DefaultAgentCommand {
		t.Errorf("AgentCommand = %q, want %q", cfg.AgentCommand, config.DefaultAgentCommand)
	}
	if cfg.BuildSystem != config.DefaultBuildSystem {
		t.Errorf("BuildSystem = %q, want %q", cfg.BuildSystem, config.DefaultBuildSystem)
	}
	if cfg.MaxRetries != config.DefaultMaxRetries {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, config.DefaultMaxRetries)
	}
	if cfg.MaxIterations != config.DefaultMaxIterations {
		t.Errorf("MaxIterations = %d, want %d", cfg.MaxIterations, config.DefaultMaxIterations)
	}
	if cfg.KBEnabled != config.DefaultKBEnabled {
		t.Errorf("KBEnabled = %v, want %v", cfg.KBEnabled, config.DefaultKBEnabled)
	}
}

func TestLoadConfig_PartialFile(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		wantAgent    string
		wantBuild    string
		wantRetries  int
		wantIter     int
		wantKBEnabled bool
	}{
		{
			name:         "only agent_command set",
			yaml:         "agent_command: my-agent\n",
			wantAgent:    "my-agent",
			wantBuild:    config.DefaultBuildSystem,
			wantRetries:  config.DefaultMaxRetries,
			wantIter:     config.DefaultMaxIterations,
			wantKBEnabled: config.DefaultKBEnabled,
		},
		{
			name:         "max_retries and max_iterations overridden",
			yaml:         "max_retries: 3\nmax_iterations: 10\n",
			wantAgent:    config.DefaultAgentCommand,
			wantBuild:    config.DefaultBuildSystem,
			wantRetries:  3,
			wantIter:     10,
			wantKBEnabled: config.DefaultKBEnabled,
		},
		{
			name:         "kb_enabled explicitly set to false",
			yaml:         "kb_enabled: false\n",
			wantAgent:    config.DefaultAgentCommand,
			wantBuild:    config.DefaultBuildSystem,
			wantRetries:  config.DefaultMaxRetries,
			wantIter:     config.DefaultMaxIterations,
			wantKBEnabled: false,
		},
		{
			name:         "build_system set to npm",
			yaml:         "build_system: npm\n",
			wantAgent:    config.DefaultAgentCommand,
			wantBuild:    "npm",
			wantRetries:  config.DefaultMaxRetries,
			wantIter:     config.DefaultMaxIterations,
			wantKBEnabled: config.DefaultKBEnabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "doug.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}

			cfg, err := config.LoadConfig(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.AgentCommand != tt.wantAgent {
				t.Errorf("AgentCommand = %q, want %q", cfg.AgentCommand, tt.wantAgent)
			}
			if cfg.BuildSystem != tt.wantBuild {
				t.Errorf("BuildSystem = %q, want %q", cfg.BuildSystem, tt.wantBuild)
			}
			if cfg.MaxRetries != tt.wantRetries {
				t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, tt.wantRetries)
			}
			if cfg.MaxIterations != tt.wantIter {
				t.Errorf("MaxIterations = %d, want %d", cfg.MaxIterations, tt.wantIter)
			}
			if cfg.KBEnabled != tt.wantKBEnabled {
				t.Errorf("KBEnabled = %v, want %v", cfg.KBEnabled, tt.wantKBEnabled)
			}
		})
	}
}

// TestLoadConfig_CLIFlagOverride demonstrates the CLI flag override pattern.
// Cobra binds flags to a *OrchestratorConfig and sets field values after
// LoadConfig returns, giving CLI flags the highest precedence.
func TestLoadConfig_CLIFlagOverride(t *testing.T) {
	dir := t.TempDir()
	// Config file sets agent_command and max_retries.
	yaml := "agent_command: file-agent\nmax_retries: 3\n"
	path := filepath.Join(dir, "doug.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file values loaded.
	if cfg.AgentCommand != "file-agent" {
		t.Errorf("before override: AgentCommand = %q, want file-agent", cfg.AgentCommand)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("before override: MaxRetries = %d, want 3", cfg.MaxRetries)
	}

	// Simulate cobra flag override (highest precedence).
	cfg.AgentCommand = "flag-agent"
	cfg.MaxRetries = 10

	if cfg.AgentCommand != "flag-agent" {
		t.Errorf("after override: AgentCommand = %q, want flag-agent", cfg.AgentCommand)
	}
	if cfg.MaxRetries != 10 {
		t.Errorf("after override: MaxRetries = %d, want 10", cfg.MaxRetries)
	}
	// Unset fields retain defaults.
	if cfg.MaxIterations != config.DefaultMaxIterations {
		t.Errorf("MaxIterations = %d, want %d", cfg.MaxIterations, config.DefaultMaxIterations)
	}
}

// ---------------------------------------------------------------------------
// DetectBuildSystem tests
// ---------------------------------------------------------------------------

func TestDetectBuildSystem(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string)
		expected string
	}{
		{
			name: "go.mod exists returns go",
			setup: func(dir string) {
				writeFile(t, filepath.Join(dir, "go.mod"), "module foo\n")
			},
			expected: "go",
		},
		{
			name: "package.json exists returns npm",
			setup: func(dir string) {
				writeFile(t, filepath.Join(dir, "package.json"), "{}\n")
			},
			expected: "npm",
		},
		{
			name: "both exist go takes precedence",
			setup: func(dir string) {
				writeFile(t, filepath.Join(dir, "go.mod"), "module foo\n")
				writeFile(t, filepath.Join(dir, "package.json"), "{}\n")
			},
			expected: "go",
		},
		{
			name:     "neither exists returns go default",
			setup:    func(dir string) {},
			expected: "go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)
			if got := config.DetectBuildSystem(dir); got != tt.expected {
				t.Errorf("DetectBuildSystem() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// writeFile is a test helper that creates a file with given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
