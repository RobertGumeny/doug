package config_test

import (
	"github.com/robertgumeny/doug/internal/testutil"
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
	defaults := config.DefaultCommandSet()
	if cfg.RunAgentCommand != defaults.Run {
		t.Errorf("RunAgentCommand = %q, want %q", cfg.RunAgentCommand, defaults.Run)
	}
	if cfg.PlanAgentCommand != defaults.Plan {
		t.Errorf("PlanAgentCommand = %q, want %q", cfg.PlanAgentCommand, defaults.Plan)
	}
	if cfg.ScaffoldAgentCommand != defaults.Scaffold {
		t.Errorf("ScaffoldAgentCommand = %q, want %q", cfg.ScaffoldAgentCommand, defaults.Scaffold)
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
	if cfg.AgentHeartbeatSeconds != config.DefaultAgentHeartbeat {
		t.Errorf("AgentHeartbeatSeconds = %d, want %d", cfg.AgentHeartbeatSeconds, config.DefaultAgentHeartbeat)
	}
}

func TestLoadConfig_PartialFile(t *testing.T) {
	defaults := config.DefaultCommandSet()
	tests := []struct {
		name          string
		yaml          string
		wantRun       string
		wantPlan      string
		wantScaffold  string
		wantBuild     string
		wantRetries   int
		wantIter      int
		wantKBEnabled bool
		wantHeartbeat int
	}{
		{
			name:          "only agent_command set",
			yaml:          "agent_command: my-agent\n",
			wantRun:       "my-agent",
			wantPlan:      "my-agent",
			wantScaffold:  "my-agent",
			wantBuild:     config.DefaultBuildSystem,
			wantRetries:   config.DefaultMaxRetries,
			wantIter:      config.DefaultMaxIterations,
			wantKBEnabled: config.DefaultKBEnabled,
			wantHeartbeat: config.DefaultAgentHeartbeat,
		},
		{
			name:          "max_retries and max_iterations overridden",
			yaml:          "max_retries: 3\nmax_iterations: 10\n",
			wantRun:       defaults.Run,
			wantPlan:      defaults.Plan,
			wantScaffold:  defaults.Scaffold,
			wantBuild:     config.DefaultBuildSystem,
			wantRetries:   3,
			wantIter:      10,
			wantKBEnabled: config.DefaultKBEnabled,
			wantHeartbeat: config.DefaultAgentHeartbeat,
		},
		{
			name:          "kb_enabled explicitly set to false",
			yaml:          "kb_enabled: false\n",
			wantRun:       defaults.Run,
			wantPlan:      defaults.Plan,
			wantScaffold:  defaults.Scaffold,
			wantBuild:     config.DefaultBuildSystem,
			wantRetries:   config.DefaultMaxRetries,
			wantIter:      config.DefaultMaxIterations,
			wantKBEnabled: false,
			wantHeartbeat: config.DefaultAgentHeartbeat,
		},
		{
			name:          "build_system set to npm",
			yaml:          "build_system: npm\n",
			wantRun:       defaults.Run,
			wantPlan:      defaults.Plan,
			wantScaffold:  defaults.Scaffold,
			wantBuild:     "npm",
			wantRetries:   config.DefaultMaxRetries,
			wantIter:      config.DefaultMaxIterations,
			wantKBEnabled: config.DefaultKBEnabled,
			wantHeartbeat: config.DefaultAgentHeartbeat,
		},
		{
			name:          "agent heartbeat overridden",
			yaml:          "agent_heartbeat_seconds: 0\n",
			wantRun:       defaults.Run,
			wantPlan:      defaults.Plan,
			wantScaffold:  defaults.Scaffold,
			wantBuild:     config.DefaultBuildSystem,
			wantRetries:   config.DefaultMaxRetries,
			wantIter:      config.DefaultMaxIterations,
			wantKBEnabled: config.DefaultKBEnabled,
			wantHeartbeat: 0,
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
			if cfg.RunAgentCommand != tt.wantRun {
				t.Errorf("RunAgentCommand = %q, want %q", cfg.RunAgentCommand, tt.wantRun)
			}
			if cfg.PlanAgentCommand != tt.wantPlan {
				t.Errorf("PlanAgentCommand = %q, want %q", cfg.PlanAgentCommand, tt.wantPlan)
			}
			if cfg.ScaffoldAgentCommand != tt.wantScaffold {
				t.Errorf("ScaffoldAgentCommand = %q, want %q", cfg.ScaffoldAgentCommand, tt.wantScaffold)
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
			if cfg.AgentHeartbeatSeconds != tt.wantHeartbeat {
				t.Errorf("AgentHeartbeatSeconds = %d, want %d", cfg.AgentHeartbeatSeconds, tt.wantHeartbeat)
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
	yaml := "run_agent_command: file-agent\nmax_retries: 3\n"
	path := filepath.Join(dir, "doug.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file values loaded.
	if cfg.RunAgentCommand != "file-agent" {
		t.Errorf("before override: RunAgentCommand = %q, want file-agent", cfg.RunAgentCommand)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("before override: MaxRetries = %d, want 3", cfg.MaxRetries)
	}

	// Simulate cobra flag override (highest precedence).
	cfg.RunAgentCommand = "flag-agent"
	cfg.MaxRetries = 10

	if cfg.RunAgentCommand != "flag-agent" {
		t.Errorf("after override: RunAgentCommand = %q, want flag-agent", cfg.RunAgentCommand)
	}
	if cfg.MaxRetries != 10 {
		t.Errorf("after override: MaxRetries = %d, want 10", cfg.MaxRetries)
	}
	// Unset fields retain defaults.
	if cfg.MaxIterations != config.DefaultMaxIterations {
		t.Errorf("MaxIterations = %d, want %d", cfg.MaxIterations, config.DefaultMaxIterations)
	}
	if cfg.AgentHeartbeatSeconds != config.DefaultAgentHeartbeat {
		t.Errorf("AgentHeartbeatSeconds = %d, want %d", cfg.AgentHeartbeatSeconds, config.DefaultAgentHeartbeat)
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
				testutil.WriteFile(t, filepath.Join(dir, "go.mod"), "module foo\n")
			},
			expected: "go",
		},
		{
			name: "pnpm-workspace.yaml exists returns pnpm",
			setup: func(dir string) {
				testutil.WriteFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "packages:\n  - packages/*\n")
			},
			expected: "pnpm",
		},
		{
			name: "package.json exists returns npm",
			setup: func(dir string) {
				testutil.WriteFile(t, filepath.Join(dir, "package.json"), "{}\n")
			},
			expected: "npm",
		},
		{
			name: "pnpm-workspace.yaml takes precedence over package.json",
			setup: func(dir string) {
				testutil.WriteFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "packages:\n  - packages/*\n")
				testutil.WriteFile(t, filepath.Join(dir, "package.json"), "{}\n")
			},
			expected: "pnpm",
		},
		{
			name: "go.mod takes precedence over pnpm-workspace.yaml",
			setup: func(dir string) {
				testutil.WriteFile(t, filepath.Join(dir, "go.mod"), "module foo\n")
				testutil.WriteFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "packages:\n  - packages/*\n")
			},
			expected: "go",
		},
		{
			name: "both exist go takes precedence",
			setup: func(dir string) {
				testutil.WriteFile(t, filepath.Join(dir, "go.mod"), "module foo\n")
				testutil.WriteFile(t, filepath.Join(dir, "package.json"), "{}\n")
			},
			expected: "go",
		},
		{
			name: "index.html exists returns static",
			setup: func(dir string) {
				testutil.WriteFile(t, filepath.Join(dir, "index.html"), "<!DOCTYPE html>\n")
			},
			expected: "static",
		},
		{
			name: "index.html takes lower precedence than package.json",
			setup: func(dir string) {
				testutil.WriteFile(t, filepath.Join(dir, "package.json"), "{}\n")
				testutil.WriteFile(t, filepath.Join(dir, "index.html"), "<!DOCTYPE html>\n")
			},
			expected: "npm",
		},
		{
			name:     "neither exists returns empty string",
			setup:    func(dir string) {},
			expected: "",
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

// ---------------------------------------------------------------------------
// ResolveManifestBuildSystem tests
// ---------------------------------------------------------------------------

func TestResolveManifestBuildSystem(t *testing.T) {
	tests := []struct {
		name           string
		buildSystem    string
		packageManager string
		runtime        string
		want           string
	}{
		{
			name:        "known build system go passes through",
			buildSystem: "go",
			want:        "go",
		},
		{
			name:        "known build system npm passes through",
			buildSystem: "npm",
			want:        "npm",
		},
		{
			name:        "known build system pnpm passes through",
			buildSystem: "pnpm",
			want:        "pnpm",
		},
		{
			name:        "known build system static passes through",
			buildSystem: "static",
			want:        "static",
		},
		{
			name:           "npm-scripts with pnpm package manager resolves to pnpm",
			buildSystem:    "npm-scripts",
			packageManager: "pnpm",
			want:           "pnpm",
		},
		{
			name:           "npm-scripts with npm package manager resolves to npm",
			buildSystem:    "npm-scripts",
			packageManager: "npm",
			want:           "npm",
		},
		{
			name:    "unknown build system node runtime falls back to npm",
			runtime: "node",
			want:    "npm",
		},
		{
			name:    "unknown build system go runtime falls back to go",
			runtime: "go",
			want:    "go",
		},
		{
			name: "all unknown inputs returns default",
			want: config.DefaultBuildSystem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.ResolveManifestBuildSystem(tt.buildSystem, tt.packageManager, tt.runtime)
			if got != tt.want {
				t.Errorf("ResolveManifestBuildSystem(%q, %q, %q) = %q, want %q",
					tt.buildSystem, tt.packageManager, tt.runtime, got, tt.want)
			}
		})
	}
}
