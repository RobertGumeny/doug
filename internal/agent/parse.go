package agent

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/types"
)

// ErrNoFrontmatter is returned when the session file contains no YAML
// frontmatter delimiters (--- ... ---).
var ErrNoFrontmatter = errors.New("no YAML frontmatter found")

// ErrMissingOutcome is returned when the frontmatter is valid YAML but the
// outcome field is absent or empty.
var ErrMissingOutcome = errors.New("outcome field is missing or empty")

// ErrInvalidOutcome is returned when the outcome field is present but does not
// match one of the four valid values (SUCCESS, BUG, FAILURE, EPIC_COMPLETE).
type ErrInvalidOutcome struct {
	Value string
}

func (e *ErrInvalidOutcome) Error() string {
	return fmt.Sprintf("invalid outcome %q: must be one of SUCCESS, BUG, FAILURE, EPIC_COMPLETE", e.Value)
}

// ParseSessionResult reads the session file at filePath, extracts the YAML
// frontmatter between the first and second --- delimiter lines, and unmarshals
// it into a SessionResult. Both CRLF and LF line endings are handled.
//
// Typed errors are returned for each failure mode:
//   - os.ErrNotExist      – file not found (errors.Is compatible)
//   - ErrNoFrontmatter    – no --- delimiters found
//   - ErrMissingOutcome   – outcome field absent or empty
//   - *ErrInvalidOutcome  – outcome is not one of the four valid values
func ParseSessionResult(filePath string) (*types.SessionResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err // caller uses errors.Is(err, os.ErrNotExist)
	}

	// Normalise to LF so scanning works the same on Windows and Unix.
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")

	// Find the first --- delimiter.
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			start = i
			break
		}
	}
	if start == -1 {
		return nil, ErrNoFrontmatter
	}

	// Find the second --- delimiter.
	end := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, ErrNoFrontmatter
	}

	frontmatter := strings.Join(lines[start+1:end], "\n")

	var result types.SessionResult
	if err := yaml.Unmarshal([]byte(frontmatter), &result); err != nil {
		return nil, fmt.Errorf("unmarshal frontmatter: %w", err)
	}

	if result.Outcome == "" {
		return nil, ErrMissingOutcome
	}

	switch result.Outcome {
	case types.OutcomeSuccess, types.OutcomeBug, types.OutcomeFailure, types.OutcomeEpicComplete:
		// valid
	default:
		return nil, &ErrInvalidOutcome{Value: string(result.Outcome)}
	}

	return &result, nil
}
