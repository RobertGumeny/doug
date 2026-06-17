package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/orchestrator"
	runstats "github.com/robertgumeny/doug/internal/stats"
)

var statsCmd = &cobra.Command{
	Use:   "stats [epic_id]",
	Short: "Show local Doug run statistics",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runStats,
}

func runStats(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	paths := orchestrator.NewPaths(projectRoot)
	epicID := ""
	if len(args) == 1 {
		epicID = args[0]
	}
	summary, err := runstats.LoadSummary(paths.LogsDir, epicID)
	if err != nil {
		return err
	}
	printStatsSummary(cmd.OutOrStdout(), summary, epicID)
	return nil
}

func printStatsSummary(w io.Writer, summary runstats.Summary, epicID string) {
	if len(summary.Rows) == 0 {
		if epicID == "" {
			writeln(w, "No Doug stats records found.")
			return
		}
		writef(w, "No Doug stats records found for %s.\n", epicID)
		return
	}

	if epicID == "" {
		writeln(w, "Doug stats")
	} else {
		writef(w, "Doug stats for %s\n", epicID)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	writef(tw, "EPIC\tTASK\tRUNS\tCOST\tINPUT\tOUTPUT\tCACHE\tDURATION\tFIRST_RESPONSE\n")
	for _, row := range summary.Rows {
		writef(tw, "%s\t%s\t%d\t$%.4f\t%d\t%d\t%d\t%s\t%s\n",
			row.EpicID,
			row.TaskID,
			row.Runs,
			row.CostUSD,
			row.InputTokens,
			row.OutputTokens,
			row.CacheTokens,
			formatStatsDuration(row.DurationMs),
			formatStatsDuration(row.FirstResponseMs),
		)
	}
	writef(tw, "TOTAL\t-\t%d\t$%.4f\t%d\t%d\t%d\t%s\t%s avg\n",
		summary.Totals.Runs,
		summary.Totals.CostUSD,
		summary.Totals.InputTokens,
		summary.Totals.OutputTokens,
		summary.Totals.CacheTokens,
		formatStatsDuration(summary.Totals.DurationMs),
		formatStatsDuration(summary.Totals.FirstResponseMs),
	)
	_ = tw.Flush()
}

func formatStatsDuration(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return d.String()
	}
	return d.Round(time.Millisecond).String()
}
