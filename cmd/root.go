package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "v0.4.0"

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
	rootCmd.Version = version
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(initCmd)
}
