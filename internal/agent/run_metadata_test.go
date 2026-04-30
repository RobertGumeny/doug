package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteRunMetadata(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "output-task.log")
	resp := RunResponse{
		Status:              RunStatusCompleted,
		Duration:            1500 * time.Millisecond,
		SessionID:           "pi-session-123",
		AvailableSessionIDs: []string{"pi-session-123", "pi-session-456"},
	}
	exitCode := 0
	resp.ExitCode = &exitCode

	if err := WriteRunMetadata(logPath, resp, nil); err != nil {
		t.Fatalf("WriteRunMetadata: %v", err)
	}

	data, err := os.ReadFile(RunMetadataPath(logPath))
	if err != nil {
		t.Fatalf("read run metadata: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		`"status": "completed"`,
		`"duration_ms": 1500`,
		`"session_id": "pi-session-123"`,
		`"available_session_ids": [`,
		`"pi-session-456"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected metadata to contain %q, got:\n%s", want, content)
		}
	}
}
