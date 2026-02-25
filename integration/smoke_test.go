// Package integration contains the end-to-end smoke test for the doug
// orchestrator. The test exercises the full orchestration loop using a mock
// agent that writes a minimal valid SUCCESS session result on every invocation.
//
// Mock agent design: the test binary itself doubles as the mock agent. When
// the environment variable MOCK_AGENT_MODE=1 is set, TestMain routes execution
// to runAsMockAgent before any tests run, writes the SUCCESS session result,
// and exits. This avoids the need to build or ship a separate binary.
//
// Run with: go test ./integration/... -v -timeout 60s
package integration

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// mockAgentEnvKey is the environment variable that signals the test binary
// to act as a mock agent subprocess instead of running the test suite.
const mockAgentEnvKey = "MOCK_AGENT_MODE"

// dougBinaryPath holds the path to the doug binary built during TestMain.
// It is set once before tests run and read by test functions.
var dougBinaryPath string

// TestMain is the entry point for the integration test binary.
//
//   - When MOCK_AGENT_MODE=1: act as mock agent (read logs/ACTIVE_TASK.md,
//     write a SUCCESS result to the session file) and exit before any tests run.
//   - Otherwise: build the doug binary, then run the test suite normally.
func TestMain(m *testing.M) {
	if os.Getenv(mockAgentEnvKey) == "1" {
		os.Exit(runAsMockAgent())
	}
	// Delegate to a helper so that deferred cleanup runs before os.Exit.
	// (Deferred functions are skipped when os.Exit is called directly.)
	os.Exit(buildAndRun(m))
}

// buildAndRun builds the doug binary, stores its path in dougBinaryPath, runs
// the test suite, and returns the exit code. Cleanup of the binary temp dir is
// deferred so it runs before the caller's os.Exit fires.
func buildAndRun(m *testing.M) int {
	binDir, err := os.MkdirTemp("", "doug-smoke-bin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: create bin dir: %v\n", err)
		return 1
	}
	defer os.RemoveAll(binDir)

	dougBin := filepath.Join(binDir, "doug")
	if runtime.GOOS == "windows" {
		dougBin += ".exe"
	}

	// When go test runs, the working directory is the package source directory
	// (integration/). The module root is its parent.
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: getwd: %v\n", err)
		return 1
	}
	moduleRoot := filepath.Dir(cwd)

	buildCmd := exec.Command("go", "build", "-o", dougBin, ".")
	buildCmd.Dir = moduleRoot
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		fmt.Fprintf(os.Stderr, "TestMain: build doug binary: %v\n%s\n", buildErr, string(buildOut))
		return 1
	}

	dougBinaryPath = dougBin
	return m.Run()
}

// runAsMockAgent is called when MOCK_AGENT_MODE=1. The working directory is
// the project root (set by agent.RunAgent via cmd.Dir). It reads
// logs/ACTIVE_TASK.md, extracts the session file path, and writes a minimal
// SUCCESS result so the orchestrator can proceed.
func runAsMockAgent() int {
	data, err := os.ReadFile(filepath.Join("logs", "ACTIVE_TASK.md"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mock agent: read logs/ACTIVE_TASK.md: %v\n", err)
		return 1
	}

	// Scan for the "**Session File**: <path>" line written by WriteActiveTask.
	var sessionPath string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "**Session File**: ") {
			sessionPath = strings.TrimPrefix(line, "**Session File**: ")
			break
		}
	}

	if sessionPath == "" {
		fmt.Fprintln(os.Stderr, "mock agent: **Session File** line not found in logs/ACTIVE_TASK.md")
		return 1
	}

	// Write the minimal valid session result that the orchestrator expects.
	// Three fields only: outcome, changelog_entry, dependencies_added.
	const successResult = "---\noutcome: SUCCESS\nchangelog_entry: \"\"\ndependencies_added: []\n---\n\n## Summary\n\nMock agent completed task.\n"
	if err := os.WriteFile(sessionPath, []byte(successResult), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "mock agent: write session file %s: %v\n", sessionPath, err)
		return 1
	}

	return 0
}

// ---------------------------------------------------------------------------
// Smoke test
// ---------------------------------------------------------------------------

// TestSmokeEndToEnd exercises the full orchestrator loop end-to-end:
//   - Creates a real git repository with go.mod, tasks.yaml, and project-state.yaml.
//   - Uses a two-task epic (SMOKE-1-001, SMOKE-1-002).
//   - Runs doug with the test binary itself as the mock agent.
//   - Asserts both tasks end up DONE in tasks.yaml.
//   - Asserts the git log contains exactly two feat: commits.
func TestSmokeEndToEnd(t *testing.T) {
	projectDir := t.TempDir()

	// Create a minimal Go project so go build ./... and go test ./... pass.
	writeTestFile(t, projectDir, "go.mod", "module smoketest\n\ngo 1.21\n")
	writeTestFile(t, projectDir, "main.go", "package main\n\nfunc main() {}\n")

	// Write the orchestrator YAML state files.
	writeTestFile(t, projectDir, "project-state.yaml", smokeProjectState)
	writeTestFile(t, projectDir, "tasks.yaml", smokeTasksYAML)

	// Initialise a real git repository with an initial commit.
	initGitRepo(t, projectDir)

	// Tell any re-invocation of this test binary to act as mock agent.
	// t.Setenv modifies the current process env; subprocesses inherit it.
	t.Setenv(mockAgentEnvKey, "1")

	// The mock agent is the test binary itself. When Doug spawns it,
	// TestMain sees MOCK_AGENT_MODE=1 and routes to runAsMockAgent.
	testBinary := os.Args[0]

	// Run the full doug orchestration loop.
	runCmd := exec.Command(
		dougBinaryPath, "run",
		"--agent", testBinary,
		"--max-iterations=5",
		"--build-system=go",
	)
	runCmd.Dir = projectDir
	output, err := runCmd.CombinedOutput()
	t.Logf("doug run output:\n%s", string(output))
	if err != nil {
		t.Fatalf("doug run failed: %v\noutput:\n%s", err, string(output))
	}

	// Both user-defined tasks must be DONE.
	assertAllTasksDone(t, projectDir)

	// Exactly two feat: commits must appear in the git log (one per task).
	assertExactlyTwoFeatCommits(t, projectDir)
}

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

// smokeProjectState is the initial project-state.yaml for the smoke test.
// current_epic.id is empty so BootstrapFromTasks runs on first iteration.
// kb_enabled is true so the loop terminates naturally via KB synthesis once
// both feature tasks are DONE (preventing the loop from repeating the last
// task until max_iterations).
const smokeProjectState = `current_epic:
  id: ""
  name: ""
  branch_name: ""
  started_at: ""
active_task:
  type: ""
  id: ""
next_task:
  type: ""
  id: ""
kb_enabled: true
metrics:
  total_tasks_completed: 0
  total_duration_seconds: 0
  tasks: []
`

// smokeTasksYAML defines the two-task epic used by the smoke test.
const smokeTasksYAML = `epic:
  id: "SMOKE-1"
  name: "Smoke Test Epic"
  tasks:
    - id: "SMOKE-1-001"
      type: "feature"
      status: "TODO"
      description: "First smoke test task"
      acceptance_criteria:
        - "Task 1 completes"
    - id: "SMOKE-1-002"
      type: "feature"
      status: "TODO"
      description: "Second smoke test task"
      acceptance_criteria:
        - "Task 2 completes"
`

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeTestFile writes content to filename inside dir, failing the test on error.
func writeTestFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
}

// initGitRepo initialises a git repository in dir with repo-local user config
// and an initial commit that includes all current files.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	steps := [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"add", "-A"},
		{"commit", "-m", "initial setup"},
	}
	for _, args := range steps {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}
}

// tasksFileSchema is used only for asserting task statuses after the run.
type tasksFileSchema struct {
	Epic struct {
		Tasks []struct {
			ID     string `yaml:"id"`
			Status string `yaml:"status"`
		} `yaml:"tasks"`
	} `yaml:"epic"`
}

// assertAllTasksDone reads tasks.yaml and fails if any user-defined task is
// not in DONE status.
func assertAllTasksDone(t *testing.T, dir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "tasks.yaml"))
	if err != nil {
		t.Fatalf("assertAllTasksDone: read tasks.yaml: %v", err)
	}
	var tasks tasksFileSchema
	if err := yaml.Unmarshal(data, &tasks); err != nil {
		t.Fatalf("assertAllTasksDone: parse tasks.yaml: %v", err)
	}
	for _, task := range tasks.Epic.Tasks {
		if task.Status != "DONE" {
			t.Errorf("task %s: expected status DONE, got %q", task.ID, task.Status)
		}
	}
}

// assertExactlyTwoFeatCommits reads the git log and verifies there are exactly
// two commits whose subject starts with "feat: ", matching the expected task IDs.
func assertExactlyTwoFeatCommits(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "log", "--pretty=format:%s")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("assertExactlyTwoFeatCommits: git log: %v", err)
	}

	var featCommits []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "feat: ") {
			featCommits = append(featCommits, line)
		}
	}

	if len(featCommits) != 2 {
		t.Errorf("expected exactly 2 feat: commits, got %d: %v", len(featCommits), featCommits)
		return
	}

	// Verify that the two commits match the expected task IDs.
	want := map[string]bool{
		"feat: SMOKE-1-001": false,
		"feat: SMOKE-1-002": false,
	}
	for _, commit := range featCommits {
		if _, ok := want[commit]; ok {
			want[commit] = true
		} else {
			t.Errorf("unexpected feat: commit: %q", commit)
		}
	}
	for msg, found := range want {
		if !found {
			t.Errorf("missing expected commit: %q", msg)
		}
	}
}
