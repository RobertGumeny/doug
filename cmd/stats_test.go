package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	runstats "github.com/robertgumeny/doug/internal/stats"
	"github.com/spf13/cobra"
)

func TestRunStatsPrintsFilteredSummary(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	logsDir := filepath.Join(dir, ".doug", "logs")
	if _, err := runstats.WriteRunStats(logsDir, "EPIC-45", runstats.RunStats{TaskID: "EPIC-45-001", Attempt: 1, InputTokens: 10, OutputTokens: 5, CacheTokens: 2, CostUSD: 0.01, DurationMs: 1500, FirstResponseMs: 250}); err != nil {
		t.Fatalf("WriteRunStats EPIC-45: %v", err)
	}
	if _, err := runstats.WriteRunStats(logsDir, "EPIC-46", runstats.RunStats{TaskID: "EPIC-46-001", Attempt: 1, InputTokens: 99, CostUSD: 0.99, DurationMs: 9000, FirstResponseMs: 900}); err != nil {
		t.Fatalf("WriteRunStats EPIC-46: %v", err)
	}

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runStats(cmd, []string{"EPIC-45"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}

	got := out.String()
	for _, want := range []string{"Doug stats for EPIC-45", "EPIC-45-001", "$0.0100", "10", "5", "2", "1.5s", "250ms", "TOTAL"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
	if strings.Contains(got, "EPIC-46-001") {
		t.Fatalf("filtered output included EPIC-46 row:\n%s", got)
	}
}

func TestStatsCommandRegistered(t *testing.T) {
	for _, command := range rootCmd.Commands() {
		if command.Name() == "stats" {
			return
		}
	}
	t.Fatal("stats command is not registered on root command")
}
