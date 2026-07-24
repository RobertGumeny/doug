package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/state"
)

const claudeSkillsManifestName = ".doug-managed-skills.json"

var dougSkillNames = []string{
	"doug-implement-bugfix",
	"doug-implement-documentation",
	"doug-implement-feature",
	"doug-plan",
	"doug-research",
	"doug-scaffold",
}

// claudeSkillsSymlink is a seam for tests and for platforms where creating a
// symlink is unavailable. It creates the preferred Claude skills bridge.
var claudeSkillsSymlink = os.Symlink

// installClaudeSkillsBridge exposes the canonical Doug skills to Claude. The
// normal bridge is a relative symlink; a real Claude skills directory receives
// ownership-recorded Doug copies instead, without touching its user entries.
func installClaudeSkillsBridge(w io.Writer, projectRoot string) error {
	canonical := filepath.Join(projectRoot, ".agents", "skills")
	claudeSkills := filepath.Join(projectRoot, ".claude", "skills")

	info, err := os.Lstat(claudeSkills)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(claudeSkills), 0o755); err != nil {
			return fmt.Errorf("create .claude directory: %w", err)
		}
		if err := claudeSkillsSymlink("../.agents/skills", claudeSkills); err == nil {
			writef(w, "  ✓ .claude/skills -> ../.agents/skills\n")
			return nil
		}
		// A symlink can be unavailable (notably on some Windows installations).
		// Fall through to the managed-copy bridge; it intentionally preserves the
		// otherwise empty directory created above.
		if err := os.MkdirAll(claudeSkills, 0o755); err != nil {
			return fmt.Errorf("create Claude fallback skills directory: %w", err)
		}
		return installClaudeSkillsFallback(w, canonical, claudeSkills)
	}
	if err != nil {
		return fmt.Errorf("inspect .claude/skills: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(claudeSkills)
		if readErr != nil {
			return fmt.Errorf("read .claude/skills link: %w", readErr)
		}
		if target != "../.agents/skills" {
			return fmt.Errorf("refusing to retarget .claude/skills: expected ../.agents/skills, found %q", target)
		}
		resolved, evalErr := filepath.EvalSymlinks(claudeSkills)
		if evalErr != nil || !samePath(resolved, canonical) {
			return fmt.Errorf("refusing broken .claude/skills bridge")
		}
		return nil
	}
	if !info.IsDir() {
		return fmt.Errorf("refusing to replace non-directory .claude/skills")
	}
	return installClaudeSkillsFallback(w, canonical, claudeSkills)
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && aa == bb
}

func installClaudeSkillsFallback(w io.Writer, canonical, claudeSkills string) error {
	owned, err := claudeFallbackOwnership(claudeSkills)
	if err != nil {
		return err
	}
	for _, skill := range dougSkillNames {
		if _, statErr := os.Lstat(filepath.Join(claudeSkills, skill)); statErr == nil && !owned {
			return fmt.Errorf("refusing to overwrite unregistered Claude skill %s", skill)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return fmt.Errorf("inspect Claude skill %s: %w", skill, statErr)
		}
	}

	for _, skill := range dougSkillNames {
		destination := filepath.Join(claudeSkills, skill)
		if owned {
			// The exact manifest proves this root is Doug-managed, so replace it
			// as a complete tree rather than leaving stale generated files behind.
			if err := os.RemoveAll(destination); err != nil {
				return fmt.Errorf("remove owned Claude fallback skill %s: %w", skill, err)
			}
		}
		if err := copyDirectory(filepath.Join(canonical, skill), destination); err != nil {
			return fmt.Errorf("copy Claude fallback skill %s: %w", skill, err)
		}
	}
	manifest, err := json.Marshal(struct {
		SchemaVersion int      `json:"schema_version"`
		Skills        []string `json:"skills"`
	}{SchemaVersion: 1, Skills: dougSkillNames})
	if err != nil {
		return fmt.Errorf("marshal Claude skills manifest: %w", err)
	}
	manifest = append(manifest, '\n')
	if err := state.AtomicWrite(filepath.Join(claudeSkills, claudeSkillsManifestName), manifest); err != nil {
		return fmt.Errorf("write Claude skills manifest: %w", err)
	}
	const warning = "Claude skills fallback uses managed copies because .claude/skills could not be symlinked"
	writef(w, "  ! Warning: %s\n", warning)
	log.Warning(warning)
	return nil
}

// claudeFallbackOwnership reports whether the exact versioned manifest proves
// ownership of the six fallback skill roots. A missing manifest is normal; a
// malformed one never proves ownership and is rejected before any mutation.
func claudeFallbackOwnership(claudeSkills string) (bool, error) {
	path := filepath.Join(claudeSkills, claudeSkillsManifestName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read Claude skills ownership manifest: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || len(raw) != 2 {
		return false, fmt.Errorf("refusing malformed Claude skills ownership manifest")
	}
	version, hasVersion := raw["schema_version"]
	skills, hasSkills := raw["skills"]
	if !hasVersion || !hasSkills {
		return false, fmt.Errorf("refusing malformed Claude skills ownership manifest")
	}
	var v int
	var names []string
	if json.Unmarshal(version, &v) != nil || json.Unmarshal(skills, &names) != nil || v != 1 || !sameSkills(names, dougSkillNames) {
		return false, fmt.Errorf("refusing malformed Claude skills ownership manifest")
	}
	return true, nil
}

func sameSkills(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func copyDirectory(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular source entry %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return state.AtomicWrite(target, data)
	})
}
