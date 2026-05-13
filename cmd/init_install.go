package cmd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/templates"
)

// entryKind classifies the merge strategy for a single install entry.
type entryKind int

const (
	// entryKindCopy writes the template bytes verbatim; respects the force flag.
	entryKindCopy entryKind = iota
	// entryKindMergeJSON deep-merges the template into an existing JSON file.
	// With force the template is written directly.
	entryKindMergeJSON
	// entryKindMergeGitignore appends missing non-comment lines to .gitignore.
	// Always merges — force is ignored.
	entryKindMergeGitignore
	// entryKindMergeAgentsMD appends or updates the managed doug block in
	// AGENTS.md. Always merges — force is ignored.
	entryKindMergeAgentsMD
	// entryKindMergeCodexTOML injects managed key/section defaults into an
	// existing .codex/config.toml. With force the template is written directly.
	entryKindMergeCodexTOML
)

// installEntry is a single file-install operation produced by buildInstallPlan.
// It carries pre-read template bytes so execution touches only the destination.
type installEntry struct {
	DstPath    string    // absolute filesystem destination
	DisplayRel string    // path shown in terminal output (relative to project root)
	Kind       entryKind // merge strategy
	Data       []byte    // pre-processed template bytes

	// projectID and projectName are only populated for entryKindMergeAgentsMD.
	projectID   string
	projectName string
}

// buildInstallPlan walks the embedded init/ FS and resolves all template
// routing rules into an ordered list of installEntry values. It reads and
// pre-processes template bytes but does not touch the filesystem.
//
// AGENTS.md project metadata (DOUG_PROJECT_ID / DOUG_PROJECT_NAME) is resolved
// at plan-build time by reading the existing AGENTS.md (if any) at dir.
func buildInstallPlan(dir string, agentSelected map[string]bool, buildSystem string) ([]installEntry, error) {
	// Pre-resolve AGENTS.md project metadata so it can be embedded in the entry.
	existingAgentsMD, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	existingStr := string(existingAgentsMD)
	projectID := extractManagedBlockField(existingStr, "DOUG_PROJECT_ID")
	if projectID == "" {
		projectID = generateProjectID(filepath.Base(dir))
	}
	projectName := extractManagedBlockField(existingStr, "DOUG_PROJECT_NAME")
	if projectName == "" {
		projectName = generateProjectName(filepath.Base(dir))
	}

	var entries []installEntry

	walkErr := fs.WalkDir(templates.Init, "init", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(path, "init/")

		newEntries, routeErr := routeTemplateFile(dir, rel, path, agentSelected, buildSystem, projectID, projectName)
		if routeErr != nil {
			return routeErr
		}
		entries = append(entries, newEntries...)
		return nil
	})

	return entries, walkErr
}

// routeTemplateFile maps a single embedded template file to zero or more
// installEntry values. It reads template bytes from the embedded FS.
func routeTemplateFile(
	dir, rel, srcPath string,
	agentSelected map[string]bool,
	buildSystem, projectID, projectName string,
) ([]installEntry, error) {
	switch {
	case strings.HasPrefix(rel, ".claude/"):
		if !agentSelected["claude"] {
			return nil, nil
		}
		return agentSettingsEntries(dir, rel, srcPath, buildSystem)

	case strings.HasPrefix(rel, ".codex/"):
		if !agentSelected["codex"] {
			return nil, nil
		}
		return agentSettingsEntries(dir, rel, srcPath, "")

	case strings.HasPrefix(rel, ".gemini/"):
		if !agentSelected["gemini"] {
			return nil, nil
		}
		return agentSettingsEntries(dir, rel, srcPath, "")

	case strings.HasPrefix(rel, ".pi/"):
		return agentSettingsEntries(dir, rel, srcPath, "")

	case strings.HasPrefix(rel, "skills/"):
		skillRel := strings.TrimPrefix(rel, "skills/")
		data, err := templates.Init.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", srcPath, err)
		}
		var entries []installEntry
		for _, dst := range selectedSkillDestinations(dir, agentSelected, skillRel) {
			dstRel, _ := filepath.Rel(dir, dst)
			entries = append(entries, installEntry{
				DstPath:    dst,
				DisplayRel: dstRel,
				Kind:       entryKindCopy,
				Data:       data,
			})
		}
		return entries, nil

	case rel == ".gitignore":
		data, err := templates.Init.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", srcPath, err)
		}
		return []installEntry{{
			DstPath:    filepath.Join(dir, ".gitignore"),
			DisplayRel: ".gitignore",
			Kind:       entryKindMergeGitignore,
			Data:       data,
		}}, nil

	case rel == "AGENTS.md":
		data, err := templates.Init.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", srcPath, err)
		}
		// Substitute project identity placeholders in the template section.
		sectionStr := strings.ReplaceAll(string(data), "{{DOUG_PROJECT_ID}}", projectID)
		sectionStr = strings.ReplaceAll(sectionStr, "{{DOUG_PROJECT_NAME}}", projectName)
		return []installEntry{{
			DstPath:     filepath.Join(dir, "AGENTS.md"),
			DisplayRel:  "AGENTS.md",
			Kind:        entryKindMergeAgentsMD,
			Data:        []byte(sectionStr),
			projectID:   projectID,
			projectName: projectName,
		}}, nil

	case rel == "CLAUDE.md":
		data, err := templates.Init.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", srcPath, err)
		}
		return []installEntry{{
			DstPath:    filepath.Join(dir, "CLAUDE.md"),
			DisplayRel: "CLAUDE.md",
			Kind:       entryKindCopy,
			Data:       data,
		}}, nil

	case strings.HasSuffix(rel, "_TEMPLATE.md"):
		data, err := templates.Init.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", srcPath, err)
		}
		return []installEntry{{
			DstPath:    filepath.Join(dir, ".doug", "logs", rel),
			DisplayRel: filepath.Join(".doug", "logs", rel),
			Kind:       entryKindCopy,
			Data:       data,
		}}, nil

	default:
		log.Warning(fmt.Sprintf("skipping unknown template file: %s", rel))
		return nil, nil
	}
}

// agentSettingsEntries returns the installEntry for a single agent settings
// file (under .claude/, .codex/, or .gemini/). For .claude/settings.json,
// build-system permissions are injected into the template before the entry is
// built. buildSystem is empty for non-Claude providers.
func agentSettingsEntries(dir, rel, srcPath, buildSystem string) ([]installEntry, error) {
	data, err := templates.Init.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", srcPath, err)
	}
	if rel == ".claude/settings.json" && buildSystem != "" {
		var injectErr error
		data, injectErr = injectBuildSystemPermissions(data, buildSystem)
		if injectErr != nil {
			log.Warning(fmt.Sprintf("could not inject build-system permissions: %v — proceeding with unmodified template", injectErr))
		}
	}
	return []installEntry{{
		DstPath:    filepath.Join(dir, rel),
		DisplayRel: rel,
		Kind:       agentSettingsKind(rel),
		Data:       data,
	}}, nil
}

// agentSettingsKind returns the merge strategy for a provider settings file
// based on its extension.
func agentSettingsKind(rel string) entryKind {
	switch {
	case strings.HasSuffix(rel, ".json"):
		return entryKindMergeJSON
	case strings.HasSuffix(rel, ".toml"):
		return entryKindMergeCodexTOML
	default:
		return entryKindCopy
	}
}

// selectedSkillDestinations returns the absolute destination paths for a skill
// file relative path for each selected agent provider. Pi skills are always
// included to support both the Pi RPC execution backend (when execution_mode: rpc
// is configured in doug.yaml) and interactive Pi companion sessions.
func selectedSkillDestinations(dir string, agentSelected map[string]bool, skillRel string) []string {
	providers := []struct {
		name   string
		root   string
		always bool
	}{
		{name: "claude", root: ".claude"},
		{name: "codex", root: ".codex"},
		{name: "gemini", root: ".gemini"},
		{name: "pi", root: ".pi", always: true},
	}

	var destinations []string
	for _, provider := range providers {
		if provider.always || agentSelected[provider.name] {
			destinations = append(destinations, filepath.Join(dir, provider.root, "skills", skillRel))
		}
	}
	return destinations
}

// executeInstallPlan applies each entry in order to the filesystem.
func executeInstallPlan(w io.Writer, dir string, entries []installEntry, force bool) error {
	for _, e := range entries {
		if mkErr := os.MkdirAll(filepath.Dir(e.DstPath), 0o755); mkErr != nil {
			return fmt.Errorf("create directory for %s: %w", e.DisplayRel, mkErr)
		}
		if err := applyInstallEntry(w, e, force); err != nil {
			return err
		}
	}
	return nil
}

// applyInstallEntry dispatches to the merge-strategy-specific apply function.
func applyInstallEntry(w io.Writer, e installEntry, force bool) error {
	switch e.Kind {
	case entryKindCopy:
		return applyEntryCopy(w, e, force)
	case entryKindMergeJSON:
		return applyEntryMergeJSON(w, e, force)
	case entryKindMergeGitignore:
		return applyEntryMergeGitignore(w, e)
	case entryKindMergeAgentsMD:
		return applyEntryMergeAgentsMD(w, e)
	case entryKindMergeCodexTOML:
		return applyEntryMergeCodexTOML(w, e, force)
	default:
		log.Warning(fmt.Sprintf("unknown install entry kind for %s — skipping", e.DisplayRel))
		return nil
	}
}

func applyEntryCopy(w io.Writer, e installEntry, force bool) error {
	if !force {
		if _, statErr := os.Stat(e.DstPath); statErr == nil {
			log.Warning(fmt.Sprintf("%s already exists — skipping (use --force to overwrite)", e.DstPath))
			return nil
		}
	}
	if writeErr := state.AtomicWrite(e.DstPath, e.Data); writeErr != nil {
		return fmt.Errorf("write %s: %w", e.DisplayRel, writeErr)
	}
	writef(w, "  ✓ %s\n", e.DisplayRel)
	log.Success(fmt.Sprintf("created %s", e.DstPath))
	return nil
}

func applyEntryMergeJSON(w io.Writer, e installEntry, force bool) error {
	if force {
		if writeErr := state.AtomicWrite(e.DstPath, e.Data); writeErr != nil {
			return fmt.Errorf("write %s: %w", e.DisplayRel, writeErr)
		}
		writef(w, "  ✓ %s\n", e.DisplayRel)
		log.Success(fmt.Sprintf("created %s", e.DstPath))
		return nil
	}

	existing, readErr := os.ReadFile(e.DstPath)
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return fmt.Errorf("read %s: %w", e.DisplayRel, readErr)
		}
		if writeErr := state.AtomicWrite(e.DstPath, e.Data); writeErr != nil {
			return fmt.Errorf("write %s: %w", e.DisplayRel, writeErr)
		}
		writef(w, "  ✓ %s\n", e.DisplayRel)
		log.Success(fmt.Sprintf("created %s", e.DstPath))
		return nil
	}

	merged, mergeErr := mergeJSONSettings(existing, e.Data)
	if mergeErr != nil {
		log.Warning(fmt.Sprintf("%s exists but merge failed (%v) — skipping (use --force to overwrite)", e.DstPath, mergeErr))
		return nil
	}
	if writeErr := state.AtomicWrite(e.DstPath, merged); writeErr != nil {
		return fmt.Errorf("write %s: %w", e.DisplayRel, writeErr)
	}
	writef(w, "  ✓ %s\n", e.DisplayRel)
	log.Success(fmt.Sprintf("merged managed settings into %s", e.DstPath))
	return nil
}

func applyEntryMergeGitignore(w io.Writer, e installEntry) error {
	existing, readErr := os.ReadFile(e.DstPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read %s: %w", e.DisplayRel, readErr)
	}

	merged := mergeGitignore(string(existing), string(e.Data))
	if writeErr := state.AtomicWrite(e.DstPath, []byte(merged)); writeErr != nil {
		return fmt.Errorf("write %s: %w", e.DisplayRel, writeErr)
	}
	writef(w, "  ✓ %s\n", e.DisplayRel)
	if os.IsNotExist(readErr) {
		log.Success(fmt.Sprintf("created %s", e.DstPath))
	} else {
		log.Success(fmt.Sprintf("updated %s", e.DstPath))
	}
	return nil
}

func applyEntryMergeAgentsMD(w io.Writer, e installEntry) error {
	existing, readErr := os.ReadFile(e.DstPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read %s: %w", e.DisplayRel, readErr)
	}
	existingStr := string(existing)

	merged := mergeAgents(existingStr, string(e.Data), e.projectID, e.projectName)
	if writeErr := state.AtomicWrite(e.DstPath, []byte(merged)); writeErr != nil {
		return fmt.Errorf("write %s: %w", e.DisplayRel, writeErr)
	}

	switch {
	case os.IsNotExist(readErr):
		writef(w, "  ✓ %s\n", e.DisplayRel)
		log.Success(fmt.Sprintf("created %s", e.DstPath))
	case normalizeText(existingStr) == merged:
		log.Success(fmt.Sprintf("kept %s", e.DstPath))
	default:
		writef(w, "  ✓ %s\n", e.DisplayRel)
		log.Success(fmt.Sprintf("updated %s", e.DstPath))
	}
	return nil
}

func applyEntryMergeCodexTOML(w io.Writer, e installEntry, force bool) error {
	if force {
		if writeErr := state.AtomicWrite(e.DstPath, e.Data); writeErr != nil {
			return fmt.Errorf("write %s: %w", e.DisplayRel, writeErr)
		}
		writef(w, "  ✓ %s\n", e.DisplayRel)
		log.Success(fmt.Sprintf("created %s", e.DstPath))
		return nil
	}

	existing, readErr := os.ReadFile(e.DstPath)
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return fmt.Errorf("read %s: %w", e.DisplayRel, readErr)
		}
		if writeErr := state.AtomicWrite(e.DstPath, e.Data); writeErr != nil {
			return fmt.Errorf("write %s: %w", e.DisplayRel, writeErr)
		}
		writef(w, "  ✓ %s\n", e.DisplayRel)
		log.Success(fmt.Sprintf("created %s", e.DstPath))
		return nil
	}

	merged := []byte(mergeCodexConfigTOML(string(existing)))
	if writeErr := state.AtomicWrite(e.DstPath, merged); writeErr != nil {
		return fmt.Errorf("write %s: %w", e.DisplayRel, writeErr)
	}
	writef(w, "  ✓ %s\n", e.DisplayRel)
	log.Success(fmt.Sprintf("merged managed settings into %s", e.DstPath))
	return nil
}
