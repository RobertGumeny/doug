package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// retiredPaths lists project-root-relative paths that are no longer part of
// the Pi-era workspace contract and should be flagged for removal.
var retiredPaths = []struct {
	rel  string
	desc string
}{
	{".codex", "pre-Pi provider directory; skills now live in .agents/skills/"},
	{".gemini", "pre-Pi provider directory; skills now live in .agents/skills/"},
}

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

// retiredExecutionFieldDescs maps the exact retired execution config field names
// found at the top level of .doug/doug.yaml to a brief human-readable label.
// Fields matching the "*_agent_command" suffix pattern are detected dynamically.
var retiredExecutionFieldDescs = map[string]string{
	"policy":           "execution routing hierarchy",
	"interaction_mode": "Pi interaction mode override",
	"execution_mode":   "stale execution mode selector",
}

// isRetiredExecutionField reports whether a top-level YAML key in doug.yaml
// is a retired execution config field that should be stripped during upgrade.
func isRetiredExecutionField(key string) bool {
	if _, ok := retiredExecutionFieldDescs[key]; ok {
		return true
	}
	return strings.HasSuffix(key, "_agent_command")
}

// inspectConfigDrift checks .doug/doug.yaml for retired execution config fields.
// Any of: policy, interaction_mode, execution_mode, or *_agent_command at the
// top level are flagged as retired. Doug source code now owns execution routing
// and does not read any of these fields from config.
// A missing config file is not an error — it is handled by the init guard.
func inspectConfigDrift(dougDir string) ([]driftItem, error) {
	configPath := filepath.Join(dougDir, "doug.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	var retiredFound []string
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		mapping := doc.Content[0]
		if mapping.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(mapping.Content); i += 2 {
				key := mapping.Content[i].Value
				if isRetiredExecutionField(key) {
					retiredFound = append(retiredFound, key+":")
				}
			}
		}
	}

	if len(retiredFound) == 0 {
		return nil, nil
	}

	desc := fmt.Sprintf(
		"retired execution config fields (%s) — Doug now uses Pi exclusively "+
			"with source-owned workflow routing; these fields will be removed",
		strings.Join(retiredFound, ", "),
	)
	return []driftItem{{
		Kind:        driftMissingConfig,
		AbsPath:     configPath,
		DisplayPath: ".doug/doug.yaml",
		Description: desc,
		Action:      actionStripConfig,
	}}, nil
}

// stripRetiredExecutionConfig reads .doug/doug.yaml, removes all top-level
// retired execution config fields (policy, interaction_mode, execution_mode,
// and any *_agent_command fields), and writes the cleaned YAML back.
// Core project settings (build_system, max_retries, etc.) are preserved.
func stripRetiredExecutionConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		doc.Content[0] = removeRetiredExecutionFields(doc.Content[0])
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(configPath, out, 0o644)
}

// removeRetiredExecutionFields returns a shallow copy of node with all
// retired execution config key-value pairs omitted. Only the top-level
// mapping is modified; nested nodes are left intact.
func removeRetiredExecutionFields(node *yaml.Node) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return node
	}
	var kept []*yaml.Node
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		if isRetiredExecutionField(keyNode.Value) {
			continue
		}
		kept = append(kept, keyNode, valNode)
	}
	result := *node
	result.Content = kept
	return &result
}

// inspectManagedSurfaces checks canonical .agents skills and the Pi extension
// against the current embedded templates. Merge entries (.gitignore, AGENTS.md)
// are always idempotently re-merged by their own strategy and are not inspected.
func inspectManagedSurfaces(projectRoot string) ([]driftItem, error) {
	entries, err := buildInstallPlan(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("build install plan: %w", err)
	}

	var items []driftItem

	for _, e := range entries {
		if e.Kind != entryKindCopy {
			continue
		}
		if !isManagedInstallPath(e.DisplayRel) {
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

func isManagedInstallPath(rel string) bool {
	return rel == filepath.Join(".pi", "extensions", "handoff.ts") ||
		strings.HasPrefix(rel, filepath.Join(".agents", "skills")+string(filepath.Separator))
}
