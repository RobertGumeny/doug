package agent

import (
	"encoding/json"
	"fmt"
	"os"
)

type runMetadata struct {
	Status                RunStatus              `json:"status"`
	DurationMilliseconds  int64                  `json:"duration_ms"`
	ExitCode              *int                   `json:"exit_code,omitempty"`
	SessionID             string                 `json:"session_id,omitempty"`
	AvailableSessionIDs   []string               `json:"available_session_ids,omitempty"`
	RestrictionViolations []RestrictionViolation `json:"restriction_violations,omitempty"`
	FirstResponseMs       int64                  `json:"first_response_ms,omitempty"`
	ToolCallCount         int                    `json:"tool_call_count,omitempty"`
	ProviderFailures      int                    `json:"provider_failures,omitempty"`
	Error                 string                 `json:"error,omitempty"`
}

// RunMetadataPath returns the sidecar metadata path for an output log.
func RunMetadataPath(outputLogPath string) string {
	return outputLogPath + ".meta.json"
}

// WriteRunMetadata persists runtime-only backend facts next to a Doug-managed
// output log so adapter-visible state can be inspected after the run.
func WriteRunMetadata(outputLogPath string, resp RunResponse, runErr error) error {
	meta := runMetadata{
		Status:                resp.Status,
		DurationMilliseconds:  resp.Duration.Milliseconds(),
		ExitCode:              resp.ExitCode,
		SessionID:             resp.SessionID,
		AvailableSessionIDs:   resp.AvailableSessionIDs,
		RestrictionViolations: resp.RestrictionViolations,
		FirstResponseMs:       resp.FirstResponseMs,
		ToolCallCount:         resp.ToolCallCount,
		ProviderFailures:      resp.ProviderFailures,
	}
	if runErr != nil {
		meta.Error = runErr.Error()
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run metadata: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(RunMetadataPath(outputLogPath), data, 0o644); err != nil {
		return fmt.Errorf("write run metadata: %w", err)
	}
	return nil
}
