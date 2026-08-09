package status

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

type testLogger struct{ infos []string }

func (l *testLogger) Info(msg string) { l.infos = append(l.infos, msg) }

func TestIndicator_NonTTYUsesLineHeartbeatLogsOnly(t *testing.T) {
	logger := &testLogger{}
	var out bytes.Buffer
	ind := New(Options{TaskID: "TASK-1", Delay: time.Second, Writer: &out, TTY: false, Logger: logger})

	ind.Heartbeat(2*time.Second, "reading file")
	ind.Finish()

	if out.Len() != 0 {
		t.Fatalf("TTY output = %q, want empty", out.String())
	}
	if got, want := logger.infos, []string{"[TASK-1] +2s — reading file"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("logger infos = %v, want %v", got, want)
	}
}

func TestIndicator_TTYAppearsAfterDelayWithElapsedHintAndNoHeartbeatLogs(t *testing.T) {
	logger := &testLogger{}
	var out bytes.Buffer
	ind := New(Options{TaskID: "TASK-1", Delay: 3 * time.Second, Writer: &out, TTY: true, Logger: logger})

	ind.Heartbeat(2*time.Second, "reading file")
	if out.Len() != 0 {
		t.Fatalf("status before delay = %q, want empty", out.String())
	}
	ind.Heartbeat(3*time.Second, "reading file")

	got := out.String()
	for _, want := range []string{"TASK-1", "+3s", "reading file", "Ctrl-C to interrupt"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output %q missing %q", got, want)
		}
	}
	if len(logger.infos) != 0 {
		t.Fatalf("TTY logger infos = %v, want no per-heartbeat log lines", logger.infos)
	}
}

func TestFormatAgentEndSummary_RoundsAndBoundsDurations(t *testing.T) {
	got := FormatAgentEndSummary(125*time.Second+600*time.Millisecond, 1499, 3, 4)
	want := "agent finished in 2m 6s — first response +1s, 3 tool calls, 4 provider failures"
	if got != want {
		t.Fatalf("FormatAgentEndSummary() = %q, want %q", got, want)
	}

	got = FormatAgentEndSummary(-time.Second, -1, 0, 0)
	want = "agent finished in 0m 0s — first response +0s, 0 tool calls, 0 provider failures"
	if got != want {
		t.Fatalf("FormatAgentEndSummary(negative) = %q, want %q", got, want)
	}
}

func TestSanitizeActivity_BoundsOneLineAndStripsTerminalControl(t *testing.T) {
	got := SanitizeActivity("\x1b[31mtool\x1b[0m\nsecret\r\t"+strings.Repeat("x", 120), "fallback")
	if strings.ContainsAny(got, "\n\r\t\x1b") {
		t.Fatalf("sanitized activity contains raw control characters: %q", got)
	}
	if !strings.HasPrefix(got, "tool secret ") {
		t.Fatalf("sanitized activity = %q, want normalized content", got)
	}
	if len([]rune(got)) > maxActivityRunes {
		t.Fatalf("sanitized activity length = %d, want <= %d", len([]rune(got)), maxActivityRunes)
	}
}

func TestSanitizeActivity_FallsBackWhenEmpty(t *testing.T) {
	if got := SanitizeActivity("\n\r\t", "waiting safely"); got != "waiting safely" {
		t.Fatalf("fallback activity = %q", got)
	}
}
