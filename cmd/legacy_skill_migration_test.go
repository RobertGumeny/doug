package cmd

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpgradeMigratesOnlyFingerprintMatchedLegacySkills(t *testing.T) {
	original := legacySkillFingerprintInventory
	legacySkillFingerprintInventory = testLegacySkillInventory()
	t.Cleanup(func() { legacySkillFingerprintInventory = original })

	t.Run("exact tree is removed", func(t *testing.T) {
		dir := t.TempDir()
		writeTestLegacySkill(t, dir, "legacy bytes")
		items, err := inspectWorkspace(dir, filepath.Join(dir, ".doug"))
		if err != nil {
			t.Fatal(err)
		}
		if !hasAction(items, actionRemoveLegacySkills) {
			t.Fatalf("expected fingerprint-matched legacy migration: %#v", items)
		}
		if err := applyUpgrade(&bytes.Buffer{}, dir, items, false); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(filepath.Join(dir, ".pi", "skills")); !os.IsNotExist(err) {
			t.Fatalf("legacy tree remains: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "doug-implement-feature", "SKILL.md")); err != nil {
			t.Fatalf("canonical skills not installed: %v", err)
		}
	})

	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, dir string)
	}{
		{"extra", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, ".pi", "skills", "user", "keep.txt"), "user bytes")
		}},
		{"missing", func(t *testing.T, dir string) {
			os.Remove(filepath.Join(dir, ".pi", "skills", "implement-feature", "SKILL.md"))
		}},
		{"modified", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, ".pi", "skills", "implement-feature", "SKILL.md"), "changed")
		}},
		{"non-regular", func(t *testing.T, dir string) {
			os.Remove(filepath.Join(dir, ".pi", "skills", "implement-feature", "SKILL.md"))
			if err := os.Symlink("elsewhere", filepath.Join(dir, ".pi", "skills", "implement-feature", "SKILL.md")); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name+" tree is retained byte-identically", func(t *testing.T) {
			dir := t.TempDir()
			writeTestLegacySkill(t, dir, "legacy bytes")
			tc.mutate(t, dir)
			before := snapshotTree(t, filepath.Join(dir, ".pi", "skills"))
			items, err := inspectWorkspace(dir, filepath.Join(dir, ".doug"))
			if err != nil {
				t.Fatal(err)
			}
			if hasAction(items, actionRemoveLegacySkills) {
				t.Fatalf("unsafe migration planned: %#v", items)
			}
			if err := applyUpgrade(&bytes.Buffer{}, dir, items, false); err != nil {
				t.Fatal(err)
			}
			if got := snapshotTree(t, filepath.Join(dir, ".pi", "skills")); got != before {
				t.Fatalf("legacy tree changed:\n got %s\nwant %s", got, before)
			}
		})
	}
}

func TestUpgradePreservesPreFinalLegacySkills(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, ".pi", "skills", "implement-feature", "SKILL.md")
	writeFile(t, stale, "an older Doug skill")
	before, err := os.ReadFile(stale)
	if err != nil {
		t.Fatal(err)
	}

	items, err := inspectWorkspace(dir, filepath.Join(dir, ".doug"))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	reportDrift(&output, items)
	if !strings.Contains(output.String(), filepath.Join(".pi", "skills")) || !hasAction(items, actionWarn) {
		t.Fatalf("missing stale-path warning: %s", output.String())
	}
	if err := applyUpgrade(&output, dir, items, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(stale); err != nil || !bytes.Equal(got, before) {
		t.Fatalf("stale legacy skill changed: %q, %v", got, err)
	}
	for _, name := range dougSkillNames {
		if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("canonical %s not installed: %v", name, err)
		}
	}
}

func testLegacySkillInventory() []legacySkillRootFingerprint {
	hash := sha256.Sum256([]byte("legacy bytes"))
	return []legacySkillRootFingerprint{{Name: "implement-feature", Files: []legacySkillFileFingerprint{{RelativePath: "SKILL.md", FileType: "regular", SHA256: fmt.Sprintf("%x", hash)}}}}
}

func writeTestLegacySkill(t *testing.T, dir, contents string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, ".pi", "skills", "implement-feature", "SKILL.md"), contents)
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasAction(items []driftItem, action upgradeAction) bool {
	for _, item := range items {
		if item.Action == action {
			return true
		}
	}
	return false
}

func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			entries = append(entries, rel+"|file|"+string(data))
			return nil
		}
		entries = append(entries, rel+"|"+info.Mode().String())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(entries, "\n")
}
