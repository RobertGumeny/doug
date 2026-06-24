package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/orchestrator"
)

type reviewExecutorFunc func(context.Context, string) (string, error)

func (f reviewExecutorFunc) ReviewCompletedEpic(ctx context.Context, epicID string) (string, error) {
	return f(ctx, epicID)
}

func TestReviewCommandRequiresEpicID(t *testing.T) {
	cmd := &cobra.Command{Use: reviewCmd.Use, Args: reviewCmd.Args}
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Fatal("expected missing argument validation error")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestRunReviewInvokesExecutorAndPrintsArtifactPath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll(filepath.Join(dir, ".doug"), 0o755); err != nil {
		t.Fatalf("create .doug: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".doug", "doug.yaml"), []byte("build_system: go\nreview_enabled: false\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	old := newReviewExecutor
	t.Cleanup(func() { newReviewExecutor = old })
	called := false
	newReviewExecutor = func(cfg *config.OrchestratorConfig, paths orchestrator.Paths) (reviewExecutor, error) {
		called = true
		if cfg.BuildSystem != "go" {
			t.Fatalf("BuildSystem = %q, want go", cfg.BuildSystem)
		}
		if paths.ProjectRoot != dir {
			t.Fatalf("ProjectRoot = %q, want %q", paths.ProjectRoot, dir)
		}
		return reviewExecutorFunc(func(_ context.Context, epicID string) (string, error) {
			if epicID != "EPIC-50" {
				t.Fatalf("epicID = %q, want EPIC-50", epicID)
			}
			return filepath.Join(dir, ".doug", "logs", "reviews", "EPIC-50", "epic-review.md"), nil
		}), nil
	}

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runReview(cmd, []string{"EPIC-50"}); err != nil {
		t.Fatalf("runReview: %v", err)
	}
	if !called {
		t.Fatal("expected review executor to be constructed")
	}
	if !strings.Contains(out.String(), "epic-review.md") {
		t.Fatalf("expected artifact path in output, got %q", out.String())
	}
}

func TestRunReviewPropagatesMissingArchiveError(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	old := newReviewExecutor
	t.Cleanup(func() { newReviewExecutor = old })
	missingArchive := errors.New("completed runtime archive for epic EPIC-50 is missing")
	newReviewExecutor = func(*config.OrchestratorConfig, orchestrator.Paths) (reviewExecutor, error) {
		return reviewExecutorFunc(func(context.Context, string) (string, error) {
			return "", missingArchive
		}), nil
	}

	err := runReview(&cobra.Command{}, []string{"EPIC-50"})
	if !errors.Is(err, missingArchive) {
		t.Fatalf("expected missing archive error, got %v", err)
	}
}

func TestReviewCommandRegistered(t *testing.T) {
	for _, command := range rootCmd.Commands() {
		if command.Name() == "review" {
			return
		}
	}
	t.Fatal("review command is not registered on root command")
}
