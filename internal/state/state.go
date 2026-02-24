// Package state provides atomic load and save operations for the two
// orchestrator state files: project-state.yaml and tasks.yaml.
//
// All writes are atomic: data is marshalled to a .tmp file in the same
// directory, then os.Rename replaces the target in a single kernel call.
// This prevents partial writes from corrupting state.
package state

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/types"
)

// ErrNotFound is returned by Load functions when the state file does not exist.
var ErrNotFound = errors.New("state file not found")

// ParseError is returned when a state file exists but cannot be unmarshalled.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error in %s: %v", e.Path, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// LoadProjectState reads project-state.yaml at path into a ProjectState.
// Returns ErrNotFound if the file is absent, or *ParseError on malformed YAML.
func LoadProjectState(path string) (*types.ProjectState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var state types.ProjectState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, &ParseError{Path: path, Err: err}
	}
	return &state, nil
}

// SaveProjectState atomically writes state to path.
// It writes to path+".tmp" first, then renames to path.
func SaveProjectState(path string, state *types.ProjectState) error {
	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal project state: %w", err)
	}
	return atomicWrite(path, data)
}

// LoadTasks reads tasks.yaml at path into a Tasks struct.
// Returns ErrNotFound if the file is absent, or *ParseError on malformed YAML.
// The UserDefined field on every loaded Task is set to true, establishing the
// UserDefined vs Synthetic distinction at the type level.
func LoadTasks(path string) (*types.Tasks, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var tasks types.Tasks
	if err := yaml.Unmarshal(data, &tasks); err != nil {
		return nil, &ParseError{Path: path, Err: err}
	}

	for i := range tasks.Epic.Tasks {
		tasks.Epic.Tasks[i].UserDefined = true
	}
	return &tasks, nil
}

// SaveTasks atomically writes tasks to path.
// It writes to path+".tmp" first, then renames to path.
func SaveTasks(path string, tasks *types.Tasks) error {
	data, err := yaml.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}
	return atomicWrite(path, data)
}

// atomicWrite writes data to path by first writing to path+".tmp",
// then calling os.Rename to replace the final target atomically.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp file %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp) // best-effort cleanup on rename failure
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
