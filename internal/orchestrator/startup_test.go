package orchestrator_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/testutil"
)

// ---------------------------------------------------------------------------
// Mock build system for startup tests
// ---------------------------------------------------------------------------

type mockBuildSys struct {
	initialized bool
	buildErr    error
	testErr     error
}

func (m *mockBuildSys) Install() error      { return nil }
func (m *mockBuildSys) Build() error        { return m.buildErr }
func (m *mockBuildSys) Test() error         { return m.testErr }
func (m *mockBuildSys) Lint() error         { return nil }
func (m *mockBuildSys) IsInitialized() bool { return m.initialized }

func setPATHWithFakeBinaries(t *testing.T, names ...string) {
	t.Helper()
	shimDir := t.TempDir()
	for _, name := range names {
		testutil.WriteFile(t, filepath.Join(shimDir, name), "#!/bin/sh\nexit 0\n")
		if err := os.Chmod(filepath.Join(shimDir, name), 0o755); err != nil {
			t.Fatalf("chmod fake binary %s: %v", name, err)
		}
	}
	t.Setenv("PATH", shimDir)
}

// ---------------------------------------------------------------------------
// CheckDependencies tests
// ---------------------------------------------------------------------------

func TestCheckDependencies_NoPolicy_ChecksPiFromSourceOwnedRouting(t *testing.T) {
	setPATHWithFakeBinaries(t, "git", "go")

	cfg := &config.OrchestratorConfig{BuildSystem: "go"}

	err := orchestrator.CheckDependencies(cfg)
	if err == nil {
		t.Fatal("expected missing-pi error, got nil")
	}
	if !strings.Contains(err.Error(), "pi") {
		t.Errorf("expected error to mention 'pi', got: %q", err.Error())
	}
}

func TestCheckDependencies_InteractivePolicy_ChecksPi(t *testing.T) {
	setPATHWithFakeBinaries(t, "git", "go")

	cfg := &config.OrchestratorConfig{
		BuildSystem: "go",
		Policy: config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"planning": {InteractionMode: config.InteractionModeInteractive},
			},
		},
	}

	err := orchestrator.CheckDependencies(cfg)
	if err == nil {
		t.Fatal("expected missing-pi error, got nil")
	}
	if !strings.Contains(err.Error(), "pi") {
		t.Errorf("expected error to mention 'pi', got: %q", err.Error())
	}
}

func TestCheckDependencies_RPCPolicy_ChecksPi(t *testing.T) {
	setPATHWithFakeBinaries(t, "git", "go")

	cfg := &config.OrchestratorConfig{
		BuildSystem: "go",
		Policy: config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"runtime": {InteractionMode: config.InteractionModeRPC},
			},
		},
	}

	err := orchestrator.CheckDependencies(cfg)
	if err == nil {
		t.Fatal("expected missing-pi error, got nil")
	}
	if !strings.Contains(err.Error(), "pi") {
		t.Errorf("expected error to mention 'pi', got: %q", err.Error())
	}
}

func TestCheckDependencies_GitMissing_NotReportedAsAgent(t *testing.T) {
	setPATHWithFakeBinaries(t, "go")

	cfg := &config.OrchestratorConfig{BuildSystem: "go"}

	err := orchestrator.CheckDependencies(cfg)
	if err == nil {
		t.Fatal("expected missing-git error, got nil")
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("expected error to mention git, got: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "pi") {
		t.Errorf("expected error to mention pi, got: %q", err.Error())
	}
	if strings.Contains(err.Error(), "agent") {
		t.Errorf("error should not refer to stale agent command dependency, got: %q", err.Error())
	}
}

func TestCheckDependencies_NpmBuildSystem_ChecksNpm(t *testing.T) {
	setPATHWithFakeBinaries(t, "git")

	cfg := &config.OrchestratorConfig{BuildSystem: "npm"}

	err := orchestrator.CheckDependencies(cfg)
	if err == nil {
		t.Fatal("expected missing-npm error, got nil")
	}
	if !strings.Contains(err.Error(), "npm") {
		t.Errorf("expected error to mention npm, got: %q", err.Error())
	}
}

func TestCheckDependencies_MultipleMissing_ErrorListsAll(t *testing.T) {
	setPATHWithFakeBinaries(t)

	cfg := &config.OrchestratorConfig{
		BuildSystem: "go",
		Policy: config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"runtime": {InteractionMode: "rpc"},
			},
		},
	}

	err := orchestrator.CheckDependencies(cfg)
	if err == nil {
		t.Fatal("expected missing-binaries error, got nil")
	}
	for _, want := range []string{"pi", "git", "go"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to mention %q, got: %q", want, err.Error())
		}
	}
}

// ---------------------------------------------------------------------------
// EnsureProjectReady tests
// ---------------------------------------------------------------------------

func TestEnsureProjectReady_NotInitialized_ReturnsNil(t *testing.T) {
	bs := &mockBuildSys{initialized: false}

	err := orchestrator.EnsureProjectReady(bs, "go", log.Discard())

	if err != nil {
		t.Errorf("expected nil when not initialized (skip pre-flight), got: %v", err)
	}
}

func TestEnsureProjectReady_NotInitialized_DoesNotRunBuild(t *testing.T) {
	// If build were called, this error would be returned.
	bs := &mockBuildSys{
		initialized: false,
		buildErr:    fmt.Errorf("build should not be called"),
	}

	err := orchestrator.EnsureProjectReady(bs, "go", log.Discard())

	if err != nil {
		t.Errorf("expected nil (build must not run when uninitialized), got: %v", err)
	}
}

func TestEnsureProjectReady_BuildFails_ReturnsError(t *testing.T) {
	bs := &mockBuildSys{
		initialized: true,
		buildErr:    fmt.Errorf("compilation error on line 42"),
	}

	err := orchestrator.EnsureProjectReady(bs, "go", log.Discard())

	if err == nil {
		t.Fatal("expected non-nil error when build fails, got nil")
	}
}

func TestEnsureProjectReady_BuildFails_ErrorContainsBuildOutput(t *testing.T) {
	bs := &mockBuildSys{
		initialized: true,
		buildErr:    fmt.Errorf("undefined: FooBar"),
	}

	err := orchestrator.EnsureProjectReady(bs, "go", log.Discard())

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "undefined: FooBar") {
		t.Errorf("error should contain build output, got: %q", err.Error())
	}
}

func TestEnsureProjectReady_TestsFail_ReturnsError(t *testing.T) {
	bs := &mockBuildSys{
		initialized: true,
		testErr:     fmt.Errorf("FAIL: TestAuthenticate"),
	}

	err := orchestrator.EnsureProjectReady(bs, "go", log.Discard())

	if err == nil {
		t.Fatal("expected non-nil error when tests fail, got nil")
	}
}

func TestEnsureProjectReady_TestsFail_ErrorContainsTestOutput(t *testing.T) {
	bs := &mockBuildSys{
		initialized: true,
		testErr:     fmt.Errorf("FAIL: TestHandleSuccess"),
	}

	err := orchestrator.EnsureProjectReady(bs, "go", log.Discard())

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "TestHandleSuccess") {
		t.Errorf("error should contain test output, got: %q", err.Error())
	}
}

func TestEnsureProjectReady_AllPass_ReturnsNil(t *testing.T) {
	bs := &mockBuildSys{initialized: true}

	err := orchestrator.EnsureProjectReady(bs, "go", log.Discard())

	if err != nil {
		t.Errorf("expected nil when build and tests pass, got: %v", err)
	}
}
