package cmd

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/templates"
)

const dougInstructionsMarker = "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->"
const dougInstructionsEndMarker = "<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->"

var initFlags struct {
	force       bool
	buildSystem string
	agents      string // comma-separated agent names (non-interactive override)
	noGitInit   bool
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new doug project",
	Long:  "Scaffold a new doug project with .doug/doug.yaml, .doug/tasks.yaml, and .doug/PRD.md.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initFlags.force, "force", false, "Overwrite existing files")
	initCmd.Flags().StringVar(&initFlags.buildSystem, "build-system", "", "Build system to use (go|npm|pnpm); auto-detected if not set")
	initCmd.Flags().StringVar(&initFlags.agents, "agents", "", "Comma-separated agent names to install skills for (e.g. claude,codex)")
	initCmd.Flags().BoolVar(&initFlags.noGitInit, "no-git-init", false, "Skip running git init")
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Determine selected agents: flag > interactive TTY > default.
	var selectedAgents []string
	if initFlags.agents != "" {
		for _, a := range strings.Split(initFlags.agents, ",") {
			if a = strings.TrimSpace(a); a != "" {
				selectedAgents = append(selectedAgents, a)
			}
		}
	} else {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			selectedAgents = promptAgentSelection()
		} else {
			selectedAgents = []string{"claude"}
		}
	}
	if len(selectedAgents) == 0 {
		selectedAgents = []string{"claude"}
	}

	return initProject(dir, initFlags.force, initFlags.buildSystem, selectedAgents, initFlags.noGitInit)
}

// promptAgentSelection shows an interactive agent selection menu on a TTY.
// Returns the selected agent names; defaults to ["claude"] on empty input.
func promptAgentSelection() []string {
	options := []string{"claude", "codex", "gemini"}

	writeln(os.Stdout, "Which agent(s) are you using? (comma-separated numbers, or press Enter for Claude)")
	for i, name := range options {
		marker := "[ ]"
		if i == 0 {
			marker = "[x]"
		}
		writef(os.Stdout, "  %d. %s %s\n", i+1, marker, name)
	}
	writef(os.Stdout, "Selection (e.g. 1,2): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return []string{"claude"}
	}
	input = strings.TrimSpace(input)

	if input == "" {
		return []string{"claude"}
	}

	var selected []string
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > len(options) {
			continue
		}
		selected = append(selected, options[n-1])
	}
	if len(selected) == 0 {
		return []string{"claude"}
	}
	return selected
}

// promptBuildSystemSelection shows an interactive build system selection menu on a TTY.
// Returns the selected build system name; defaults to "go" on empty or invalid input.
func promptBuildSystemSelection() string {
	options := []string{"go", "npm", "pnpm", "static"}
	writeln(os.Stdout, "No build system detected. Which build system does this project use?")
	for i, name := range options {
		writef(os.Stdout, "  %d. %s\n", i+1, name)
	}
	writef(os.Stdout, "Selection (1-4, or press Enter for go): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(input) == "" {
		return "go"
	}
	n, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || n < 1 || n > len(options) {
		return "go"
	}
	return options[n-1]
}

// injectBuildSystemPermissions appends build-system-specific Bash permissions
// to the "permissions.allow" array in the settings.json template. Returns the
// template unchanged if bs is empty, not in the BuildSystems registry, or has
// no permissions defined. Returns an error only when the template JSON is malformed.
func injectBuildSystemPermissions(template []byte, bs string) ([]byte, error) {
	info, ok := config.BuildSystems[bs]
	if !ok || len(info.Permissions) == 0 {
		return template, nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(template, &obj); err != nil {
		return nil, err
	}

	// Navigate/create permissions.allow.
	permsVal := obj["permissions"]
	permsMap, _ := permsVal.(map[string]interface{})
	if permsMap == nil {
		permsMap = make(map[string]interface{})
		obj["permissions"] = permsMap
	}

	allowVal := permsMap["allow"]
	allowArr, _ := allowVal.([]interface{})

	toAdd := make([]interface{}, len(info.Permissions))
	for i, p := range info.Permissions {
		toAdd[i] = p
	}

	merged, _ := mergeStringArrays(allowArr, toAdd)
	permsMap["allow"] = merged

	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// initProject is the testable core of the init command. It generates the .doug/
// directory with doug.yaml, project-state.yaml, tasks.yaml, and PRD.md.
// selectedAgents controls which agent skill directories are populated.
func initProject(dir string, force bool, buildSystem string, selectedAgents []string, noGitInit bool) error {
	dougDir := filepath.Join(dir, ".doug")

	// Guard: refuse to re-initialize an existing project unless --force is set.
	if !force {
		if _, statErr := os.Stat(filepath.Join(dougDir, "project-state.yaml")); statErr == nil {
			return fmt.Errorf(".doug/project-state.yaml already exists — project appears to be already initialized; use --force to overwrite")
		}
	}

	// Ensure .doug/ directory exists.
	if err := os.MkdirAll(dougDir, 0o755); err != nil {
		return fmt.Errorf("create .doug directory: %w", err)
	}

	// Validate explicit --build-system flag before doing any work.
	if buildSystem != "" {
		switch buildSystem {
		case "go", "npm", "pnpm", "static":
		default:
			return fmt.Errorf("unsupported build system %q: must be one of: go, npm, pnpm, static", buildSystem)
		}
	}

	// Determine the build system: flag > auto-detect > prompt (TTY) > fallback.
	bs := buildSystem
	if bs == "" {
		bs = config.DetectBuildSystem(dir) // returns "" when no marker files found
	}

	claudeSelected := false
	for _, a := range selectedAgents {
		if strings.ToLower(strings.TrimSpace(a)) == "claude" {
			claudeSelected = true
			break
		}
	}

	if bs == "" && claudeSelected {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			bs = promptBuildSystemSelection()
		} else {
			log.Warning("no build system detected and stdin is not a TTY — defaulting to 'go'; " +
				"set --build-system flag or add a marker file (go.mod, package.json, pnpm-workspace.yaml) to auto-detect")
			bs = "go"
		}
	}
	if bs == "" {
		bs = "go" // final fallback when claude not selected
	}

	// Warn on unknown agent names before doing any work.
	for _, name := range selectedAgents {
		if _, ok := agentRegistry[name]; !ok {
			log.Warning(fmt.Sprintf("unknown agent %q — no skills directory defined; skipping skill copy for this agent", name))
		}
	}

	type fileSpec struct {
		path    string
		content string
	}
	specs := []fileSpec{
		{filepath.Join(dougDir, "doug.yaml"), dougYAMLContent(bs)},
		{filepath.Join(dougDir, "project-state.yaml"), projectStateContent()},
		{filepath.Join(dougDir, "tasks.yaml"), tasksYAMLContent()},
		{filepath.Join(dougDir, "PRD.md"), prdContent()},
	}

	for _, spec := range specs {
		if !force {
			if _, statErr := os.Stat(spec.path); statErr == nil {
				log.Warning(fmt.Sprintf("%s already exists — skipping (use --force to overwrite)", spec.path))
				continue
			}
		}
		if err := state.AtomicWrite(spec.path, []byte(spec.content)); err != nil {
			return fmt.Errorf("write %s: %w", spec.path, err)
		}
		log.Success(fmt.Sprintf("created %s", spec.path))
	}

	// Copy embedded init/ templates into the target project.
	if err := copyInitTemplates(dir, force, selectedAgents, bs); err != nil {
		return err
	}

	// Create docs/kb/ directory (silent if already exists).
	kbDir := filepath.Join(dir, "docs", "kb")
	if _, statErr := os.Stat(kbDir); os.IsNotExist(statErr) {
		if err := os.MkdirAll(kbDir, 0o755); err != nil {
			return fmt.Errorf("create docs/kb directory: %w", err)
		}
		log.Success("created docs/kb/")
	}

	// Create CHANGELOG.md at project root if it does not already exist.
	// Never overwrite an existing CHANGELOG.md — it is user-maintained.
	changelogPath := filepath.Join(dir, "CHANGELOG.md")
	if _, statErr := os.Stat(changelogPath); os.IsNotExist(statErr) {
		if err := state.AtomicWrite(changelogPath, []byte(changelogContent())); err != nil {
			return fmt.Errorf("write CHANGELOG.md: %w", err)
		}
		log.Success("created CHANGELOG.md")
	}

	if !noGitInit {
		gitDir := filepath.Join(dir, ".git")
		if _, statErr := os.Stat(gitDir); os.IsNotExist(statErr) {
			cmd := exec.Command("git", "init", dir)
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Warning(fmt.Sprintf("git init failed: %v\n%s", err, out))
			} else {
				log.Success("initialized git repository")
			}
		}
	}

	log.Info("project initialized — edit .doug/doug.yaml and .doug/tasks.yaml, then run: doug run")
	return nil
}

// copyInitTemplates walks the embedded init/ FS and copies files to the target project.
//
// Destination mapping:
//   - init/CLAUDE.md                      → {dir}/CLAUDE.md
//   - init/skills-config.yaml             → {dir}/.doug/skills-config.yaml
//   - init/*_TEMPLATE.md                  → {dir}/.doug/logs/
//   - init/skills/**                      → {dir}/{provider}/skills/ (selected agents only)
//   - init/.gitignore                     → {dir}/.gitignore
//   - init/AGENTS.md                      → {dir}/AGENTS.md (append doug section if marker absent)
//   - init/.claude/**                     → {dir}/.claude/** (selected agents only)
//   - init/.codex/**                      → {dir}/.codex/** (selected agents only)
//   - init/.gemini/**                     → {dir}/.gemini/** (selected agents only)
func copyInitTemplates(dir string, force bool, selectedAgents []string, buildSystem string) error {
	agentSelected := make(map[string]bool)
	for _, name := range selectedAgents {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			agentSelected[name] = true
		}
	}

	return fs.WalkDir(templates.Init, "init", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Strip the "init/" prefix to get the relative path within the init tree.
		rel := strings.TrimPrefix(path, "init/")

		// Per-agent settings: copy/merge only for selected agents.
		if strings.HasPrefix(rel, ".claude/") {
			if !agentSelected["claude"] {
				return nil
			}
			data, readErr := templates.Init.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("read template %s: %w", path, readErr)
			}
			if rel == ".claude/settings.json" {
				var injectErr error
				data, injectErr = injectBuildSystemPermissions(data, buildSystem)
				if injectErr != nil {
					log.Warning(fmt.Sprintf("could not inject build-system permissions: %v — proceeding with unmodified template", injectErr))
				}
			}
			return copyOrMergeAgentSettings(filepath.Join(dir, rel), rel, data, force)
		}
		if strings.HasPrefix(rel, ".codex/") {
			if !agentSelected["codex"] {
				return nil
			}
			data, readErr := templates.Init.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("read template %s: %w", path, readErr)
			}
			return copyOrMergeAgentSettings(filepath.Join(dir, rel), rel, data, force)
		}
		if strings.HasPrefix(rel, ".gemini/") {
			if !agentSelected["gemini"] {
				return nil
			}
			data, readErr := templates.Init.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("read template %s: %w", path, readErr)
			}
			return copyOrMergeAgentSettings(filepath.Join(dir, rel), rel, data, force)
		}

		// Skills: copy into each selected provider's local skills directory.
		if strings.HasPrefix(rel, "skills/") {
			skillRel := strings.TrimPrefix(rel, "skills/")
			data, readErr := templates.Init.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("read template %s: %w", path, readErr)
			}

			for _, dst := range selectedSkillDestinations(dir, agentSelected, skillRel) {
				if !force {
					if _, statErr := os.Stat(dst); statErr == nil {
						log.Warning(fmt.Sprintf("%s already exists — skipping (use --force to overwrite)", dst))
						continue
					}
				}
				if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
					return fmt.Errorf("create directory for %s: %w", dst, mkErr)
				}
				if writeErr := state.AtomicWrite(dst, data); writeErr != nil {
					return fmt.Errorf("write %s: %w", dst, writeErr)
				}
				log.Success(fmt.Sprintf("created %s", dst))
			}
			return nil
		}

		if rel == ".gitignore" {
			data, readErr := templates.Init.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("read template %s: %w", path, readErr)
			}
			return copyOrMergeGitignore(filepath.Join(dir, rel), data)
		}

		if rel == "AGENTS.md" {
			data, readErr := templates.Init.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("read template %s: %w", path, readErr)
			}
			return copyOrMergeAgents(filepath.Join(dir, "AGENTS.md"), data, dir)
		}

		// Determine single destination path for non-skills files.
		var dst string
		switch {
		case rel == "CLAUDE.md":
			dst = filepath.Join(dir, "CLAUDE.md")
		case rel == "skills-config.yaml":
			dst = filepath.Join(dir, ".doug", "skills-config.yaml")
		case strings.HasSuffix(rel, "_TEMPLATE.md"):
			dst = filepath.Join(dir, ".doug", "logs", rel)
		default:
			log.Warning(fmt.Sprintf("skipping unknown template file: %s", rel))
			return nil
		}

		if !force {
			if _, statErr := os.Stat(dst); statErr == nil {
				log.Warning(fmt.Sprintf("%s already exists — skipping (use --force to overwrite)", dst))
				return nil
			}
		}

		// Ensure parent directory exists.
		if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
			return fmt.Errorf("create directory for %s: %w", dst, mkErr)
		}

		data, readErr := templates.Init.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read template %s: %w", path, readErr)
		}

		if writeErr := state.AtomicWrite(dst, data); writeErr != nil {
			return fmt.Errorf("write %s: %w", dst, writeErr)
		}

		log.Success(fmt.Sprintf("created %s", dst))
		return nil
	})
}

func copyOrMergeGitignore(dst string, template []byte) error {
	if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
		return fmt.Errorf("create directory for %s: %w", dst, mkErr)
	}

	existing, readErr := os.ReadFile(dst)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read %s: %w", dst, readErr)
	}

	merged := mergeGitignore(string(existing), string(template))
	if writeErr := state.AtomicWrite(dst, []byte(merged)); writeErr != nil {
		return fmt.Errorf("write %s: %w", dst, writeErr)
	}

	if os.IsNotExist(readErr) {
		log.Success(fmt.Sprintf("created %s", dst))
	} else {
		log.Success(fmt.Sprintf("updated %s", dst))
	}
	return nil
}

func copyOrMergeAgents(dst string, dougSection []byte, dir string) error {
	if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
		return fmt.Errorf("create directory for %s: %w", dst, mkErr)
	}

	existing, readErr := os.ReadFile(dst)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read %s: %w", dst, readErr)
	}
	existingStr := string(existing)

	// Resolve project metadata: preserve existing values, generate if absent.
	projectID := extractManagedBlockField(existingStr, "DOUG_PROJECT_ID")
	if projectID == "" {
		projectID = generateProjectID(filepath.Base(dir))
	}
	projectName := extractManagedBlockField(existingStr, "DOUG_PROJECT_NAME")
	if projectName == "" {
		projectName = generateProjectName(filepath.Base(dir))
	}

	// Substitute placeholders in the template section.
	sectionStr := strings.ReplaceAll(string(dougSection), "{{DOUG_PROJECT_ID}}", projectID)
	sectionStr = strings.ReplaceAll(sectionStr, "{{DOUG_PROJECT_NAME}}", projectName)

	merged := mergeAgents(existingStr, sectionStr, projectID, projectName)
	if writeErr := state.AtomicWrite(dst, []byte(merged)); writeErr != nil {
		return fmt.Errorf("write %s: %w", dst, writeErr)
	}

	switch {
	case os.IsNotExist(readErr):
		log.Success(fmt.Sprintf("created %s", dst))
	case normalizeText(existingStr) == merged:
		log.Success(fmt.Sprintf("kept %s", dst))
	default:
		log.Success(fmt.Sprintf("updated %s", dst))
	}
	return nil
}

func mergeGitignore(existing, template string) string {
	existing = strings.ReplaceAll(existing, "\r\n", "\n")
	template = strings.ReplaceAll(template, "\r\n", "\n")

	existingTrimmed := strings.TrimRight(existing, "\n")
	templateTrimmed := strings.TrimRight(template, "\n")
	if existingTrimmed == "" {
		if templateTrimmed == "" {
			return ""
		}
		return templateTrimmed + "\n"
	}

	existingLines := strings.Split(existingTrimmed, "\n")
	seen := make(map[string]bool, len(existingLines))
	for _, line := range existingLines {
		seen[strings.TrimSpace(line)] = true
	}

	var additions []string
	for _, line := range strings.Split(templateTrimmed, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !seen[trimmed] {
			additions = append(additions, line)
			seen[trimmed] = true
		}
	}

	if len(additions) == 0 {
		return existingTrimmed + "\n"
	}

	return existingTrimmed + "\n\n" + strings.Join(additions, "\n") + "\n"
}

func mergeAgents(existing, dougSection, projectID, projectName string) string {
	existing = normalizeText(existing)
	dougSection = normalizeText(dougSection)

	if existing == "" {
		return dougSection
	}
	if !strings.Contains(existing, dougInstructionsMarker) {
		return existing + "\n\n" + dougSection
	}
	// Marker already present — ensure project metadata is in the block.
	return ensureMetadataInBlock(existing, projectID, projectName)
}

// ensureMetadataInBlock injects DOUG_PROJECT_ID and DOUG_PROJECT_NAME into the
// managed block if they are not already present. If they exist, the content is
// returned unchanged so that existing IDs are never silently replaced.
func ensureMetadataInBlock(content, projectID, projectName string) string {
	if strings.Contains(content, "DOUG_PROJECT_ID:") {
		return content
	}
	meta := "DOUG_PROJECT_ID: " + projectID + "\nDOUG_PROJECT_NAME: " + projectName + "\n\n"
	return strings.Replace(content, dougInstructionsMarker+"\n", dougInstructionsMarker+"\n"+meta, 1)
}

// extractManagedBlockField reads a KEY: value line from inside the managed
// AGENTS.md block. Returns an empty string if the field or block is absent.
func extractManagedBlockField(content, fieldName string) string {
	startIdx := strings.Index(content, dougInstructionsMarker)
	if startIdx == -1 {
		return ""
	}
	block := content[startIdx:]
	if endIdx := strings.Index(block, dougInstructionsEndMarker); endIdx != -1 {
		block = block[:endIdx]
	}
	prefix := fieldName + ":"
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	return ""
}

// slugify converts a string to a lowercase, hyphen-separated slug containing
// only alphanumeric characters and hyphens. Consecutive separators are collapsed.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		case r == '-' || r == '_' || r == ' ':
			if !prevHyphen {
				b.WriteRune('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// generateProjectID returns a stable project identifier of the form
// "<slug>-<6hexchars>", where the slug is derived from the project directory
// name and the suffix is randomly generated.
func generateProjectID(dirName string) string {
	slug := slugify(dirName)
	if slug == "" {
		slug = "project"
	}
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return slug + "-000000"
	}
	return fmt.Sprintf("%s-%x", slug, b)
}

// generateProjectName returns a human-readable display name derived from the
// project directory name by title-casing each word after splitting on hyphens,
// underscores, and spaces.
func generateProjectName(dirName string) string {
	s := strings.NewReplacer("-", " ", "_", " ").Replace(dirName)
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	return s + "\n"
}

func selectedSkillDestinations(dir string, agentSelected map[string]bool, skillRel string) []string {
	providers := []struct {
		name string
		root string
	}{
		{name: "claude", root: ".claude"},
		{name: "codex", root: ".codex"},
		{name: "gemini", root: ".gemini"},
	}

	var destinations []string
	for _, provider := range providers {
		if agentSelected[provider.name] {
			destinations = append(destinations, filepath.Join(dir, provider.root, "skills", skillRel))
		}
	}
	return destinations
}

func copyOrMergeAgentSettings(dst, rel string, template []byte, force bool) error {
	if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
		return fmt.Errorf("create directory for %s: %w", dst, mkErr)
	}

	if force {
		if writeErr := state.AtomicWrite(dst, template); writeErr != nil {
			return fmt.Errorf("write %s: %w", dst, writeErr)
		}
		log.Success(fmt.Sprintf("created %s", dst))
		return nil
	}

	existing, readErr := os.ReadFile(dst)
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return fmt.Errorf("read %s: %w", dst, readErr)
		}
		if writeErr := state.AtomicWrite(dst, template); writeErr != nil {
			return fmt.Errorf("write %s: %w", dst, writeErr)
		}
		log.Success(fmt.Sprintf("created %s", dst))
		return nil
	}

	var merged []byte
	switch rel {
	case ".codex/config.toml":
		merged = []byte(mergeCodexConfigTOML(string(existing)))
	case ".claude/settings.json", ".gemini/settings.json", ".gemini/policies/doug-default.json":
		out, mergeErr := mergeJSONSettings(existing, template)
		if mergeErr != nil {
			log.Warning(fmt.Sprintf("%s exists but merge failed (%v) — skipping (use --force to overwrite)", dst, mergeErr))
			return nil
		}
		merged = out
	default:
		log.Warning(fmt.Sprintf("%s already exists — skipping (use --force to overwrite)", dst))
		return nil
	}

	if writeErr := state.AtomicWrite(dst, merged); writeErr != nil {
		return fmt.Errorf("write %s: %w", dst, writeErr)
	}
	log.Success(fmt.Sprintf("merged managed settings into %s", dst))
	return nil
}

func mergeJSONSettings(existing, template []byte) ([]byte, error) {
	var current map[string]interface{}
	if err := json.Unmarshal(existing, &current); err != nil {
		return nil, err
	}

	var managed map[string]interface{}
	if err := json.Unmarshal(template, &managed); err != nil {
		return nil, err
	}

	deepMergeJSON(current, managed)

	out, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func deepMergeJSON(dst, src map[string]interface{}) {
	for key, srcVal := range src {
		dstVal, exists := dst[key]
		if !exists {
			dst[key] = srcVal
			continue
		}

		srcMap, srcMapOK := srcVal.(map[string]interface{})
		dstMap, dstMapOK := dstVal.(map[string]interface{})
		if srcMapOK && dstMapOK {
			deepMergeJSON(dstMap, srcMap)
			dst[key] = dstMap
			continue
		}

		srcArr, srcArrOK := srcVal.([]interface{})
		dstArr, dstArrOK := dstVal.([]interface{})
		if srcArrOK && dstArrOK {
			if merged, ok := mergeStringArrays(dstArr, srcArr); ok {
				dst[key] = merged
				continue
			}
		}

		dst[key] = srcVal
	}
}

func mergeStringArrays(existing, managed []interface{}) ([]interface{}, bool) {
	seen := make(map[string]bool)
	out := make([]interface{}, 0, len(existing)+len(managed))

	for _, value := range existing {
		s, ok := value.(string)
		if !ok {
			return nil, false
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, value := range managed {
		s, ok := value.(string)
		if !ok {
			return nil, false
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}

	return out, true
}

func mergeCodexConfigTOML(existing string) string {
	rootDefaults := map[string]string{
		"approval_policy": `"never"`,
		"sandbox_mode":    `"workspace-write"`,
		"web_search":      `"cached"`,
	}
	rootOrder := []string{"approval_policy", "sandbox_mode", "web_search"}
	sectionName := "sandbox_workspace_write"
	sectionDefaults := map[string]string{
		"network_access": "false",
		"writable_roots": "[]",
	}
	sectionOrder := []string{"network_access", "writable_roots"}

	lines := strings.Split(existing, "\n")
	inSection := ""
	foundRoot := make(map[string]bool)
	foundSection := make(map[string]bool)

	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			inSection = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trim, "["), "]"))
			continue
		}

		eq := strings.Index(trim, "=")
		if eq == -1 {
			continue
		}
		key := strings.TrimSpace(trim[:eq])
		if inSection == "" {
			if value, ok := rootDefaults[key]; ok {
				lines[i] = fmt.Sprintf("%s = %s", key, value)
				foundRoot[key] = true
			}
			continue
		}
		if inSection == sectionName {
			if value, ok := sectionDefaults[key]; ok {
				lines[i] = fmt.Sprintf("%s = %s", key, value)
				foundSection[key] = true
			}
		}
	}

	firstSection := len(lines)
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			firstSection = i
			break
		}
	}

	var missingRoot []string
	for _, key := range rootOrder {
		if !foundRoot[key] {
			missingRoot = append(missingRoot, fmt.Sprintf("%s = %s", key, rootDefaults[key]))
		}
	}
	if len(missingRoot) > 0 {
		prefix := append([]string{}, lines[:firstSection]...)
		if len(prefix) > 0 && strings.TrimSpace(prefix[len(prefix)-1]) != "" {
			prefix = append(prefix, "")
		}
		prefix = append(prefix, missingRoot...)
		if firstSection < len(lines) && strings.TrimSpace(lines[firstSection]) != "" {
			prefix = append(prefix, "")
		}
		lines = append(prefix, lines[firstSection:]...)
	}

	sectionStart := -1
	sectionEnd := len(lines)
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trim, "["), "]"))
			if name == sectionName {
				sectionStart = i
				continue
			}
			if sectionStart != -1 {
				sectionEnd = i
				break
			}
		}
	}

	if sectionStart == -1 {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("[%s]", sectionName))
		for _, key := range sectionOrder {
			lines = append(lines, fmt.Sprintf("%s = %s", key, sectionDefaults[key]))
		}
	} else {
		var missingSection []string
		for _, key := range sectionOrder {
			if !foundSection[key] {
				missingSection = append(missingSection, fmt.Sprintf("%s = %s", key, sectionDefaults[key]))
			}
		}
		if len(missingSection) > 0 {
			prefix := append([]string{}, lines[:sectionEnd]...)
			prefix = append(prefix, missingSection...)
			lines = append(prefix, lines[sectionEnd:]...)
		}
	}

	return strings.Join(lines, "\n")
}

// dougYAMLContent returns the .doug/doug.yaml file content with inline YAML comments
// and the detected (or specified) build system pre-filled.
func dougYAMLContent(buildSystem string) string {
	return fmt.Sprintf(`# doug.yaml — orchestrator configuration
# See https://github.com/robertgumeny/doug for documentation.
agent_command: 'claude -p "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"' # Command used to invoke the agent (e.g. claude, codex, gemini, etc.)
# agent_command: codex exec "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"
# agent_command: gemini --approval-mode auto_edit --output-format json --sandbox "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}} and complete the task described in .doug/ACTIVE_TASK.md"
build_system: %s # Build system: go | npm | pnpm (auto-detected by init; override here)
max_retries: 3 # Max FAILURE outcomes before a task is BLOCKED
max_iterations: 10 # Max loop iterations before the run exits
kb_enabled: true # If false, skip KB synthesis task after features complete
agent_heartbeat_seconds: 30 # Periodic liveness log cadence while agent runs (0 disables)
`, buildSystem)
}

// tasksYAMLContent returns a starter tasks.yaml with one example epic and two tasks,
// containing all required fields.
func tasksYAMLContent() string {
	return `epic:
  id: "EPIC-1"
  name: "First Epic"
  tasks:
    - id: "EPIC-1-001"
      type: "feature"
      status: "TODO"
      description: "Implement the first feature of the project."
      acceptance_criteria:
        - "The feature is implemented and all related tests pass"
        - "Code follows the project's conventions and style guidelines"
    - id: "EPIC-1-002"
      type: "feature"
      status: "TODO"
      description: "Implement the second feature of the project."
      acceptance_criteria:
        - "The feature is implemented and all related tests pass"
        - "All acceptance criteria have been verified end-to-end"
`
}

// projectStateContent returns a minimal valid project-state.yaml for a new project.
// BootstrapFromTasks fires on first run because state.CurrentEpic.ID is empty,
// populating the rest of the state from tasks.yaml.
func projectStateContent() string {
	return "{}\n"
}

// changelogContent returns a starter CHANGELOG.md following the Keep a Changelog format.
func changelogContent() string {
	return `# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

### Removed
`
}

// prdContent returns a starter PRD.md template for new projects.
func prdContent() string {
	return `# PRD: [Project Name]

**Version**: 1.0
**Status**: Draft

---

## Problem

[Describe the problem this project solves and why it matters.]

---

## Goal

[What does success look like? What will this project produce?]

---

## Non-Goals

- [What is explicitly out of scope?]

---

## Architecture

[High-level architecture diagram or description.]

---

## Epics

| Epic | Theme | Tasks | Depends On |
|------|-------|-------|------------|
| 1    | [Theme] | 2  | —          |

---

## Definition of Done

- [ ] All tasks are DONE
- [ ] Build passes
- [ ] Tests pass
`
}
