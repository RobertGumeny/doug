package cmd

import (
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "doug",
	Short: "Run coding-agent tasks with deterministic validation",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		writeln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// If ldflags didn't inject a version (local go build/go run), fall back to
	// the module version embedded by the Go toolchain (set when using go install
	// with a tagged release).
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	rootCmd.Version = version
	rootCmd.InitDefaultVersionFlag()
	rootCmd.Flags().Lookup("version").Shorthand = "v"
	cobra.EnableCommandSorting = false
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(handoffCmd)
	rootCmd.AddCommand(researchCmd)
	rootCmd.AddCommand(scaffoldCmd)
	rootCmd.AddCommand(revertCmd)
	rootCmd.AddCommand(upgradeCmd)
}
