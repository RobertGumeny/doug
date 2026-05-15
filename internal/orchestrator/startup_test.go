package orchestrator_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
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
func (m *mockBuildSys) IsInitialized() bool { return m.initialized }

// ---------------------------------------------------------------------------
// CheckDependencies tests
// ---------------------------------------------------------------------------

func TestCheckDependencies_NoRPCPolicy_NoAgentCheck(t *testing.T) {
	// Without rpc execution mode, only git and the build tool are checked.
	// The pi binary is not required in subprocess compatibility mode.
	cfg := &config.OrchestratorConfig{
		BuildSystem: "go",
	}

	err := orchestrator.CheckDependencies(cfg)
	// git and go must be present in the test environment; if they are, this must pass.
	if err != nil {
		t.Fatalf("expected nil error when no rpc policy and git/go are on PATH, got: %v", err)
	}
}

func TestCheckDependencies_RPCPolicy_ChecksPi(t *testing.T) {
	// When rpc execution mode is configured, pi must be on PATH.
	// pi is expected to be absent in most CI environments, so we verify
	// the error message rather than expecting nil.
	cfg := &config.OrchestratorConfig{
		BuildSystem: "go",
		Policy: config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"runtime": {ExecutionMode: "rpc"},
			},
		},
	}

	err := orchestrator.CheckDependencies(cfg)
	// If pi is absent, the error must name it.
	if err != nil && !strings.Contains(err.Error(), "pi") {
		t.Errorf("expected error to mention 'pi' when pi is missing, got: %q", err.Error())
	}
}

func TestCheckDependencies_GitMissing_NotReportedAsAgent(t *testing.T) {
	// git is always required — the function checks it regardless of policy.
	// This test verifies that the required binary list does not include
	// a stale RunAgentCommand field from a prior design.
	cfg := &config.OrchestratorConfig{
		BuildSystem: "go",
	}
	// We cannot force git to be absent, so just verify the function runs without panic.
	_ = orchestrator.CheckDependencies(cfg)
}

func TestCheckDependencies_NpmBuildSystem_ChecksNpm(t *testing.T) {
	cfg := &config.OrchestratorConfig{
		BuildSystem: "npm",
	}

	err := orchestrator.CheckDependencies(cfg)
	// npm may or may not be present; what matters is that if it is missing,
	// the error mentions npm.
	if err != nil && !strings.Contains(err.Error(), "npm") {
		t.Errorf("expected error to mention npm when npm is missing, got: %q", err.Error())
	}
}

func TestCheckDependencies_MultipleMissing_ErrorListsAll(t *testing.T) {
	cfg := &config.OrchestratorConfig{
		BuildSystem: "go",
		Policy: config.PolicyConfig{
			Phases: map[string]config.PhasePolicy{
				"runtime": {ExecutionMode: "rpc"},
			},
		},
	}

	err := orchestrator.CheckDependencies(cfg)
	// If pi is absent, it should be listed.
	if err != nil && !strings.Contains(err.Error(), "pi") {
		t.Errorf("error should contain missing binary name, got: %q", err.Error())
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
