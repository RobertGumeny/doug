package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/testutil"
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
	if cfg.LintEnabled != config.DefaultLintEnabled {
		t.Errorf("LintEnabled = %v, want %v", cfg.LintEnabled, config.DefaultLintEnabled)
	}
	if cfg.LintCommand != "" {
		t.Errorf("LintCommand = %q, want empty string", cfg.LintCommand)
	}
}

func TestLoadConfig_PartialFile(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		wantBuild     string
		wantRetries   int
		wantIter      int
		wantKBEnabled bool
		wantHeartbeat int
	}{
		{
			name:          "max_retries and max_iterations overridden",
			yaml:          "max_retries: 3\nmax_iterations: 10\n",
			wantBuild:     config.DefaultBuildSystem,
			wantRetries:   3,
			wantIter:      10,
			wantKBEnabled: config.DefaultKBEnabled,
			wantHeartbeat: config.DefaultAgentHeartbeat,
		},
		{
			name:          "kb_enabled explicitly set to false",
			yaml:          "kb_enabled: false\n",
			wantBuild:     config.DefaultBuildSystem,
			wantRetries:   config.DefaultMaxRetries,
			wantIter:      config.DefaultMaxIterations,
			wantKBEnabled: false,
			wantHeartbeat: config.DefaultAgentHeartbeat,
		},
		{
			name:          "build_system set to npm",
			yaml:          "build_system: npm\n",
			wantBuild:     "npm",
			wantRetries:   config.DefaultMaxRetries,
			wantIter:      config.DefaultMaxIterations,
			wantKBEnabled: config.DefaultKBEnabled,
			wantHeartbeat: config.DefaultAgentHeartbeat,
		},
		{
			name:          "agent heartbeat overridden",
			yaml:          "agent_heartbeat_seconds: 0\n",
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
	yaml := "max_retries: 3\n"
	path := filepath.Join(dir, "doug.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file value loaded.
	if cfg.MaxRetries != 3 {
		t.Errorf("before override: MaxRetries = %d, want 3", cfg.MaxRetries)
	}

	// Simulate cobra flag override (highest precedence).
	cfg.MaxRetries = 10

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
// Policy block loading tests
// ---------------------------------------------------------------------------

func TestLoadConfig_RejectsStaleExecutionModePolicyField(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "top-level stale field",
			yaml: "execution_mode: rpc\n",
		},
		{
			name: "phase stale field",
			yaml: "policy:\n  phases:\n    runtime:\n      execution_mode: rpc\n",
		},
		{
			name: "task stale field",
			yaml: "policy:\n  tasks:\n    feature:\n      execution_mode: rpc\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "doug.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o644); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := config.LoadConfig(path)
			if err == nil {
				t.Fatal("expected stale execution_mode config to be rejected")
			}
			if !strings.Contains(err.Error(), "execution_mode") || !strings.Contains(err.Error(), "interaction_mode") {
				t.Fatalf("error %q does not clearly mention stale execution_mode and interaction_mode", err.Error())
			}
		})
	}
}

func TestLoadConfig_StalePlanningExecutionModeMentionsInteractiveMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doug.yaml")
	data := []byte("policy:\n  phases:\n    planning:\n      execution_mode: rpc\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.LoadConfig(path)
	if err == nil {
		t.Fatal("expected stale planning execution_mode to be rejected")
	}
	msg := err.Error()
	for _, want := range []string{"policy.phases.planning.execution_mode", "policy.phases.planning.interaction_mode", "interactive"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not mention %q", msg, want)
		}
	}
}

func TestLoadConfig_UnsupportedPhaseInteractionModeNamesPhaseAndAcceptedModes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doug.yaml")
	data := []byte("policy:\n  phases:\n    runtime:\n      interaction_mode: docker\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.LoadConfig(path)
	if err == nil {
		t.Fatal("expected unsupported interaction_mode to be rejected")
	}
	msg := err.Error()
	for _, want := range []string{"policy.phases.runtime.interaction_mode", "docker", "interactive", "rpc"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not mention %q", msg, want)
		}
	}
}

func TestLoadConfig_PolicyBlock(t *testing.T) {
	tests := []struct {
		name           string
		yaml           string
		wantPhases     map[string]config.PhasePolicy
		wantTaskSkill  map[string]string
		wantTaskExMode map[string]string
	}{
		{
			name: "policy block absent — empty PolicyConfig",
			yaml: "max_retries: 3\n",
		},
		{
			name: "phase policy loaded",
			yaml: `
policy:
  phases:
    runtime:
      interaction_mode: rpc
      routing_profile: standard
`,
			wantPhases: map[string]config.PhasePolicy{
				"runtime": {InteractionMode: "rpc", RoutingProfile: "standard"},
			},
		},
		{
			name: "task policy skill loaded",
			yaml: `
policy:
  tasks:
    feature:
      skill: custom-feature-skill
    bugfix:
      skill: custom-bugfix-skill
`,
			wantTaskSkill: map[string]string{
				"feature": "custom-feature-skill",
				"bugfix":  "custom-bugfix-skill",
			},
		},
		{
			name: "task policy interaction mode and routing profile loaded",
			yaml: `
policy:
  tasks:
    feature:
      interaction_mode: rpc
      routing_profile: fast
`,
			wantTaskExMode: map[string]string{
				"feature": "rpc",
			},
		},
		{
			name: "full policy block with phases and tasks",
			yaml: `
policy:
  phases:
    runtime:
      interaction_mode: rpc
    planning:
      interaction_mode: interactive
  tasks:
    feature:
      skill: my-feature-skill
      interaction_mode: rpc
`,
			wantPhases: map[string]config.PhasePolicy{
				"runtime":  {InteractionMode: "rpc"},
				"planning": {InteractionMode: "interactive"},
			},
			wantTaskSkill: map[string]string{
				"feature": "my-feature-skill",
			},
			wantTaskExMode: map[string]string{
				"feature": "rpc",
			},
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

			for phase, want := range tt.wantPhases {
				got, ok := cfg.Policy.Phases[phase]
				if !ok {
					t.Errorf("phase %q missing from cfg.Policy.Phases", phase)
					continue
				}
				if got.InteractionMode != want.InteractionMode {
					t.Errorf("phase %q InteractionMode = %q, want %q", phase, got.InteractionMode, want.InteractionMode)
				}
				if got.RoutingProfile != want.RoutingProfile {
					t.Errorf("phase %q RoutingProfile = %q, want %q", phase, got.RoutingProfile, want.RoutingProfile)
				}
			}

			for taskType, wantSkill := range tt.wantTaskSkill {
				tp, ok := cfg.Policy.Tasks[taskType]
				if !ok {
					t.Errorf("task %q missing from cfg.Policy.Tasks", taskType)
					continue
				}
				if tp.Skill != wantSkill {
					t.Errorf("task %q Skill = %q, want %q", taskType, tp.Skill, wantSkill)
				}
			}

			for taskType, wantMode := range tt.wantTaskExMode {
				tp, ok := cfg.Policy.Tasks[taskType]
				if !ok {
					t.Errorf("task %q missing from cfg.Policy.Tasks", taskType)
					continue
				}
				if tp.InteractionMode != wantMode {
					t.Errorf("task %q InteractionMode = %q, want %q", taskType, tp.InteractionMode, wantMode)
				}
			}
		})
	}
}

func TestLoadConfig_PolicyAbsent_DefaultsToEmpty(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.LoadConfig(filepath.Join(dir, "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Policy.Phases) != 0 {
		t.Errorf("expected empty Phases, got %v", cfg.Policy.Phases)
	}
	if len(cfg.Policy.Tasks) != 0 {
		t.Errorf("expected empty Tasks, got %v", cfg.Policy.Tasks)
	}
}

func TestLoadConfig_PolicyResolveSkillFromConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `
policy:
  tasks:
    feature:
      skill: config-feature-skill
`
	path := filepath.Join(dir, "doug.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := cfg.Policy.ResolveSkill("feature", "implement-feature")
	if got != "config-feature-skill" {
		t.Errorf("ResolveSkill = %q, want %q", got, "config-feature-skill")
	}

	// Other task types fall back to the provided default.
	got = cfg.Policy.ResolveSkill("bugfix", "implement-bugfix")
	if got != "implement-bugfix" {
		t.Errorf("ResolveSkill(bugfix) = %q, want %q", got, "implement-bugfix")
	}
}

// ---------------------------------------------------------------------------
// Validate tests
// ---------------------------------------------------------------------------

func TestValidate_DefaultsPassValidation(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.LoadConfig(filepath.Join(dir, "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("unexpected LoadConfig error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config failed validation: %v", err)
	}
}

func TestValidate_AllKnownBuildSystemsPass(t *testing.T) {
	for _, bs := range []string{"go", "npm", "pnpm", "static"} {
		t.Run(bs, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "doug.yaml")
			testutil.WriteFile(t, path, "build_system: "+bs+"\n")
			cfg, err := config.LoadConfig(path)
			if err != nil {
				t.Fatalf("unexpected LoadConfig error: %v", err)
			}
			if err := cfg.Validate(); err != nil {
				t.Errorf("build_system %q failed validation: %v", bs, err)
			}
		})
	}
}

func TestValidate_UnknownBuildSystemFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doug.yaml")
	testutil.WriteFile(t, path, "build_system: rust\n")
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected LoadConfig error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for unknown build_system, got nil")
	}
	if !containsAll(err.Error(), "unsupported build_system", "rust") {
		t.Errorf("error message not actionable: %v", err)
	}
}

func TestValidate_NegativeMaxRetriesFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doug.yaml")
	testutil.WriteFile(t, path, "max_retries: -1\n")
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected LoadConfig error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for negative max_retries, got nil")
	}
	if !containsAll(err.Error(), "max_retries") {
		t.Errorf("error message not actionable: %v", err)
	}
}

func TestValidate_ZeroMaxRetriePasses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doug.yaml")
	testutil.WriteFile(t, path, "max_retries: 0\n")
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected LoadConfig error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("max_retries: 0 failed validation unexpectedly: %v", err)
	}
}

func TestValidate_ZeroMaxIterationsFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doug.yaml")
	testutil.WriteFile(t, path, "max_iterations: 0\n")
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected LoadConfig error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for max_iterations: 0, got nil")
	}
	if !containsAll(err.Error(), "max_iterations") {
		t.Errorf("error message not actionable: %v", err)
	}
}

func TestValidate_NegativeMaxIterationsFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doug.yaml")
	testutil.WriteFile(t, path, "max_iterations: -5\n")
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected LoadConfig error: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for negative max_iterations, got nil")
	}
}

// ---------------------------------------------------------------------------
// Regression tests — config-driven resolution behavior
// ---------------------------------------------------------------------------

// TestRegression_DefaultConfigResolution verifies that when no config file
// exists, the default build system and agent heartbeat remain stable. These
// defaults drive build-system–specific agent briefings and monitoring so any
// accidental change would silently alter runtime behavior.
func TestRegression_DefaultConfigResolution(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.LoadConfig(filepath.Join(dir, "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"BuildSystem", cfg.BuildSystem, config.DefaultBuildSystem},
		{"MaxRetries", cfg.MaxRetries, config.DefaultMaxRetries},
		{"MaxIterations", cfg.MaxIterations, config.DefaultMaxIterations},
		{"AgentHeartbeatSeconds", cfg.AgentHeartbeatSeconds, config.DefaultAgentHeartbeat},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// TestRegression_PolicySkillOverridePrecedence verifies that the resolution
// chain policy.Tasks > hardcoded default is preserved.
// This chain drives which skill file the agent loads; breaking precedence
// would silently swap skills across all projects.
func TestRegression_PolicySkillOverridePrecedence(t *testing.T) {
	// Policy override must beat the file-level default.
	dir := t.TempDir()
	yaml := `
policy:
  tasks:
    feature:
      skill: policy-feature-skill
`
	path := filepath.Join(dir, "doug.yaml")
	testutil.WriteFile(t, path, yaml)

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := cfg.Policy.ResolveSkill("feature", "hardcoded-fallback")
	if got != "policy-feature-skill" {
		t.Errorf("policy skill override not respected: got %q, want %q", got, "policy-feature-skill")
	}

	// When no policy override, fallback is returned unchanged.
	got = cfg.Policy.ResolveSkill("bugfix", "hardcoded-bugfix")
	if got != "hardcoded-bugfix" {
		t.Errorf("fallback not returned for unconfigured task type: got %q, want %q", got, "hardcoded-bugfix")
	}
}

// TestRegression_TaskOverridesPhaseInResolution verifies the override
// hierarchy: task-level non-mode policy settings override phase-level settings
// for single-value fields, interaction mode remains source-owned by phase, and
// list fields (WriteScopes, ReadPathAdditions) merge additively (phase first,
// then task).
func TestRegression_TaskOverridesPhaseInResolution(t *testing.T) {
	dir := t.TempDir()
	yaml := `
policy:
  phases:
    runtime:
      interaction_mode: interactive
      routing_profile: standard
      write_scopes:
        - /phase/path
  tasks:
    feature:
      interaction_mode: rpc
      write_scopes:
        - /task/path
`
	path := filepath.Join(dir, "doug.yaml")
	testutil.WriteFile(t, path, yaml)

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exec := cfg.Policy.ResolveExecution("runtime", "feature")

	// interaction_mode in config cannot change source-owned phase routing.
	if exec.InteractionMode != "rpc" {
		t.Errorf("InteractionMode = %q, want rpc (source-owned runtime mode)", exec.InteractionMode)
	}
	// Phase routing_profile falls through when task doesn't set it.
	if exec.RoutingProfile != "standard" {
		t.Errorf("RoutingProfile = %q, want standard (phase fallback)", exec.RoutingProfile)
	}
	// WriteScopes are merged additively: phase first, then task.
	if len(exec.WriteScopes) != 2 || exec.WriteScopes[0] != "/phase/path" || exec.WriteScopes[1] != "/task/path" {
		t.Errorf("WriteScopes = %v, want [/phase/path /task/path]", exec.WriteScopes)
	}
}

func TestPolicyConfig_RequiresPiIncludesInteractiveAndRPCOnly(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want bool
	}{
		{name: "interactive requires pi", mode: config.InteractionModeInteractive, want: true},
		{name: "rpc requires pi", mode: config.InteractionModeRPC, want: true},
		{name: "empty does not require pi until defaults are resolved", mode: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := config.PolicyConfig{
				Phases: map[string]config.PhasePolicy{
					"runtime": {InteractionMode: tt.mode},
				},
			}

			if got := policy.RequiresPi(); got != tt.want {
				t.Errorf("RequiresPi() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyConfig_RequiresRPCOnlyMatchesRPCMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want bool
	}{
		{name: "interactive is pi but not rpc", mode: config.InteractionModeInteractive, want: false},
		{name: "rpc requires rpc", mode: config.InteractionModeRPC, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := config.PolicyConfig{
				Tasks: map[string]config.TaskPolicy{
					"feature": {InteractionMode: tt.mode},
				},
			}

			if got := policy.RequiresRPC(); got != tt.want {
				t.Errorf("RequiresRPC() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Lint config tests
// ---------------------------------------------------------------------------

func TestLoadConfig_LintEnabled_LoadedFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doug.yaml")
	testutil.WriteFile(t, path, "lint_enabled: true\n")

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.LintEnabled {
		t.Errorf("LintEnabled = false, want true")
	}
}

func TestLoadConfig_LintCommand_LoadedFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doug.yaml")
	testutil.WriteFile(t, path, "lint_enabled: true\nlint_command: golangci-lint run ./...\n")

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.LintEnabled {
		t.Errorf("LintEnabled = false, want true")
	}
	if cfg.LintCommand != "golangci-lint run ./..." {
		t.Errorf("LintCommand = %q, want %q", cfg.LintCommand, "golangci-lint run ./...")
	}
}

func TestLoadConfig_LintFieldsAbsent_DefaultToOff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doug.yaml")
	testutil.WriteFile(t, path, "max_retries: 3\n")

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LintEnabled {
		t.Error("LintEnabled should default to false when absent from config")
	}
	if cfg.LintCommand != "" {
		t.Errorf("LintCommand should default to empty string when absent, got %q", cfg.LintCommand)
	}
}

func TestBuildSystems_LintCmdDefaults(t *testing.T) {
	tests := []struct {
		buildSystem string
		wantLintCmd string
	}{
		{"go", "go vet ./..."},
		{"npm", "npm run lint"},
		{"pnpm", "pnpm run lint"},
		{"static", ""},
	}
	for _, tt := range tests {
		t.Run(tt.buildSystem, func(t *testing.T) {
			bs, ok := config.BuildSystems[tt.buildSystem]
			if !ok {
				t.Fatalf("build system %q not found in BuildSystems registry", tt.buildSystem)
			}
			if bs.LintCmd != tt.wantLintCmd {
				t.Errorf("BuildSystems[%q].LintCmd = %q, want %q", tt.buildSystem, bs.LintCmd, tt.wantLintCmd)
			}
		})
	}
}

// containsAll reports whether s contains all the given substrings.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
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
