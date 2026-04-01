package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/types"
)

const (
	planTaskID   = "PLAN"
	planFileBody = "# Project Plan\n\n" +
		"Use this document as the primary planning artifact for the project.\n\n" +
		"## Intent\n\n" +
		"- Capture free-form planning notes, assumptions, and open questions here.\n" +
		"- Refine the plan until it is ready for deterministic handoff.\n" +
		"- Keep derivative backlog artifacts out of this document until `doug handoff` generates them.\n\n" +
		"## Draft Notes\n\n" +
		"Document the current plan in whatever level of detail is useful.\n\n" +
		"## Handoff Readiness\n\n" +
		"When the plan is ready, ensure the deterministic handoff payload lives under `## Handoff Data` as fenced YAML.\n\n" +
		"## Handoff Data\n\n" +
		"```yaml\n" +
		"schema_version: 1\n" +
		"project:\n" +
		"  name: \"\"\n" +
		"  mode: \"\"\n" +
		"epics: []\n" +
		"```\n"
)

var (
	planLoadConfig = config.LoadConfig
	planRunAgent   = agent.RunAgent
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Create or refine .doug/plan/PLAN.md with the configured planning skill",
	Long:  "Create .doug/plan/PLAN.md when missing, brief the configured provider with the plan skill, and keep planning centered on PLAN.md while reserving deterministic derivative artifacts for doug handoff.",
	RunE:  runPlan,
}

func runPlan(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	return planProjectContext(cmd.Context(), projectRoot, cmd.OutOrStdout())
}

func planProject(projectRoot string) error {
	return planProjectContext(context.Background(), projectRoot, io.Discard)
}

func planProjectContext(ctx context.Context, projectRoot string, outWriter io.Writer) error {
	paths := orchestrator.NewPaths(projectRoot)
	logger := log.New()

	cfg, err := planLoadConfig(paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	planPath, created, err := ensurePlanDocument(paths.DougDir)
	if err != nil {
		return err
	}

	skillName, err := agent.GetSkillForTaskType("plan", paths.SkillsConfigPath)
	if err != nil {
		return fmt.Errorf("resolve plan skill: %w", err)
	}

	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:      planTaskID,
		TaskType:    types.TaskType("plan"),
		DougDir:     paths.DougDir,
		Attempts:    1,
		MaxRetries:  1,
		BuildSystem: cfg.BuildSystem,
		Description: "Create or refine .doug/plan/PLAN.md as the free-form planning artifact for the repository.",
		AcceptanceCriteria: []string{
			"Use .doug/plan/PLAN.md as the single primary planning artifact.",
			"Keep planning free-form while targeting the deterministic handoff contract.",
			"Do not create or rely on derivative backlog artifacts; doug handoff owns epic packages and manifest.yaml generation.",
		},
		ContextSections: []agent.ActiveTaskSection{
			{
				Heading: "Planning Artifact Contract",
				Body: "Treat `.doug/plan/PLAN.md` as the only required planning artifact.\n\n" +
					"- You may rewrite or expand `PLAN.md` freely.\n" +
					"- Keep planning notes, structure, and handoff-ready content in `PLAN.md`.\n" +
					"- Do not create required stage files or alternate primary planning documents.\n" +
					"- `doug handoff` owns deterministic derivatives such as `.doug/plan/epics/<EPIC-ID>/` packages and `.doug/plan/manifest.yaml`.\n",
			},
			{
				Heading: "Plan File Path",
				Body:    fmt.Sprintf("Update `%s` directly. It already exists at this path and should remain the source of truth for planning.\n", planPath),
			},
		},
	}, logger); err != nil {
		return fmt.Errorf("write plan active task: %w", err)
	}

	if created {
		writef(outWriter, "Created %s\n", filepath.ToSlash(filepath.Join(".doug", "plan", "PLAN.md")))
	} else {
		writef(outWriter, "Using existing %s\n", filepath.ToSlash(filepath.Join(".doug", "plan", "PLAN.md")))
	}

	resolvedCmd := strings.ReplaceAll(cfg.AgentCommand, "{{skill_name}}", skillName)
	resolvedCmd = strings.ReplaceAll(resolvedCmd, "{{task_id}}", planTaskID)

	logger.Info("invoking agent for planning")
	heartbeatEvery := time.Duration(cfg.AgentHeartbeatSeconds) * time.Second
	_, err = planRunAgent(ctx, resolvedCmd, projectRoot, heartbeatEvery, func(elapsed time.Duration) {
		logger.Info(fmt.Sprintf("[%s] +%s", planTaskID, elapsed.Round(time.Second)))
	}, nil)
	if err != nil {
		return err
	}

	return nil
}

func ensurePlanDocument(dougDir string) (string, bool, error) {
	planDir := filepath.Join(dougDir, "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create plan directory %s: %w", planDir, err)
	}

	planPath := filepath.Join(planDir, "PLAN.md")
	if _, err := os.Stat(planPath); err == nil {
		return planPath, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, fmt.Errorf("stat %s: %w", planPath, err)
	}

	if err := state.AtomicWrite(planPath, []byte(planFileBody)); err != nil {
		return "", false, fmt.Errorf("write %s: %w", planPath, err)
	}

	return planPath, true, nil
}
