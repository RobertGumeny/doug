package cmd

import (
	"encoding/json"
	"strings"
)

// dougInstructionsMarker and dougInstructionsEndMarker delimit the managed
// doug-specific block inside a project's AGENTS.md.
const dougInstructionsMarker = "<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->"
const dougInstructionsEndMarker = "<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->"

// mergeGitignore returns a .gitignore that contains all lines from existing
// plus any non-comment, non-blank lines from template that are not already
// present. If existing is empty the template is returned as-is.
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

// mergeAgents returns the content to write to AGENTS.md. When the doug marker
// is absent the dougSection is appended. When the marker is already present,
// ensureMetadataInBlock injects project metadata if missing and leaves the
// rest of the block unchanged.
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

// normalizeText normalises line endings to LF, trims trailing newlines, and
// appends a single trailing newline. Empty input is returned unchanged.
func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	return s + "\n"
}

// mergeJSONSettings deep-merges the managed template into the existing JSON
// object, union-merging string arrays and recursively merging nested objects.
// Returns the re-serialised JSON with a trailing newline.
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

// deepMergeJSON merges src into dst in place. Nested maps are merged
// recursively; string arrays are union-merged; all other values from src
// overwrite dst.
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

// mergeStringArrays returns a deduplicated union of existing and managed,
// preserving order (existing entries first). Returns (nil, false) if either
// slice contains a non-string element.
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

