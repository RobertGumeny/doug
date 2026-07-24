package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
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
