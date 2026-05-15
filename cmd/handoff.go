package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/plan"
)

var handoffNow = currentTimeUTC

var handoffCmd = &cobra.Command{
	Use:   "handoff",
	Short: "Package approved plan output for execution",
	Long:  "Turn approved work from .doug/plan/PLAN.md into execution-ready epics, and generate a scaffold manifest when the plan includes greenfield bootstrap work.",
	RunE:  runHandoff,
}

func runHandoff(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := plan.HandoffProjectPlan(projectRoot, handoffNow())
	if err != nil {
		return err
	}

	writef(cmd.OutOrStdout(), "Generated %d epic package(s) in .doug/plan/epics/\n", result.EpicCount)
	if result.ManifestGenerated {
		writeln(cmd.OutOrStdout(), "Generated .doug/plan/manifest.yaml")
	}
	return nil
}

func currentTimeUTC() time.Time {
	return time.Now().UTC()
}
