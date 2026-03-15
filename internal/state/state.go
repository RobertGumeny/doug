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
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/types"
)

// ErrNotFound is returned by Load functions when the state file does not exist.
var ErrNotFound = errors.New("state file not found")

// ParseError is returned when a state file exists but cannot be unmarshalled.
// Fields is populated (from *yaml.TypeError) when a type mismatch is detected
// and identifies the specific offending fields. Hint carries optional
// formatting guidance for the user.
type ParseError struct {
	Path   string
	Err    error
	Fields []string // field-level messages extracted from *yaml.TypeError
	Hint   string   // optional formatting guidance note
}

func (e *ParseError) Error() string {
	var msg string
	if len(e.Fields) > 0 {
		msg = fmt.Sprintf("parse error in %s:\n  %s", e.Path, strings.Join(e.Fields, "\n  "))
	} else {
		msg = fmt.Sprintf("parse error in %s: %v", e.Path, e.Err)
	}
	if e.Hint != "" {
		msg += "\n" + e.Hint
	}
	return msg
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// tasksYAMLHint is the formatting guidance note appended to tasks.yaml parse errors.
const tasksYAMLHint = "Hint: tasks.yaml requires an 'epic' block with 'id', 'name', and 'tasks' list; " +
	"each task requires 'id', 'type' (feature), and 'status' (TODO|IN_PROGRESS|DONE|BLOCKED)"

// newTasksParseError builds a ParseError for tasks.yaml with field-level
// details (when the underlying error is *yaml.TypeError) and a formatting hint.
func newTasksParseError(path string, err error) *ParseError {
	pe := &ParseError{Path: path, Err: err, Hint: tasksYAMLHint}
	if typeErr, ok := err.(*yaml.TypeError); ok {
		pe.Fields = typeErr.Errors
	}
	return pe
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
	return AtomicWrite(path, data)
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
		return nil, newTasksParseError(path, err)
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
	return AtomicWrite(path, data)
}

// AtomicWrite writes data to path by first writing to path+".tmp",
// then calling os.Rename to replace the final target atomically.
func AtomicWrite(path string, data []byte) error {
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
