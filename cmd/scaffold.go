package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/types"
)

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Validate scaffold preconditions and manifest input",
	Long:  "Validate that doug init has run and that .doug/plan/manifest.yaml exists and passes schema validation.",
	RunE:  runScaffold,
}

func runScaffold(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	return scaffoldProject(projectRoot)
}

// scaffoldProject is the testable core of the scaffold command shell.
// This task only establishes the command entrypoint and precondition guards.
func scaffoldProject(projectRoot string) error {
	paths := orchestrator.NewPaths(projectRoot)

	if _, err := os.Stat(paths.StatePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s not found — run doug init first", paths.StatePath)
		}
		return fmt.Errorf("stat %s: %w", paths.StatePath, err)
	}

	if _, err := types.LoadManifest(paths.ManifestPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s not found — generate .doug/plan/manifest.yaml before running doug scaffold", paths.ManifestPath)
		}
		return err
	}

	return nil
}
