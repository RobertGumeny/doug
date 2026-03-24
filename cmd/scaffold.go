package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/types"
)

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Construct the synthetic scaffold task and write ACTIVE_TASK.md",
	Long:  "Validate scaffold preconditions, build the synthetic scaffold task from .doug/plan/manifest.yaml, and write .doug/ACTIVE_TASK.md for the agent.",
	RunE:  runScaffold,
}

func runScaffold(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	return scaffoldProject(projectRoot)
}

func scaffoldProject(projectRoot string) error {
	paths := orchestrator.NewPaths(projectRoot)

	if _, err := os.Stat(paths.StatePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s not found — run doug init first", paths.StatePath)
		}
		return fmt.Errorf("stat %s: %w", paths.StatePath, err)
	}

	manifest, err := types.LoadManifest(paths.ManifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s not found — generate .doug/plan/manifest.yaml before running doug scaffold", paths.ManifestPath)
		}
		return err
	}

	task, err := buildScaffoldTask(manifest)
	if err != nil {
		return err
	}

	if _, err := agent.GetSkillForTaskType(string(task.Type), paths.SkillsConfigPath); err != nil {
		return fmt.Errorf("resolve scaffold skill: %w", err)
	}

	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:             task.ID,
		TaskType:           task.Type,
		DougDir:            paths.DougDir,
		Description:        task.Description,
		AcceptanceCriteria: task.AcceptanceCriteria,
		Attempts:           1,
		MaxRetries:         1,
		ContextSections: []agent.ActiveTaskSection{
			{
				Heading: "Manifest Context",
				Body:    manifestContextBody(manifest),
			},
		},
	}, log.Discard()); err != nil {
		return fmt.Errorf("write scaffold active task: %w", err)
	}

	return nil
}

func buildScaffoldTask(manifest *types.Manifest) (types.Task, error) {
	if manifest == nil {
		return types.Task{}, fmt.Errorf("build scaffold task: manifest is required")
	}

	descriptorParts := []string{
		manifest.Scaffold.Language,
		manifest.Scaffold.Runtime,
		manifest.Scaffold.Framework,
	}

	return types.Task{
		ID:     "SCAFFOLD",
		Type:   types.TaskTypeScaffold,
		Status: types.StatusInProgress,
		Description: fmt.Sprintf(
			"Materialize the initial %s project scaffold for %q using %s.",
			manifest.Project.Mode,
			manifest.Project.Name,
			strings.Join(descriptorParts, "/"),
		),
		AcceptanceCriteria: []string{
			"Create the day-0 project scaffold described by the manifest.",
			"Install the requested dependencies and package manager layout.",
			"Honor every manifest constraint provided in the structured context below.",
		},
	}, nil
}

func manifestContextBody(manifest *types.Manifest) string {
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Sprintf("Failed to render manifest context: %v\n", err)
	}

	return "The following manifest is the source of truth for this scaffold task.\n\n```yaml\n" + string(data) + "```\n"
}
