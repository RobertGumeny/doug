package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"
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
	defer os.RemoveAll(binDir)

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
// verifies that the task is marked DONE in tasks.yaml after a single iteration.
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

	// Overwrite tasks.yaml with a single TODO feature task.
	writeTestTasksYAML(t, dir)

	// Write a minimal doug.yaml:
	//   - kb_enabled: false to avoid injecting a documentation task
	//   - max_iterations: 1 to exit after one agent invocation
	writeFile(t, filepath.Join(dir, "doug.yaml"),
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

	// Assert: tasks.yaml shows EPIC-1-001 as DONE.
	tasksData, readErr := os.ReadFile(filepath.Join(dir, "tasks.yaml"))
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

// writeTestTasksYAML writes a minimal tasks.yaml with a single TODO feature task.
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
	writeFile(t, filepath.Join(dir, "tasks.yaml"), content)
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
