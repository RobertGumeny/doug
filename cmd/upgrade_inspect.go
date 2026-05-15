package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/config"
)

// retiredPaths lists project-root-relative paths that are no longer part of
// the Pi-era workspace contract and should be flagged for removal.
var retiredPaths = []struct {
	rel  string
	desc string
}{
	{".claude", "pre-Pi provider directory; skills now live in .pi/skills/"},
	{".codex", "pre-Pi provider directory; skills now live in .pi/skills/"},
	{".gemini", "pre-Pi provider directory; skills now live in .pi/skills/"},
}

// requiredPhases lists the Doug workflow phases that must carry
// execution_mode: rpc in a Pi-era .doug/doug.yaml.
var requiredPhases = []string{"runtime", "planning", "scaffold", "research", "post_epic_kb"}

// inspectWorkspace runs all inspection stages and returns the combined
// drift items in the order: retired artifacts, config drift, managed surfaces.
func inspectWorkspace(projectRoot, dougDir string) ([]driftItem, error) {
	var items []driftItem

	retired, err := inspectRetiredArtifacts(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("retired artifacts: %w", err)
	}
	items = append(items, retired...)

	cfgDrift, err := inspectConfigDrift(dougDir)
	if err != nil {
		return nil, fmt.Errorf("config drift: %w", err)
	}
	items = append(items, cfgDrift...)

	managed, err := inspectManagedSurfaces(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("managed surfaces: %w", err)
	}
	items = append(items, managed...)

	return items, nil
}

// inspectRetiredArtifacts checks for provider-specific directories that were
// used in the pre-Pi era but are no longer part of the workspace contract.
func inspectRetiredArtifacts(projectRoot string) ([]driftItem, error) {
	var items []driftItem
	for _, rp := range retiredPaths {
		abs := filepath.Join(projectRoot, rp.rel)
		if _, err := os.Stat(abs); err == nil {
			items = append(items, driftItem{
				Kind:        driftRetiredArtifact,
				AbsPath:     abs,
				DisplayPath: rp.rel,
				Description: rp.desc,
				Action:      actionRemove,
			})
		}
	}
	return items, nil
}

// configSnapshot is an exact-presence parser for .doug/doug.yaml. It avoids
// using OrchestratorConfig (which fills in defaults) so absent fields are
// distinguishable from explicitly-set zero values.
type configSnapshot struct {
	Policy *policySnapshot `yaml:"policy"`
}

type policySnapshot struct {
	Phases map[string]phasePolicySnapshot `yaml:"phases"`
}

type phasePolicySnapshot struct {
	ExecutionMode string `yaml:"execution_mode"`
}

// inspectConfigDrift checks .doug/doug.yaml for missing Pi-era policy fields.
// A missing file is not an error — it is handled by the init guard.
func inspectConfigDrift(dougDir string) ([]driftItem, error) {
	configPath := filepath.Join(dougDir, "doug.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var snap configSnapshot
	if err := yaml.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	var items []driftItem

	if snap.Policy == nil || len(snap.Policy.Phases) == 0 {
		items = append(items, driftItem{
			Kind:        driftMissingConfig,
			AbsPath:     configPath,
			DisplayPath: ".doug/doug.yaml",
			Description: "policy.phases block is absent — add execution_mode: rpc for all phases to activate Pi execution",
			Action:      actionPatch,
		})
		return items, nil
	}

	for _, phase := range requiredPhases {
		pp, ok := snap.Policy.Phases[phase]
		if !ok || pp.ExecutionMode != config.ExecutionModeRPC {
			items = append(items, driftItem{
				Kind:        driftMissingConfig,
				AbsPath:     configPath,
				DisplayPath: ".doug/doug.yaml",
				Description: fmt.Sprintf("policy.phases.%s missing execution_mode: rpc", phase),
				Action:      actionPatch,
			})
		}
	}

	return items, nil
}

// inspectManagedSurfaces checks .pi/ targets against the current embedded init
// templates. Only entryKindCopy entries under .pi/ are checked — merge entries
// (.gitignore, AGENTS.md) are always idempotently re-merged by their own strategy
// and do not need drift detection.
func inspectManagedSurfaces(projectRoot string) ([]driftItem, error) {
	entries, err := buildInstallPlan(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("build install plan: %w", err)
	}

	piDir := filepath.Join(projectRoot, ".pi")
	var items []driftItem

	for _, e := range entries {
		if e.Kind != entryKindCopy {
			continue
		}
		if !strings.HasPrefix(e.DstPath, piDir) {
			continue
		}

		existing, readErr := os.ReadFile(e.DstPath)
		if os.IsNotExist(readErr) {
			items = append(items, driftItem{
				Kind:        driftMissingManaged,
				AbsPath:     e.DstPath,
				DisplayPath: e.DisplayRel,
				Description: "managed surface is absent",
				Action:      actionReinstall,
			})
			continue
		}
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", e.DstPath, readErr)
		}
		if !bytes.Equal(existing, e.Data) {
			items = append(items, driftItem{
				Kind:        driftOutdatedManaged,
				AbsPath:     e.DstPath,
				DisplayPath: e.DisplayRel,
				Description: "managed surface differs from current embedded template",
				Action:      actionReinstall,
			})
		}
	}

	return items, nil
}
