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
	Short: "Generate backlog epics from .doug/plan/PLAN.md",
	Long:  "Parse the structured handoff data in .doug/plan/PLAN.md, emit deterministic backlog epic packages under .doug/plan/epics/, and derive .doug/plan/manifest.yaml when scaffold data is present.",
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
