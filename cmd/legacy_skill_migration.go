package cmd

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// legacySkillTreeMatchesInventory proves that skillsRoot contains precisely the
// final unnamespaced Doug skill tree. It intentionally follows no symlinks:
// every inventory entry must be a regular file with its recorded hash, and
// every directory and file in the tree must be expected.
func legacySkillTreeMatchesInventory(skillsRoot string) (bool, error) {
	rootInfo, err := os.Lstat(skillsRoot)
	if err != nil {
		return false, err
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return false, nil
	}

	expectedFiles := make(map[string]legacySkillFileFingerprint)
	expectedDirs := map[string]bool{".": true}
	for _, root := range legacySkillFingerprintInventory {
		expectedDirs[root.Name] = true
		for _, file := range root.Files {
			rel := filepath.Join(root.Name, filepath.FromSlash(file.RelativePath))
			expectedFiles[rel] = file
			for dir := filepath.Dir(rel); dir != "."; dir = filepath.Dir(dir) {
				expectedDirs[dir] = true
			}
		}
	}

	seenFiles := make(map[string]bool, len(expectedFiles))
	seenDirs := make(map[string]bool, len(expectedDirs))
	err = filepath.WalkDir(skillsRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(skillsRoot, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			if !expectedDirs[rel] {
				return errUnexpectedLegacySkillEntry
			}
			seenDirs[rel] = true
			return nil
		}

		fingerprint, ok := expectedFiles[rel]
		if !ok || !info.Mode().IsRegular() || fingerprint.FileType != "regular" {
			return errUnexpectedLegacySkillEntry
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != fingerprint.SHA256 {
			return errUnexpectedLegacySkillEntry
		}
		seenFiles[rel] = true
		return nil
	})
	if err != nil {
		if err == errUnexpectedLegacySkillEntry {
			return false, nil
		}
		return false, err
	}
	return sameLegacySkillEntries(expectedFiles, seenFiles) && sameLegacySkillDirs(expectedDirs, seenDirs), nil
}

var errUnexpectedLegacySkillEntry = fmt.Errorf("unexpected legacy skill entry")

func sameLegacySkillEntries(want map[string]legacySkillFileFingerprint, got map[string]bool) bool {
	if len(want) != len(got) {
		return false
	}
	for path := range want {
		if !got[path] {
			return false
		}
	}
	return true
}

func sameLegacySkillDirs(want, got map[string]bool) bool {
	if len(want) != len(got) {
		return false
	}
	for path := range want {
		if !got[path] {
			return false
		}
	}
	return true
}
