package plan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

// PromoteEpic copies a planned backlog epic package into root .doug/ and
// marks its metadata ACTIVE so the existing runtime rollover/bootstrap path can
// activate it on the subsequent orchestrator run.
func PromoteEpic(projectRoot, epicID string, now time.Time) error {
	paths := NewEpicPackagePaths(projectRoot, epicID)

	metadata, err := LoadEpicMetadata(paths.MetadataPath)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return fmt.Errorf("backlog epic %q not found at %s", epicID, paths.EpicDir)
		}
		return err
	}
	if metadata.Status != types.EpicStatusPlanned {
		return fmt.Errorf("backlog epic %q has status %q; only %q epics can be promoted", epicID, metadata.Status, types.EpicStatusPlanned)
	}

	statePath := filepath.Join(projectRoot, ".doug", "project-state.yaml")
	projectState, err := state.LoadProjectState(statePath)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return fmt.Errorf("project state not found at %s — run `doug init` to initialise the project", statePath)
		}
		return fmt.Errorf("load project state: %w", err)
	}
	if currentEpicActive(projectState) {
		return fmt.Errorf("runtime workspace already has active epic %q — complete or retire it before promoting %q", projectState.CurrentEpic.ID, epicID)
	}

	prdData, err := os.ReadFile(paths.PRDPath)
	if err != nil {
		return fmt.Errorf("read backlog epic PRD %q: %w", paths.PRDPath, err)
	}
	tasksData, err := os.ReadFile(paths.TasksPath)
	if err != nil {
		return fmt.Errorf("read backlog epic tasks %q: %w", paths.TasksPath, err)
	}

	rootDougDir := filepath.Join(projectRoot, ".doug")
	if err := state.AtomicWrite(filepath.Join(rootDougDir, prdFileName), prdData); err != nil {
		return fmt.Errorf("write root PRD: %w", err)
	}
	if err := state.AtomicWrite(filepath.Join(rootDougDir, tasksFileName), tasksData); err != nil {
		return fmt.Errorf("write root tasks: %w", err)
	}

	activatedAt := now.UTC().Format(time.RFC3339)
	metadata.Status = types.EpicStatusActive
	metadata.ActivatedAt = &activatedAt
	metadata.CompletedAt = nil
	if err := SaveEpicMetadata(paths.MetadataPath, metadata); err != nil {
		return err
	}

	return nil
}

func currentEpicActive(projectState *types.ProjectState) bool {
	if projectState == nil {
		return false
	}
	if strings.TrimSpace(projectState.CurrentEpic.ID) == "" {
		return false
	}
	if projectState.CurrentEpic.CompletedAt == nil {
		return true
	}
	return strings.TrimSpace(*projectState.CurrentEpic.CompletedAt) == ""
}
