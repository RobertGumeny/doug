package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPriorGapCommandsExposeWorkflowHelp(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *cobra.Command
		long    string
		example string
	}{
		{
			name:    "mcp",
			cmd:     mcpCmd,
			long:    "lifecycle tools such as status, reconciliation, task claiming, and completion reporting",
			example: "doug mcp",
		},
		{
			name:    "review",
			cmd:     reviewCmd,
			long:    "post-epic advisory review again for an archived completed epic",
			example: "doug review EPIC-3",
		},
		{
			name:    "stats",
			cmd:     statsCmd,
			long:    "Summarize local run telemetry from .doug/logs",
			example: "doug stats EPIC-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.cmd.Long, tt.long) {
				t.Fatalf("Long help missing %q; got:\n%s", tt.long, tt.cmd.Long)
			}
			if !strings.Contains(tt.cmd.Example, tt.example) {
				t.Fatalf("example help missing %q; got:\n%s", tt.example, tt.cmd.Example)
			}
		})
	}
}

func TestBuildSystemFlagHelpListsSupportedSystems(t *testing.T) {
	for _, tc := range []struct {
		name  string
		usage string
	}{
		{name: "init", usage: initCmd.Flags().Lookup("build-system").Usage},
		{name: "run", usage: runCmd.Flags().Lookup("build-system").Usage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(tc.usage, "go|npm|pnpm|static") {
				t.Fatalf("build-system flag help = %q, want supported list go|npm|pnpm|static", tc.usage)
			}
		})
	}
}
