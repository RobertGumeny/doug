package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitInstallsCanonicalDougSkills(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatal(err)
	}
	for _, name := range dougSkillNames {
		if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("canonical skill %s: %v", name, err)
		}
		if _, err := os.Lstat(filepath.Join(dir, ".pi", "skills", name)); !os.IsNotExist(err) {
			t.Errorf("legacy Pi skill %s exists", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".pi", "extensions", "handoff.ts")); err != nil {
		t.Errorf("Pi handoff extension: %v", err)
	}
}

func TestInitCreatesClaudeSkillsSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := initProject(dir, false, "go", true); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".claude", "skills")
	target, err := os.Readlink(link)
	if err != nil || target != "../.agents/skills" {
		t.Fatalf("Readlink = %q, %v", target, err)
	}
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil || !samePath(resolved, filepath.Join(dir, ".agents", "skills")) {
		t.Fatalf("bridge resolves to %q, %v", resolved, err)
	}
}

// samePath must look through symlinks on both sides. Bridge validation compares
// an EvalSymlinks-resolved link against an unresolved expected path, so an
// Abs-only comparison rejected correct bridges wherever the project path
// traversed a symlink — the macOS /var -> /private/var case.
func TestSamePathResolvesSymlinkedParents(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.MkdirAll(filepath.Join(real, ".agents", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	viaLink := filepath.Join(link, ".agents", "skills")
	viaReal := filepath.Join(real, ".agents", "skills")
	if !samePath(viaLink, viaReal) {
		t.Errorf("samePath(%q, %q) = false, want true", viaLink, viaReal)
	}
	if samePath(viaReal, filepath.Join(real, ".agents", "other")) {
		t.Error("samePath matched two distinct paths")
	}
}

func TestInitClaudeSkillsFallback(t *testing.T) {
	dir := t.TempDir()
	old := claudeSkillsSymlink
	claudeSkillsSymlink = func(_, _ string) error { return errors.New("unsupported") }
	t.Cleanup(func() { claudeSkillsSymlink = old })
	var out bytes.Buffer
	if err := doInitProject(&out, dir, false, "go", true, initDefaultMaxRetries, initDefaultMaxIterations, initDefaultKBEnabled); err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(dir, ".claude", "skills", "user-skill", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(user), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(user, []byte("user bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installClaudeSkillsBridge(&out, dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(user)
	if err != nil || string(data) != "user bytes" {
		t.Fatalf("user fixture changed: %q, %v", data, err)
	}
	for _, name := range dougSkillNames {
		if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("fallback skill %s: %v", name, err)
		}
	}
	if _, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", claudeSkillsManifestName)); err != nil {
		t.Errorf("manifest: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("managed copies")) {
		t.Error("missing fallback warning")
	}
}

func TestUpgradeClaudeBridgeSymlinkStates(t *testing.T) {
	t.Run("absent and empty become exact relative symlink", func(t *testing.T) {
		for _, empty := range []bool{false, true} {
			dir := upgradeBridgeFixture(t)
			path := filepath.Join(dir, ".claude", "skills")
			if empty {
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := applyUpgrade(&bytes.Buffer{}, dir, []driftItem{{Kind: driftClaudeBridge, Action: actionBridge}}, false); err != nil {
				t.Fatal(err)
			}
			target, err := os.Readlink(path)
			if err != nil || target != "../.agents/skills" {
				t.Fatalf("bridge = %q, %v", target, err)
			}
		}
	})
	t.Run("correct is unchanged; wrong and broken provide remediation", func(t *testing.T) {
		dir := upgradeBridgeFixture(t)
		path := filepath.Join(dir, ".claude", "skills")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../.agents/skills", path); err != nil {
			t.Fatal(err)
		}
		items, err := inspectClaudeSkillsBridge(dir)
		if err != nil || len(items) != 0 {
			t.Fatalf("correct bridge inspection = %v, %v", items, err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		for _, target := range []string{"../wrong", "../missing"} {
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			_, err := inspectClaudeSkillsBridge(dir)
			if err == nil || !strings.Contains(err.Error(), "remove it or restore") {
				t.Fatalf("link %q error = %v", target, err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestUpgradeClaudeBridgePreservesUserSkills(t *testing.T) {
	dir := upgradeBridgeFixture(t)
	user := filepath.Join(dir, ".claude", "skills", "user-skill", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(user), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(user, []byte("user bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := applyUpgrade(&out, dir, []driftItem{{Kind: driftClaudeBridge, Action: actionBridge}}, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(user); err != nil || string(got) != "user bytes" {
		t.Fatalf("user fixture = %q, %v", got, err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, ".claude", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "doug-") {
			count++
		}
	}
	if count != len(dougSkillNames) {
		t.Fatalf("managed copy count = %d, want %d", count, len(dougSkillNames))
	}
	if _, err := claudeFallbackOwnership(filepath.Join(dir, ".claude", "skills")); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if !strings.Contains(out.String(), "managed copies") {
		t.Fatal("missing fallback warning")
	}
}

func TestUpgradeClaudeBridgeRejectsUnregisteredConflict(t *testing.T) {
	dir := upgradeBridgeFixture(t)
	conflict := filepath.Join(dir, ".claude", "skills", "doug-plan", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(conflict), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conflict, []byte("do not touch"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := applyUpgrade(&bytes.Buffer{}, dir, []driftItem{{Kind: driftClaudeBridge, Action: actionBridge}}, false)
	if err == nil || !strings.Contains(err.Error(), "unregistered") {
		t.Fatalf("apply error = %v", err)
	}
	if got, err := os.ReadFile(conflict); err != nil || string(got) != "do not touch" {
		t.Fatalf("conflict changed = %q, %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".claude", "skills", claudeSkillsManifestName)); !os.IsNotExist(err) {
		t.Fatalf("manifest written despite conflict: %v", err)
	}
}

func TestUpgradeSkillMigrationIsIdempotent(t *testing.T) {
	dir := upgradeBridgeFixture(t)
	user := filepath.Join(dir, ".claude", "skills", "user", "fixture")
	if err := os.MkdirAll(filepath.Dir(user), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(user, []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := inspectWorkspace(dir, filepath.Join(dir, ".doug"))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyUpgrade(&bytes.Buffer{}, dir, items, false); err != nil {
		t.Fatal(err)
	}
	items, err = inspectWorkspace(dir, filepath.Join(dir, ".doug"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("second inspection found drift: %+v", items)
	}
}

func upgradeBridgeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := copyInitTemplates(&bytes.Buffer{}, dir, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ".claude", "skills")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestUpgradeNeverRemovesClaudeDirectory(t *testing.T) {
	for _, force := range []bool{false, true} {
		dir := t.TempDir()
		claude := filepath.Join(dir, ".claude")
		if err := os.MkdirAll(claude, 0o755); err != nil {
			t.Fatal(err)
		}
		items, err := inspectRetiredArtifacts(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			if item.DisplayPath == ".claude" {
				t.Fatalf(".claude reported as retired")
			}
		}
		if err := applyUpgrade(&bytes.Buffer{}, dir, items, force); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(claude); err != nil {
			t.Fatalf(".claude removed with force=%v: %v", force, err)
		}
	}
}
