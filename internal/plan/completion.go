package plan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

const runtimeArchiveDirName = "epics"

// FinalizeEpicCompletion archives the executed root-level runtime snapshot and,
// when the epic originated from backlog planning, propagates the terminal
// completion state back into backlog metadata.
func FinalizeEpicCompletion(projectRoot string, currentEpic types.EpicState, completedAt string) (string, error) {
	archiveDir, err := archiveRuntimeSnapshot(projectRoot, currentEpic.ID)
	if err != nil {
		return "", err
	}

	paths := NewEpicPackagePaths(projectRoot, currentEpic.ID)
	metadata, err := LoadEpicMetadata(paths.MetadataPath)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return archiveDir, nil
		}
		return archiveDir, err
	}

	if metadata.EpicID != currentEpic.ID {
		return archiveDir, fmt.Errorf(
			"backlog epic metadata mismatch for %q: metadata declares %q",
			currentEpic.ID,
			metadata.EpicID,
		)
	}
	if metadata.Status != types.EpicStatusActive {
		return archiveDir, fmt.Errorf(
			"cannot finalize backlog epic %q from runtime state: metadata status is %q, expected %q",
			currentEpic.ID,
			metadata.Status,
			types.EpicStatusActive,
		)
	}

	completed := completedAt
	metadata.Status = types.EpicStatusCompleted
	metadata.CompletedAt = &completed
	if err := SaveEpicMetadata(paths.MetadataPath, metadata); err != nil {
		return archiveDir, err
	}

	return archiveDir, nil
}

func archiveRuntimeSnapshot(projectRoot, epicID string) (string, error) {
	dougDir := filepath.Join(projectRoot, ".doug")
	archiveDir := filepath.Join(dougDir, "logs", runtimeArchiveDirName, epicID)
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return "", fmt.Errorf("create runtime archive directory %q: %w", archiveDir, err)
	}

	files := []string{"PRD.md", "tasks.yaml", "project-state.yaml", "ACTIVE_TASK.md"}
	for _, name := range files {
		src := filepath.Join(dougDir, name)
		data, err := os.ReadFile(src)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) && name == "ACTIVE_TASK.md" {
				continue
			}
			return "", fmt.Errorf("read runtime snapshot file %q: %w", src, err)
		}
		if err := state.AtomicWrite(filepath.Join(archiveDir, name), data); err != nil {
			return "", fmt.Errorf("write runtime archive file %q: %w", filepath.Join(archiveDir, name), err)
		}
	}

	stamp := time.Now().UTC().Format(time.RFC3339) + "\n"
	if err := state.AtomicWrite(filepath.Join(archiveDir, "archived_at.txt"), []byte(stamp)); err != nil {
		return "", fmt.Errorf("write runtime archive timestamp: %w", err)
	}

	return archiveDir, nil
}
