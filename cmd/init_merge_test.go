package cmd

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// mergeStringArrays
// ---------------------------------------------------------------------------

func TestMergeStringArrays_EmptyBoth(t *testing.T) {
	got, ok := mergeStringArrays(nil, nil)
	if !ok {
		t.Fatal("expected ok=true for nil inputs")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestMergeStringArrays_ExistingOnly(t *testing.T) {
	existing := toIface("a", "b")
	got, ok := mergeStringArrays(existing, nil)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestMergeStringArrays_ManagedOnly(t *testing.T) {
	managed := toIface("x", "y")
	got, ok := mergeStringArrays(nil, managed)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestMergeStringArrays_Deduplicates(t *testing.T) {
	existing := toIface("a", "b", "c")
	managed := toIface("b", "d")
	got, ok := mergeStringArrays(existing, managed)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d]=%v, want %v", i, got[i], w)
		}
	}
}

func TestMergeStringArrays_PreservesExistingOrder(t *testing.T) {
	existing := toIface("z", "a", "m")
	managed := toIface("a", "b")
	got, ok := mergeStringArrays(existing, managed)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got[0] != "z" || got[1] != "a" || got[2] != "m" || got[3] != "b" {
		t.Errorf("order not preserved: %v", got)
	}
}

func TestMergeStringArrays_NonStringReturnsFalse(t *testing.T) {
	existing := []interface{}{"a", 42}
	_, ok := mergeStringArrays(existing, nil)
	if ok {
		t.Fatal("expected ok=false when non-string element present")
	}
}

func toIface(ss ...string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// ---------------------------------------------------------------------------
// mergeJSONSettings
// ---------------------------------------------------------------------------

func TestMergeJSONSettings_AddsMissingTopLevelKey(t *testing.T) {
	existing := []byte(`{"a": 1}`)
	template := []byte(`{"b": 2}`)
	got, err := mergeJSONSettings(existing, template)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, `"a"`) {
		t.Error("existing key 'a' not preserved")
	}
	if !strings.Contains(s, `"b"`) {
		t.Error("managed key 'b' not injected")
	}
}

func TestMergeJSONSettings_PreservesExistingValue(t *testing.T) {
	existing := []byte(`{"key": "original"}`)
	template := []byte(`{"key": "managed"}`)
	got, err := mergeJSONSettings(existing, template)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Non-array scalars: managed overwrites existing (deepMergeJSON semantics).
	if !strings.Contains(string(got), `"managed"`) {
		t.Error("managed scalar value should overwrite existing")
	}
}

func TestMergeJSONSettings_MergesNestedObjects(t *testing.T) {
	existing := []byte(`{"outer": {"a": 1, "b": 2}}`)
	template := []byte(`{"outer": {"b": 99, "c": 3}}`)
	got, err := mergeJSONSettings(existing, template)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, `"a"`) {
		t.Error("existing nested key 'a' not preserved")
	}
	if !strings.Contains(s, `"c"`) {
		t.Error("managed nested key 'c' not injected")
	}
}

func TestMergeJSONSettings_UnionMergesStringArrays(t *testing.T) {
	existing := []byte(`{"arr": ["x", "y"]}`)
	template := []byte(`{"arr": ["y", "z"]}`)
	got, err := mergeJSONSettings(existing, template)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(got)
	if strings.Count(s, `"y"`) != 1 {
		t.Errorf("duplicate 'y' expected exactly once; got:\n%s", s)
	}
	if !strings.Contains(s, `"z"`) {
		t.Error("managed array value 'z' not appended")
	}
	if !strings.Contains(s, `"x"`) {
		t.Error("existing array value 'x' not preserved")
	}
}

func TestMergeJSONSettings_TrailingNewline(t *testing.T) {
	existing := []byte(`{}`)
	template := []byte(`{"k": "v"}`)
	got, err := mergeJSONSettings(existing, template)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 || got[len(got)-1] != '\n' {
		t.Errorf("expected trailing newline; got %q", got)
	}
}

func TestMergeJSONSettings_InvalidExistingJSON(t *testing.T) {
	_, err := mergeJSONSettings([]byte(`{invalid`), []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for invalid existing JSON")
	}
}

func TestMergeJSONSettings_InvalidTemplateJSON(t *testing.T) {
	_, err := mergeJSONSettings([]byte(`{}`), []byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid template JSON")
	}
}

// ---------------------------------------------------------------------------
// mergeCodexConfigTOML
// ---------------------------------------------------------------------------

func TestMergeCodexConfigTOML_AddsMissingRootKeys(t *testing.T) {
	existing := "custom_key = \"keep\"\n"
	got := mergeCodexConfigTOML(existing)
	for _, want := range []string{
		`approval_policy = "never"`,
		`sandbox_mode = "workspace-write"`,
		`web_search = "cached"`,
		`custom_key = "keep"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q; content:\n%s", want, got)
		}
	}
}

func TestMergeCodexConfigTOML_AddsMissingSection(t *testing.T) {
	existing := "approval_policy = \"never\"\n"
	got := mergeCodexConfigTOML(existing)
	if !strings.Contains(got, "[sandbox_workspace_write]") {
		t.Errorf("missing [sandbox_workspace_write] section; got:\n%s", got)
	}
	if !strings.Contains(got, "network_access = false") {
		t.Errorf("missing network_access key; got:\n%s", got)
	}
	if !strings.Contains(got, "writable_roots = []") {
		t.Errorf("missing writable_roots key; got:\n%s", got)
	}
}

func TestMergeCodexConfigTOML_UpdatesExistingSectionKeys(t *testing.T) {
	existing := "[sandbox_workspace_write]\nnetwork_access = true\nwritable_roots = [\"/tmp\"]\n"
	got := mergeCodexConfigTOML(existing)
	if !strings.Contains(got, "network_access = false") {
		t.Errorf("network_access not updated to false; got:\n%s", got)
	}
	if !strings.Contains(got, "writable_roots = []") {
		t.Errorf("writable_roots not updated to []; got:\n%s", got)
	}
}

func TestMergeCodexConfigTOML_UpdatesExistingRootKeys(t *testing.T) {
	existing := `web_search = "live"
custom_key = "keep"
`
	got := mergeCodexConfigTOML(existing)
	if !strings.Contains(got, `web_search = "cached"`) {
		t.Errorf("web_search not updated; got:\n%s", got)
	}
	if !strings.Contains(got, `custom_key = "keep"`) {
		t.Errorf("custom_key not preserved; got:\n%s", got)
	}
}

func TestMergeCodexConfigTOML_Idempotent(t *testing.T) {
	// Run the full merge once, then run again on the output — result should be stable.
	first := mergeCodexConfigTOML("custom = \"value\"\n")
	second := mergeCodexConfigTOML(first)
	if first != second {
		t.Errorf("merge is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestMergeCodexConfigTOML_PreservesSectionOrderWhenSectionExists(t *testing.T) {
	existing := "[sandbox_workspace_write]\nnetwork_access = true\n"
	got := mergeCodexConfigTOML(existing)
	sectionIdx := strings.Index(got, "[sandbox_workspace_write]")
	wrIndex := strings.Index(got, "writable_roots")
	if wrIndex < sectionIdx {
		t.Errorf("writable_roots appears before section header:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// mergeGitignore (additional edge cases)
// ---------------------------------------------------------------------------

func TestMergeGitignore_EmptyExisting(t *testing.T) {
	got := mergeGitignore("", "# doug\n.doug/\n")
	if !strings.Contains(got, ".doug/") {
		t.Errorf("expected .doug/ in output; got: %q", got)
	}
}

func TestMergeGitignore_EmptyTemplate(t *testing.T) {
	existing := "node_modules/\n"
	got := mergeGitignore(existing, "")
	if got != existing {
		t.Errorf("expected unchanged output; got: %q", got)
	}
}

func TestMergeGitignore_BothEmpty(t *testing.T) {
	got := mergeGitignore("", "")
	if got != "" {
		t.Errorf("expected empty output; got: %q", got)
	}
}

func TestMergeGitignore_CommentsNotAppended(t *testing.T) {
	existing := "node_modules/\n"
	template := "# comment only\n"
	got := mergeGitignore(existing, template)
	if got != "node_modules/\n" {
		t.Errorf("comment-only template should not add entries; got: %q", got)
	}
}

func TestMergeGitignore_CRLFNormalized(t *testing.T) {
	existing := "node_modules/\r\n"
	template := ".doug/\r\n"
	got := mergeGitignore(existing, template)
	if strings.Contains(got, "\r\n") {
		t.Errorf("CRLF should be normalised; got: %q", got)
	}
	if !strings.Contains(got, ".doug/") {
		t.Errorf("expected .doug/ in output; got: %q", got)
	}
}

func TestMergeGitignore_TrailingNewlineAlwaysPresent(t *testing.T) {
	cases := []struct{ existing, template string }{
		{"a/\n", "b/\n"},
		{"a/\n", "a/\n"},
		{"", "a/\n"},
	}
	for _, tc := range cases {
		got := mergeGitignore(tc.existing, tc.template)
		if got != "" && !strings.HasSuffix(got, "\n") {
			t.Errorf("missing trailing newline for existing=%q template=%q; got: %q",
				tc.existing, tc.template, got)
		}
	}
}
