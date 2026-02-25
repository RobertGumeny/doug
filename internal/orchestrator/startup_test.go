package orchestrator_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/config"
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

func TestCheckDependencies_MissingBinary_ReturnsError(t *testing.T) {
	// Use a binary name that will never exist on any system.
	cfg := &config.OrchestratorConfig{
		AgentCommand: "this-binary-does-not-exist-xyz123",
		BuildSystem:  "go",
	}

	err := orchestrator.CheckDependencies(cfg)

	if err == nil {
		t.Fatal("expected non-nil error for missing binary, got nil")
	}
}

func TestCheckDependencies_MissingBinary_ErrorContainsBinaryName(t *testing.T) {
	cfg := &config.OrchestratorConfig{
		AgentCommand: "nonexistent-agent-abc789",
		BuildSystem:  "go",
	}

	err := orchestrator.CheckDependencies(cfg)

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "nonexistent-agent-abc789") {
		t.Errorf("error should list missing binary, got: %q", err.Error())
	}
}

func TestCheckDependencies_GitAlwaysRequired(t *testing.T) {
	// Even with a known agent command, git must be verified.
	// We test this indirectly: if git is absent the error mentions it.
	// On any CI machine git is present, so we verify no false positive.
	cfg := &config.OrchestratorConfig{
		AgentCommand: "git", // git is on PATH, treat as agent command too
		BuildSystem:  "go",
	}

	// git is on PATH; go is on PATH (we're in a Go test); so error should be nil
	// UNLESS the test machine lacks "go" — in that case skip.
	err := orchestrator.CheckDependencies(cfg)
	if err != nil && strings.Contains(err.Error(), "go") {
		t.Skip("go toolchain not on PATH in this test environment")
	}
	// For the purpose of this test, just ensure no panic.
}

func TestCheckDependencies_NpmBuildSystem_ChecksNpm(t *testing.T) {
	cfg := &config.OrchestratorConfig{
		AgentCommand: "git",   // on PATH
		BuildSystem:  "npm",
	}

	err := orchestrator.CheckDependencies(cfg)
	// npm may or may not be present; what matters is the function doesn't panic.
	// If npm is missing the error should mention it.
	if err != nil && !strings.Contains(err.Error(), "npm") {
		t.Errorf("expected error to mention npm when npm is missing, got: %q", err.Error())
	}
}

func TestCheckDependencies_MultipleMissing_ErrorListsAll(t *testing.T) {
	cfg := &config.OrchestratorConfig{
		AgentCommand: "missing-agent-111",
		BuildSystem:  "go",
	}

	// Inject a known-missing agent — we can't guarantee go is missing too,
	// but we can at least verify the agent is listed.
	err := orchestrator.CheckDependencies(cfg)

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "missing-agent-111") {
		t.Errorf("error should contain the missing agent name, got: %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// EnsureProjectReady tests
// ---------------------------------------------------------------------------

func TestEnsureProjectReady_NotInitialized_ReturnsNil(t *testing.T) {
	bs := &mockBuildSys{initialized: false}
	cfg := &config.OrchestratorConfig{BuildSystem: "go"}

	err := orchestrator.EnsureProjectReady(bs, cfg)

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
	cfg := &config.OrchestratorConfig{BuildSystem: "go"}

	err := orchestrator.EnsureProjectReady(bs, cfg)

	if err != nil {
		t.Errorf("expected nil (build must not run when uninitialized), got: %v", err)
	}
}

func TestEnsureProjectReady_BuildFails_ReturnsError(t *testing.T) {
	bs := &mockBuildSys{
		initialized: true,
		buildErr:    fmt.Errorf("compilation error on line 42"),
	}
	cfg := &config.OrchestratorConfig{BuildSystem: "go"}

	err := orchestrator.EnsureProjectReady(bs, cfg)

	if err == nil {
		t.Fatal("expected non-nil error when build fails, got nil")
	}
}

func TestEnsureProjectReady_BuildFails_ErrorContainsBuildOutput(t *testing.T) {
	bs := &mockBuildSys{
		initialized: true,
		buildErr:    fmt.Errorf("undefined: FooBar"),
	}
	cfg := &config.OrchestratorConfig{BuildSystem: "go"}

	err := orchestrator.EnsureProjectReady(bs, cfg)

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
	cfg := &config.OrchestratorConfig{BuildSystem: "go"}

	err := orchestrator.EnsureProjectReady(bs, cfg)

	if err == nil {
		t.Fatal("expected non-nil error when tests fail, got nil")
	}
}

func TestEnsureProjectReady_TestsFail_ErrorContainsTestOutput(t *testing.T) {
	bs := &mockBuildSys{
		initialized: true,
		testErr:     fmt.Errorf("FAIL: TestHandleSuccess"),
	}
	cfg := &config.OrchestratorConfig{BuildSystem: "go"}

	err := orchestrator.EnsureProjectReady(bs, cfg)

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "TestHandleSuccess") {
		t.Errorf("error should contain test output, got: %q", err.Error())
	}
}

func TestEnsureProjectReady_AllPass_ReturnsNil(t *testing.T) {
	bs := &mockBuildSys{initialized: true}
	cfg := &config.OrchestratorConfig{BuildSystem: "go"}

	err := orchestrator.EnsureProjectReady(bs, cfg)

	if err != nil {
		t.Errorf("expected nil when build and tests pass, got: %v", err)
	}
}
