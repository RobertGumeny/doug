package cmd

import (
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/templates"
)

func TestLegacySkillFingerprintInventory(t *testing.T) {
	// This manifest is intentionally independent from templates.Init. The
	// unnamespaced template bytes no longer exist in that embedded filesystem.
	want := []string{
		"implement-bugfix|SKILL.md|regular|54a0c62ecaba6e93aaac8f979cab87a5953ce6e7ccf61907f49790c70eb87f9e",
		"implement-documentation|SKILL.md|regular|855d08bb0f7b4b9741be5b5adc631519bc627cda725f1b71054dc9e7fd8aca33",
		"implement-feature|SKILL.md|regular|772da50ea6ba88abbe28813d3dd7db3ef2c8a7c0348baf057f417c733075ba5d",
		"plan|SKILL.md|regular|527bd7f4f2658cc81cf338d34f3e9f610d618ddecbdebbbfcd4e0ba8d958b45a",
		"plan|references/bugfix.md|regular|19bc017ffb525f0f74c760ee849f8f52ce9b4f4f6066f43f3fe41661618d6d23",
		"plan|references/definition.md|regular|ace99598eaab95e246b4a00b85baede91eef598989e5840fadfcaa35dd8a3fa4",
		"plan|references/discovery.md|regular|3c1bb99b2dce6af078d769d6ca295ae5733f418e394924b0fc2f776fd1fdfbac",
		"plan|references/feature.md|regular|c3395219c75b7b4d7621efc4c505c92e733e8854032721d4ae3915716a89f1fc",
		"plan|references/greenfield.md|regular|227ea37c04997fa9602a676ab08884700e1cd3d2265815fe213612c4db3dfee9",
		"plan|references/prd-template.md|regular|bea560d31df55023e7c3968ddb021136c508c8e848a29032a5bc94eef927fe6c",
		"plan|references/refactor.md|regular|1e2d9c5919fa22223218bc10565b1d0717fc1f0c39b8021fcb6fdccabd46aa7a",
		"plan|references/roadmapping.md|regular|274c2e2b138b552e0588fbab0e78da1a150bfb9c006374588d77f42a14e40f54",
		"plan|references/task-examples.md|regular|444a2de5b4fd908798c1cbbfa1825d21a2d7fc4a648c48f11a06eca53fb9e52a",
		"research|SKILL.md|regular|15181576edc53b125b405c4ac19ed55e2fc3273fee77fff144e6f68a2f6b6729",
		"scaffold|SKILL.md|regular|38c3ad8fa77184d1e77e528ba06e44148330a534b35ec9e9f06762ef5cb1fe67",
	}
	got := make([]string, 0, len(want))
	for _, root := range legacySkillFingerprintInventory {
		for _, file := range root.Files {
			got = append(got, strings.Join([]string{root.Name, file.RelativePath, file.FileType, file.SHA256}, "|"))
		}
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy skill fingerprint inventory =\n%v\nwant\n%v", got, want)
	}
}

func TestEmbeddedSkillsUseDougNamespace(t *testing.T) {
	const skillsRoot = "init/skills"
	wantNames := map[string]bool{
		"doug-implement-feature":       true,
		"doug-implement-bugfix":        true,
		"doug-implement-documentation": true,
		"doug-scaffold":                true,
		"doug-plan":                    true,
		"doug-research":                true,
	}

	entries, err := templates.Init.ReadDir(skillsRoot)
	if err != nil {
		t.Fatalf("read embedded skills: %v", err)
	}
	if len(entries) != len(wantNames) {
		t.Fatalf("embedded skill roots = %d, want %d", len(entries), len(wantNames))
	}
	for _, entry := range entries {
		if !entry.IsDir() || !wantNames[entry.Name()] {
			t.Errorf("embedded skill root %q is not a Doug-owned namespaced root", entry.Name())
			continue
		}
		data, readErr := templates.Init.ReadFile(skillsRoot + "/" + entry.Name() + "/SKILL.md")
		if readErr != nil {
			t.Errorf("read %s/SKILL.md: %v", entry.Name(), readErr)
			continue
		}
		if !strings.Contains(string(data), `name: "`+entry.Name()+`"`) {
			t.Errorf("%s/SKILL.md frontmatter does not name %q", entry.Name(), entry.Name())
		}
	}

	if err := fs.WalkDir(templates.Init, skillsRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == skillsRoot || !d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, skillsRoot+"/")
		if !strings.Contains(rel, "/") && !strings.HasPrefix(rel, "doug-") {
			t.Errorf("unnamespaced built-in skill root remains embedded: %s", rel)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk embedded skills: %v", err)
	}
}
