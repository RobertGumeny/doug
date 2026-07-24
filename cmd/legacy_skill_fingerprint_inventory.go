package cmd

// legacySkillFileFingerprint is the immutable identity of one regular file in
// the final unnamespaced built-in skill inventory. It is deliberately static:
// the embedded templates have been renamed and must never be used to recreate
// these legacy fingerprints.
type legacySkillFileFingerprint struct {
	RelativePath string
	FileType     string
	SHA256       string
}

// legacySkillRootFingerprint records every file in one legacy skill root.
type legacySkillRootFingerprint struct {
	Name  string
	Files []legacySkillFileFingerprint
}

// legacySkillFingerprintInventory is the final, pre-doug-* template inventory.
// Keep this checked-in data unchanged: upgrade uses it to identify only exact,
// untouched legacy trees that Doug may safely migrate.
var legacySkillFingerprintInventory = []legacySkillRootFingerprint{
	{
		Name: "implement-bugfix",
		Files: []legacySkillFileFingerprint{{
			RelativePath: "SKILL.md",
			FileType:     "regular",
			SHA256:       "54a0c62ecaba6e93aaac8f979cab87a5953ce6e7ccf61907f49790c70eb87f9e",
		}},
	},
	{
		Name: "implement-documentation",
		Files: []legacySkillFileFingerprint{{
			RelativePath: "SKILL.md",
			FileType:     "regular",
			SHA256:       "855d08bb0f7b4b9741be5b5adc631519bc627cda725f1b71054dc9e7fd8aca33",
		}},
	},
	{
		Name: "implement-feature",
		Files: []legacySkillFileFingerprint{{
			RelativePath: "SKILL.md",
			FileType:     "regular",
			SHA256:       "772da50ea6ba88abbe28813d3dd7db3ef2c8a7c0348baf057f417c733075ba5d",
		}},
	},
	{
		Name: "plan",
		Files: []legacySkillFileFingerprint{
			{RelativePath: "SKILL.md", FileType: "regular", SHA256: "527bd7f4f2658cc81cf338d34f3e9f610d618ddecbdebbbfcd4e0ba8d958b45a"},
			{RelativePath: "references/bugfix.md", FileType: "regular", SHA256: "19bc017ffb525f0f74c760ee849f8f52ce9b4f4f6066f43f3fe41661618d6d23"},
			{RelativePath: "references/definition.md", FileType: "regular", SHA256: "ace99598eaab95e246b4a00b85baede91eef598989e5840fadfcaa35dd8a3fa4"},
			{RelativePath: "references/discovery.md", FileType: "regular", SHA256: "3c1bb99b2dce6af078d769d6ca295ae5733f418e394924b0fc2f776fd1fdfbac"},
			{RelativePath: "references/feature.md", FileType: "regular", SHA256: "c3395219c75b7b4d7621efc4c505c92e733e8854032721d4ae3915716a89f1fc"},
			{RelativePath: "references/greenfield.md", FileType: "regular", SHA256: "227ea37c04997fa9602a676ab08884700e1cd3d2265815fe213612c4db3dfee9"},
			{RelativePath: "references/prd-template.md", FileType: "regular", SHA256: "bea560d31df55023e7c3968ddb021136c508c8e848a29032a5bc94eef927fe6c"},
			{RelativePath: "references/refactor.md", FileType: "regular", SHA256: "1e2d9c5919fa22223218bc10565b1d0717fc1f0c39b8021fcb6fdccabd46aa7a"},
			{RelativePath: "references/roadmapping.md", FileType: "regular", SHA256: "274c2e2b138b552e0588fbab0e78da1a150bfb9c006374588d77f42a14e40f54"},
			{RelativePath: "references/task-examples.md", FileType: "regular", SHA256: "444a2de5b4fd908798c1cbbfa1825d21a2d7fc4a648c48f11a06eca53fb9e52a"},
		},
	},
	{
		Name: "research",
		Files: []legacySkillFileFingerprint{{
			RelativePath: "SKILL.md",
			FileType:     "regular",
			SHA256:       "15181576edc53b125b405c4ac19ed55e2fc3273fee77fff144e6f68a2f6b6729",
		}},
	},
	{
		Name: "scaffold",
		Files: []legacySkillFileFingerprint{{
			RelativePath: "SKILL.md",
			FileType:     "regular",
			SHA256:       "38c3ad8fa77184d1e77e528ba06e44148330a534b35ec9e9f06762ef5cb1fe67",
		}},
	},
}
