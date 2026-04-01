package types

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const ManifestSchemaVersionV1 = 1

// Manifest mirrors .doug/plan/manifest.yaml for schema version 1.
type Manifest struct {
	SchemaVersion int                  `yaml:"schema_version"`
	Project       ManifestProject      `yaml:"project"`
	Scaffold      ManifestScaffold     `yaml:"scaffold"`
	Dependencies  ManifestDependencies `yaml:"dependencies"`
	Constraints   []string             `yaml:"constraints"`
}

type ManifestProject struct {
	Name string `yaml:"name"`
	Mode string `yaml:"mode"`
}

type ManifestScaffold struct {
	Language       string `yaml:"language"`
	Runtime        string `yaml:"runtime"`
	Framework      string `yaml:"framework"`
	PackageManager string `yaml:"package_manager"`
	BuildSystem    string `yaml:"build_system"`
}

type ManifestDependencies struct {
	Runtime     []string `yaml:"runtime"`
	Development []string `yaml:"development"`
}

type rawManifest struct {
	SchemaVersion *int                  `yaml:"schema_version"`
	Project       *ManifestProject      `yaml:"project"`
	Scaffold      *ManifestScaffold     `yaml:"scaffold"`
	Dependencies  *ManifestDependencies `yaml:"dependencies"`
	Constraints   []string              `yaml:"constraints"`
}

// LoadManifest reads and validates a manifest.yaml file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}

	var raw rawManifest
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse manifest %q: %w", path, err)
	}

	if err := raw.validate(path); err != nil {
		return nil, err
	}

	manifest := &Manifest{
		SchemaVersion: *raw.SchemaVersion,
		Project:       *raw.Project,
		Scaffold:      *raw.Scaffold,
		Dependencies:  *raw.Dependencies,
		Constraints:   raw.Constraints,
	}
	if err := ValidateManifest(path, manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

// ValidateManifest validates an in-memory manifest using the same rules as
// LoadManifest so callers can verify derived data before writing it to disk.
func ValidateManifest(path string, manifest *Manifest) error {
	if manifest == nil {
		return fmt.Errorf("invalid manifest %q: manifest is nil", path)
	}

	raw := rawManifest{
		SchemaVersion: &manifest.SchemaVersion,
		Project:       &manifest.Project,
		Scaffold:      &manifest.Scaffold,
		Dependencies:  &manifest.Dependencies,
		Constraints:   manifest.Constraints,
	}
	return raw.validate(path)
}

func (m rawManifest) validate(path string) error {
	required := []struct {
		ok   bool
		name string
	}{
		{ok: m.SchemaVersion != nil, name: "schema_version"},
		{ok: m.Project != nil, name: "project"},
		{ok: m.Scaffold != nil, name: "scaffold"},
		{ok: m.Dependencies != nil, name: "dependencies"},
		{ok: m.Constraints != nil, name: "constraints"},
	}

	for _, field := range required {
		if !field.ok {
			return manifestFieldError(path, field.name)
		}
	}

	if *m.SchemaVersion != ManifestSchemaVersionV1 {
		return fmt.Errorf(
			"invalid manifest %q: unsupported schema_version %d (supported: %d)",
			path, *m.SchemaVersion, ManifestSchemaVersionV1,
		)
	}

	requiredStrings := []struct {
		value string
		name  string
	}{
		{value: m.Project.Name, name: "project.name"},
		{value: m.Project.Mode, name: "project.mode"},
		{value: m.Scaffold.Language, name: "scaffold.language"},
		{value: m.Scaffold.Runtime, name: "scaffold.runtime"},
		{value: m.Scaffold.Framework, name: "scaffold.framework"},
		{value: m.Scaffold.PackageManager, name: "scaffold.package_manager"},
		{value: m.Scaffold.BuildSystem, name: "scaffold.build_system"},
	}

	for _, field := range requiredStrings {
		if strings.TrimSpace(field.value) == "" {
			return manifestFieldError(path, field.name)
		}
	}

	requiredLists := []struct {
		value []string
		name  string
	}{
		{value: m.Dependencies.Runtime, name: "dependencies.runtime"},
		{value: m.Dependencies.Development, name: "dependencies.development"},
	}

	for _, field := range requiredLists {
		if field.value == nil {
			return manifestFieldError(path, field.name)
		}
	}

	return nil
}

func manifestFieldError(path, field string) error {
	return fmt.Errorf("invalid manifest %q: missing required field %q", path, field)
}
