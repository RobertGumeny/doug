package plan

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

const (
	backlogPlanDirName  = "plan"
	backlogEpicsDirName = "epics"
	prdFileName         = "PRD.md"
	tasksFileName       = "tasks.yaml"
	metadataFileName    = "metadata.yaml"
)

// EpicPackagePaths captures the deterministic backlog epic package layout under
// .doug/plan/epics/<EPIC-ID>/.
type EpicPackagePaths struct {
	PlanDir      string
	EpicsDir     string
	EpicDir      string
	PRDPath      string
	TasksPath    string
	MetadataPath string
}

type rawEpicMetadata struct {
	EpicID         *string                    `yaml:"epic_id"`
	Status         *types.EpicLifecycleStatus `yaml:"status"`
	CreatedAt      *string                    `yaml:"created_at"`
	SourcePlanPath *string                    `yaml:"source_plan_path"`
	ActivatedAt    *string                    `yaml:"activated_at"`
	CompletedAt    *string                    `yaml:"completed_at"`
}

// NewEpicPackagePaths derives the deterministic package paths for a backlog epic.
func NewEpicPackagePaths(projectRoot, epicID string) EpicPackagePaths {
	planDir := filepath.Join(projectRoot, ".doug", backlogPlanDirName)
	epicsDir := filepath.Join(planDir, backlogEpicsDirName)
	epicDir := filepath.Join(epicsDir, epicID)
	return EpicPackagePaths{
		PlanDir:      planDir,
		EpicsDir:     epicsDir,
		EpicDir:      epicDir,
		PRDPath:      filepath.Join(epicDir, prdFileName),
		TasksPath:    filepath.Join(epicDir, tasksFileName),
		MetadataPath: filepath.Join(epicDir, metadataFileName),
	}
}

// LoadEpicMetadata reads metadata.yaml for a backlog epic package and validates
// the required fields plus lifecycle status values.
func LoadEpicMetadata(path string) (*types.EpicMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, state.ErrNotFound
		}
		return nil, fmt.Errorf("read epic metadata %q: %w", path, err)
	}

	var raw rawEpicMetadata
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse epic metadata %q: %w", path, err)
	}

	if err := raw.validate(path); err != nil {
		return nil, err
	}

	return &types.EpicMetadata{
		EpicID:         strings.TrimSpace(*raw.EpicID),
		Status:         *raw.Status,
		CreatedAt:      strings.TrimSpace(*raw.CreatedAt),
		SourcePlanPath: strings.TrimSpace(*raw.SourcePlanPath),
		ActivatedAt:    trimOptionalString(raw.ActivatedAt),
		CompletedAt:    trimOptionalString(raw.CompletedAt),
	}, nil
}

// SaveEpicMetadata validates and atomically writes metadata.yaml.
func SaveEpicMetadata(path string, metadata *types.EpicMetadata) error {
	if metadata == nil {
		return fmt.Errorf("invalid epic metadata %q: metadata is nil", path)
	}
	if err := validateEpicMetadata(path, metadata); err != nil {
		return err
	}

	data, err := yaml.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal epic metadata %q: %w", path, err)
	}
	return state.AtomicWrite(path, data)
}

func (m rawEpicMetadata) validate(path string) error {
	required := []struct {
		ok   bool
		name string
	}{
		{ok: m.EpicID != nil, name: "epic_id"},
		{ok: m.Status != nil, name: "status"},
		{ok: m.CreatedAt != nil, name: "created_at"},
		{ok: m.SourcePlanPath != nil, name: "source_plan_path"},
	}

	for _, field := range required {
		if !field.ok {
			return epicMetadataFieldError(path, field.name)
		}
	}

	metadata := &types.EpicMetadata{
		EpicID:         strings.TrimSpace(*m.EpicID),
		Status:         *m.Status,
		CreatedAt:      strings.TrimSpace(*m.CreatedAt),
		SourcePlanPath: strings.TrimSpace(*m.SourcePlanPath),
		ActivatedAt:    trimOptionalString(m.ActivatedAt),
		CompletedAt:    trimOptionalString(m.CompletedAt),
	}
	return validateEpicMetadata(path, metadata)
}

func validateEpicMetadata(path string, metadata *types.EpicMetadata) error {
	requiredStrings := []struct {
		value string
		name  string
	}{
		{value: metadata.EpicID, name: "epic_id"},
		{value: metadata.CreatedAt, name: "created_at"},
		{value: metadata.SourcePlanPath, name: "source_plan_path"},
	}

	for _, field := range requiredStrings {
		if strings.TrimSpace(field.value) == "" {
			return epicMetadataFieldError(path, field.name)
		}
	}

	if !metadata.Status.IsValid() {
		return fmt.Errorf(
			"invalid epic metadata %q: unsupported status %q (supported: %s, %s, %s)",
			path,
			metadata.Status,
			types.EpicStatusPlanned,
			types.EpicStatusActive,
			types.EpicStatusCompleted,
		)
	}

	return nil
}

func epicMetadataFieldError(path, field string) error {
	return fmt.Errorf("invalid epic metadata %q: missing required field %q", path, field)
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}
