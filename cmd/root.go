package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "doug",
	Short: "doug is a task automation CLI",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(switchCmd)
}
