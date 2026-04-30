package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// dougBin and mockAgentBin are set by TestMain after compiling both binaries.
var (
	dougBin      string
	mockAgentBin string
)

// TestMain compiles the doug binary and the mock agent once, then runs all
// tests. Both binaries are written to a shared temp directory.
func TestMain(m *testing.M) {
	binDir, err := os.MkdirTemp("", "doug-integration-bins-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: create bin dir: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if removeErr := os.RemoveAll(binDir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "TestMain: cleanup bin dir: %v\n", removeErr)
		}
	}()

	exeSuffix := ""
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}

	dougBin = filepath.Join(binDir, "doug"+exeSuffix)
	mockAgentBin = filepath.Join(binDir, "mockagent"+exeSuffix)

	// Build doug from the module root (one level up from integration/).
	if err := buildBinary("../", dougBin); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: build doug: %v\n", err)
		os.Exit(1)
	}

	// Build the mock agent.
	if err := buildBinary("./testdata/mockagent", mockAgentBin); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: build mockagent: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// buildBinary compiles the Go package at src into outBin.
func buildBinary(src, outBin string) error {
	cmd := exec.Command("go", "build", "-o", outBin, src)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build -o %s %s: %w\n%s", outBin, src, err, out)
	}
	return nil
}

// TestSmokeFullLoop runs the full orchestration loop with a mock agent and
// verifies that the task is marked DONE in .doug/tasks.yaml after a single iteration.
func TestSmokeFullLoop(t *testing.T) {
	// Skip if required tools are not on PATH.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH; skipping smoke test")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found on PATH; skipping smoke test")
	}

	dir := t.TempDir()

	// Write a minimal Go project so build/test verification in HandleSuccess passes.
	// go.sum is not needed for a module with no external dependencies.
	writeFile(t, filepath.Join(dir, "go.mod"), "module smoke-test\n\ngo 1.21\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	// Initialize a git repo with an initial commit so EnsureEpicBranch works.
	mustRunGit(t, dir, "init")
	mustRunGit(t, dir, "config", "user.email", "test@example.com")
	mustRunGit(t, dir, "config", "user.name", "Test")
	mustRunGit(t, dir, "add", "-A")
	mustRunGit(t, dir, "commit", "-m", "initial commit")

	// Run doug init to scaffold the project.
	runCmd(t, dir, dougBin, "init")

	// Overwrite .doug/tasks.yaml with a single TODO feature task.
	writeTestTasksYAML(t, dir)

	// Write a minimal .doug/doug.yaml:
	//   - kb_enabled: false to avoid injecting a documentation task
	//   - max_iterations: 1 to exit after one agent invocation
	writeFile(t, filepath.Join(dir, ".doug", "doug.yaml"),
		"build_system: go\nmax_retries: 5\nmax_iterations: 1\nkb_enabled: false\n")

	// Run: doug run --agent <mockAgentBin>
	// --agent overrides agent_command so the mock binary is invoked.
	// filepath.ToSlash converts the path to forward slashes so that
	// splitShellArgs inside RunAgent does not mistake Windows path separators
	// (\) for POSIX escape characters. Forward slashes are valid on Windows.
	cmd := exec.Command(dougBin, "run", "--agent", filepath.ToSlash(mockAgentBin))
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doug run failed:\n%s\nerr: %v", out, err)
	}

	// Assert: .doug/tasks.yaml shows EPIC-1-001 as DONE.
	tasksData, readErr := os.ReadFile(filepath.Join(dir, ".doug", "tasks.yaml"))
	if readErr != nil {
		t.Fatalf("read tasks.yaml: %v", readErr)
	}

	var tasks struct {
		Epic struct {
			Tasks []struct {
				ID     string `yaml:"id"`
				Status string `yaml:"status"`
			} `yaml:"tasks"`
		} `yaml:"epic"`
	}
	if err := yaml.Unmarshal(tasksData, &tasks); err != nil {
		t.Fatalf("parse tasks.yaml: %v", err)
	}
	if len(tasks.Epic.Tasks) == 0 {
		t.Fatal("tasks.yaml: no tasks found")
	}

	got := tasks.Epic.Tasks[0].Status
	if got != "DONE" {
		t.Errorf("expected EPIC-1-001 status DONE, got %q\ndoug run output:\n%s", got, out)
	}
}

// TestBugFixAndResume verifies the BUG→bugfix→SUCCESS loop path.
//
// The mock agent reports BUG on the first call (which causes the orchestrator
// to schedule a synthetic bugfix task), SUCCESS on the second call (completing
// the bugfix), and SUCCESS on the third call (completing the original feature
// task). After three iterations the original task must be marked DONE.
func TestBugFixAndResume(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH; skipping smoke test")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found on PATH; skipping smoke test")
	}

	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module smoke-test\n\ngo 1.21\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	mustRunGit(t, dir, "init")
	mustRunGit(t, dir, "config", "user.email", "test@example.com")
	mustRunGit(t, dir, "config", "user.name", "Test")
	mustRunGit(t, dir, "add", "-A")
	mustRunGit(t, dir, "commit", "-m", "initial commit")

	runCmd(t, dir, dougBin, "init")

	writeFile(t, filepath.Join(dir, ".doug", "tasks.yaml"), `epic:
  id: "EPIC-1"
  name: "Test Epic"
  tasks:
    - id: "EPIC-1-001"
      type: "feature"
      status: "TODO"
      description: "Implement the bug fix resume feature."
      acceptance_criteria:
        - "Task completes after bugfix"
`)
	// max_iterations:3 — one for the BUG, one for the bugfix SUCCESS, one for
	// the original task SUCCESS.
	writeFile(t, filepath.Join(dir, ".doug", "doug.yaml"),
		"build_system: go\nmax_retries: 5\nmax_iterations: 3\nkb_enabled: false\n")

	// Script: BUG on call 0, SUCCESS on calls 1 and 2.
	writeFile(t, filepath.Join(dir, ".doug", "mockagent-script"), "BUG\nSUCCESS\nSUCCESS\n")

	cmd := exec.Command(dougBin, "run", "--agent", filepath.ToSlash(mockAgentBin))
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doug run failed:\n%s\nerr: %v", out, err)
	}

	tasksData, readErr := os.ReadFile(filepath.Join(dir, ".doug", "tasks.yaml"))
	if readErr != nil {
		t.Fatalf("read tasks.yaml: %v", readErr)
	}
	var tasks struct {
		Epic struct {
			Tasks []struct {
				ID     string `yaml:"id"`
				Status string `yaml:"status"`
			} `yaml:"tasks"`
		} `yaml:"epic"`
	}
	if err := yaml.Unmarshal(tasksData, &tasks); err != nil {
		t.Fatalf("parse tasks.yaml: %v", err)
	}
	if len(tasks.Epic.Tasks) == 0 {
		t.Fatal("tasks.yaml: no tasks found")
	}
	if got := tasks.Epic.Tasks[0].Status; got != "DONE" {
		t.Errorf("expected EPIC-1-001 status DONE after bugfix+resume, got %q\ndoug run output:\n%s", got, out)
	}
}

func TestBugfixDispatchFailsWithoutActiveBugContext(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH; skipping smoke test")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found on PATH; skipping smoke test")
	}

	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module smoke-test\n\ngo 1.21\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	mustRunGit(t, dir, "init")
	mustRunGit(t, dir, "config", "user.email", "test@example.com")
	mustRunGit(t, dir, "config", "user.name", "Test")
	mustRunGit(t, dir, "add", "-A")
	mustRunGit(t, dir, "commit", "-m", "initial commit")

	runCmd(t, dir, dougBin, "init")
	writeTestTasksYAML(t, dir)
	writeFile(t, filepath.Join(dir, ".doug", "doug.yaml"),
		"build_system: go\nmax_retries: 5\nmax_iterations: 1\nkb_enabled: false\n")

	projectState := &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID:         "EPIC-1",
			Name:       "Test Epic",
			BranchName: "epic/EPIC-1",
			StartedAt:  "2026-04-13T00:00:00Z",
		},
		ActiveTask: types.TaskPointer{
			Type:     types.TaskTypeBugfix,
			ID:       "BUG-EPIC-1-001",
			Attempts: 1,
		},
	}
	if err := state.SaveProjectState(filepath.Join(dir, ".doug", "project-state.yaml"), projectState); err != nil {
		t.Fatalf("write project-state.yaml: %v", err)
	}

	cmd := exec.Command(dougBin, "run", "--agent", filepath.ToSlash(mockAgentBin))
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doug run to fail without ACTIVE_BUG.md, got success:\n%s", out)
	}
	if !containsAll(string(out), "ACTIVE_BUG.md", "cannot dispatch bugfix agent") {
		t.Fatalf("expected error about missing ACTIVE_BUG.md, got:\n%s", out)
	}
}

// TestFailureRetryBlocked verifies the FAILURE→retry→BLOCKED loop path.
//
// The mock agent always reports FAILURE. With max_retries:2 the orchestrator
// retries once and then marks the task BLOCKED. doug run must exit with a
// non-zero status and tasks.yaml must show the task as BLOCKED.
func TestFailureRetryBlocked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH; skipping smoke test")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found on PATH; skipping smoke test")
	}

	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module smoke-test\n\ngo 1.21\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	mustRunGit(t, dir, "init")
	mustRunGit(t, dir, "config", "user.email", "test@example.com")
	mustRunGit(t, dir, "config", "user.name", "Test")
	mustRunGit(t, dir, "add", "-A")
	mustRunGit(t, dir, "commit", "-m", "initial commit")

	runCmd(t, dir, dougBin, "init")

	writeFile(t, filepath.Join(dir, ".doug", "tasks.yaml"), `epic:
  id: "EPIC-1"
  name: "Test Epic"
  tasks:
    - id: "EPIC-1-001"
      type: "feature"
      status: "TODO"
      description: "Implement a feature that always fails."
      acceptance_criteria:
        - "Task is blocked after max retries"
`)
	// max_retries:2 — attempt 1 retries, attempt 2 blocks.
	writeFile(t, filepath.Join(dir, ".doug", "doug.yaml"),
		"build_system: go\nmax_retries: 2\nmax_iterations: 5\nkb_enabled: false\n")

	// All calls return FAILURE.
	writeFile(t, filepath.Join(dir, ".doug", "mockagent-script"), "FAILURE\nFAILURE\nFAILURE\n")

	cmd := exec.Command(dougBin, "run", "--agent", filepath.ToSlash(mockAgentBin))
	cmd.Dir = dir
	out, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("expected doug run to exit with an error after task is BLOCKED, but it exited 0\noutput:\n%s", out)
	}

	tasksData, readErr := os.ReadFile(filepath.Join(dir, ".doug", "tasks.yaml"))
	if readErr != nil {
		t.Fatalf("read tasks.yaml: %v", readErr)
	}
	var tasks struct {
		Epic struct {
			Tasks []struct {
				ID     string `yaml:"id"`
				Status string `yaml:"status"`
			} `yaml:"tasks"`
		} `yaml:"epic"`
	}
	if err := yaml.Unmarshal(tasksData, &tasks); err != nil {
		t.Fatalf("parse tasks.yaml: %v", err)
	}
	if len(tasks.Epic.Tasks) == 0 {
		t.Fatal("tasks.yaml: no tasks found")
	}
	if got := tasks.Epic.Tasks[0].Status; got != "BLOCKED" {
		t.Errorf("expected EPIC-1-001 status BLOCKED, got %q\ndoug run output:\n%s", got, out)
	}
}

// TestMalformedOutcomeStopsWithContractError verifies that an invalid outcome
// value does not flow through the normal FAILURE retry/block path.
func TestMalformedOutcomeStopsWithContractError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH; skipping smoke test")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found on PATH; skipping smoke test")
	}

	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "go.mod"), "module smoke-test\n\ngo 1.21\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	mustRunGit(t, dir, "init")
	mustRunGit(t, dir, "config", "user.email", "test@example.com")
	mustRunGit(t, dir, "config", "user.name", "Test")
	mustRunGit(t, dir, "add", "-A")
	mustRunGit(t, dir, "commit", "-m", "initial commit")

	runCmd(t, dir, dougBin, "init")
	writeTestTasksYAML(t, dir)
	writeFile(t, filepath.Join(dir, ".doug", "doug.yaml"),
		"build_system: go\nmax_retries: 2\nmax_iterations: 5\nkb_enabled: false\n")

	// Lowercase "completed" is intentionally invalid: valid outcomes are
	// SUCCESS, FAILURE, BUG, and EPIC_COMPLETE.
	writeFile(t, filepath.Join(dir, ".doug", "mockagent-script"), "completed\n")

	cmd := exec.Command(dougBin, "run", "--agent", filepath.ToSlash(mockAgentBin))
	cmd.Dir = dir
	out, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("expected doug run to fail on malformed outcome, got success:\n%s", out)
	}
	if !containsAll(string(out), "agent result contract error", `invalid outcome "completed"`) {
		t.Fatalf("expected contract error with invalid outcome value, got:\n%s", out)
	}

	tasksData, readErr := os.ReadFile(filepath.Join(dir, ".doug", "tasks.yaml"))
	if readErr != nil {
		t.Fatalf("read tasks.yaml: %v", readErr)
	}
	var tasks struct {
		Epic struct {
			Tasks []struct {
				ID     string `yaml:"id"`
				Status string `yaml:"status"`
			} `yaml:"tasks"`
		} `yaml:"epic"`
	}
	if err := yaml.Unmarshal(tasksData, &tasks); err != nil {
		t.Fatalf("parse tasks.yaml: %v", err)
	}
	if len(tasks.Epic.Tasks) == 0 {
		t.Fatal("tasks.yaml: no tasks found")
	}
	if got := tasks.Epic.Tasks[0].Status; got != "TODO" {
		t.Errorf("expected task to remain TODO after malformed outcome, got %q\noutput:\n%s", got, out)
	}

	stateData, stateErr := os.ReadFile(filepath.Join(dir, ".doug", "project-state.yaml"))
	if stateErr != nil {
		t.Fatalf("read project-state.yaml: %v", stateErr)
	}
	var projectState struct {
		ActiveTask struct {
			Attempts int    `yaml:"attempts"`
			Type     string `yaml:"type"`
		} `yaml:"active_task"`
	}
	if err := yaml.Unmarshal(stateData, &projectState); err != nil {
		t.Fatalf("parse project-state.yaml: %v", err)
	}
	if projectState.ActiveTask.Attempts != 0 {
		t.Errorf("expected malformed outcome to restore attempts to 0, got %d", projectState.ActiveTask.Attempts)
	}
	if projectState.ActiveTask.Type != "feature" {
		t.Errorf("expected active task type to remain feature, got %q", projectState.ActiveTask.Type)
	}
}

// TestBuildFailAfterSuccess verifies the SUCCESS→build-fail retry path.
//
// The mock agent reports SUCCESS but simultaneously corrupts main.go so that
// the orchestrator's build verification fails. doug run exits cleanly with
// status 0 but the task must NOT be marked DONE and the project must be PAUSED.
// After the test fixes main.go, a second doug run resumes from the PAUSED state,
// passes build verification, and marks the task DONE — demonstrating that the
// retry iteration is triggered correctly.
func TestBuildFailAfterSuccess(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH; skipping smoke test")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found on PATH; skipping smoke test")
	}

	dir := t.TempDir()

	validMain := "package main\n\nfunc main() {}\n"
	writeFile(t, filepath.Join(dir, "go.mod"), "module smoke-test\n\ngo 1.21\n")
	writeFile(t, filepath.Join(dir, "main.go"), validMain)

	mustRunGit(t, dir, "init")
	mustRunGit(t, dir, "config", "user.email", "test@example.com")
	mustRunGit(t, dir, "config", "user.name", "Test")
	mustRunGit(t, dir, "add", "-A")
	mustRunGit(t, dir, "commit", "-m", "initial commit")

	runCmd(t, dir, dougBin, "init")

	writeFile(t, filepath.Join(dir, ".doug", "tasks.yaml"), `epic:
  id: "EPIC-1"
  name: "Test Epic"
  tasks:
    - id: "EPIC-1-001"
      type: "feature"
      status: "TODO"
      description: "Implement a feature with a build failure."
      acceptance_criteria:
        - "Task resumes after build is fixed"
`)
	writeFile(t, filepath.Join(dir, ".doug", "doug.yaml"),
		"build_system: go\nmax_retries: 5\nmax_iterations: 1\nkb_enabled: false\n")

	// Call 0: report SUCCESS but break the build.
	writeFile(t, filepath.Join(dir, ".doug", "mockagent-script"), "SUCCESS+BREAK_BUILD\n")

	// Run 1: agent writes SUCCESS + corrupts main.go → build fails → project PAUSED.
	run1 := exec.Command(dougBin, "run", "--agent", filepath.ToSlash(mockAgentBin))
	run1.Dir = dir
	out1, err1 := run1.CombinedOutput()
	if err1 != nil {
		t.Fatalf("doug run (run 1) failed unexpectedly:\n%s\nerr: %v", out1, err1)
	}

	// Assert: task NOT DONE after build failure.
	tasksData, readErr := os.ReadFile(filepath.Join(dir, ".doug", "tasks.yaml"))
	if readErr != nil {
		t.Fatalf("read tasks.yaml after run 1: %v", readErr)
	}
	var tasks struct {
		Epic struct {
			Tasks []struct {
				ID     string `yaml:"id"`
				Status string `yaml:"status"`
			} `yaml:"tasks"`
		} `yaml:"epic"`
	}
	if err := yaml.Unmarshal(tasksData, &tasks); err != nil {
		t.Fatalf("parse tasks.yaml after run 1: %v", err)
	}
	if len(tasks.Epic.Tasks) == 0 {
		t.Fatal("tasks.yaml: no tasks found")
	}
	if got := tasks.Epic.Tasks[0].Status; got == "DONE" {
		t.Errorf("task must NOT be DONE after build failure, but got %q\nrun 1 output:\n%s", got, out1)
	}

	// Assert: project is PAUSED.
	stateData, stateErr := os.ReadFile(filepath.Join(dir, ".doug", "project-state.yaml"))
	if stateErr != nil {
		t.Fatalf("read project-state.yaml after run 1: %v", stateErr)
	}
	var projectState struct {
		Status string `yaml:"status"`
	}
	if err := yaml.Unmarshal(stateData, &projectState); err != nil {
		t.Fatalf("parse project-state.yaml after run 1: %v", err)
	}
	if projectState.Status != "PAUSED" {
		t.Errorf("expected project status PAUSED after build failure, got %q\nrun 1 output:\n%s", projectState.Status, out1)
	}

	// Fix main.go so the retry iteration can pass build verification.
	writeFile(t, filepath.Join(dir, "main.go"), validMain)

	// Run 2: resume from PAUSED state — build verification passes, task is marked DONE.
	run2 := exec.Command(dougBin, "run", "--agent", filepath.ToSlash(mockAgentBin))
	run2.Dir = dir
	out2, err2 := run2.CombinedOutput()
	if err2 != nil {
		t.Fatalf("doug run (run 2 / retry) failed:\n%s\nerr: %v", out2, err2)
	}

	tasksData2, readErr2 := os.ReadFile(filepath.Join(dir, ".doug", "tasks.yaml"))
	if readErr2 != nil {
		t.Fatalf("read tasks.yaml after run 2: %v", readErr2)
	}
	if err := yaml.Unmarshal(tasksData2, &tasks); err != nil {
		t.Fatalf("parse tasks.yaml after run 2: %v", err)
	}
	if got := tasks.Epic.Tasks[0].Status; got != "DONE" {
		t.Errorf("expected EPIC-1-001 status DONE after retry, got %q\nrun 2 output:\n%s", got, out2)
	}
}

// writeTestTasksYAML writes a minimal .doug/tasks.yaml with a single TODO feature task.
func writeTestTasksYAML(t *testing.T, dir string) {
	t.Helper()
	content := `epic:
  id: "EPIC-1"
  name: "Test Epic"
  tasks:
    - id: "EPIC-1-001"
      type: "feature"
      status: "TODO"
      description: "Implement the smoke test feature."
      acceptance_criteria:
        - "Smoke test passes"
`
	writeFile(t, filepath.Join(dir, ".doug", "tasks.yaml"), content)
}

// writeFile creates a file and its parent directories with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// mustRunGit runs a git command in dir and fails the test on error.
func mustRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// runCmd executes a command in dir and fails the test on error.
func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}
