package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/plan"
)

const (
	planTaskID = "PLAN"
)

var (
	planLoadConfig = config.LoadConfig
	planRunAgent   = agent.RunAgent
)

var planFlags struct {
	intent string
	mode   string
	epic   string
}

var planCmd = &cobra.Command{
	Use:   "plan [planning-intent...]",
	Short: "Create or refine .doug/plan/PLAN.md with the configured planning skill",
	Long:  "Create .doug/plan/PLAN.md when missing, brief the configured provider with the plan skill, and keep planning centered on PLAN.md while reserving deterministic derivative artifacts for doug handoff.",
	Args:  cobra.ArbitraryArgs,
	RunE:  runPlan,
}

type planRunContext struct {
	Intent string
	Mode   string
	Epic   string
}

func init() {
	planCmd.Flags().StringVar(&planFlags.intent, "intent", "", "explicit planning objective for this run")
	planCmd.Flags().StringVar(&planFlags.mode, "mode", "", "planning mode hint (discovery|roadmapping|definition|feature|refactor|bugfix|greenfield)")
	planCmd.Flags().StringVar(&planFlags.epic, "epic", "", "target epic hint for this planning run")
}

func runPlan(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	runCtx, err := resolvePlanRunContext(cmd, args)
	if err != nil {
		return err
	}

	return planProjectContext(cmd.Context(), projectRoot, cmd.OutOrStdout(), runCtx)
}

func planProjectContext(ctx context.Context, projectRoot string, outWriter io.Writer, runCtx planRunContext) error {
	paths := orchestrator.NewPaths(projectRoot)
	logger := log.New()

	cfg, err := planLoadConfig(paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	archivedBugs, err := plan.LoadArchivedBugContext(projectRoot)
	if err != nil {
		return fmt.Errorf("load archived bug planning context: %w", err)
	}

	_, created, err := plan.EnsurePlanDocument(paths.DougDir, plan.WorkbookContext{
		PlanningIntent: runCtx.Intent,
		PlanningMode:   runCtx.Mode,
		TargetEpicHint: runCtx.Epic,
		ArchivedBugs:   archivedBugs,
	})
	if err != nil {
		return err
	}

	skillName, err := agent.GetSkillForTaskType("plan", paths.SkillsConfigPath)
	if err != nil {
		return fmt.Errorf("resolve plan skill: %w", err)
	}

	if created {
		writef(outWriter, "Created %s\n", filepath.ToSlash(filepath.Join(".doug", "plan", "PLAN.md")))
	} else {
		writef(outWriter, "Using existing %s\n", filepath.ToSlash(filepath.Join(".doug", "plan", "PLAN.md")))
	}

	resolvedCmd := resolvePlanAgentCommand(cfg.PlanAgentCommand, skillName, planTaskID)

	logger.Info("invoking agent for planning")
	_, err = planRunAgent(ctx, resolvedCmd, projectRoot, 0, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func resolvePlanRunContext(cmd *cobra.Command, args []string) (planRunContext, error) {
	intentFromArgs := strings.TrimSpace(strings.Join(args, " "))
	intentFromFlag := strings.TrimSpace(planFlags.intent)
	if intentFromArgs != "" && intentFromFlag != "" && intentFromArgs != intentFromFlag {
		return planRunContext{}, fmt.Errorf("planning intent provided twice with different values; use either positional intent or --intent")
	}

	mode := strings.ToLower(strings.TrimSpace(planFlags.mode))
	if mode != "" {
		validModes := []string{"discovery", "roadmapping", "definition", "feature", "refactor", "bugfix", "greenfield"}
		if !slices.Contains(validModes, mode) {
			return planRunContext{}, fmt.Errorf("invalid planning mode %q; want one of: %s", mode, strings.Join(validModes, ", "))
		}
	}

	intent := intentFromFlag
	if intent == "" {
		intent = intentFromArgs
	}

	return planRunContext{
		Intent: intent,
		Mode:   mode,
		Epic:   strings.TrimSpace(planFlags.epic),
	}, nil
}
func resolvePlanAgentCommand(agentCommand, skillName, taskID string) string {
	resolved := strings.ReplaceAll(agentCommand, "{{skill_name}}", skillName)
	resolved = strings.ReplaceAll(resolved, "{{task_id}}", taskID)

	runtimePrompt := config.RuntimePrompt
	planPrompt := config.PlanPrompt
	legacyRuntimePrompt := "This is a doug-orchestrated run: use .doug/ACTIVE_TASK.md as the task brief and complete the task described there."

	if strings.Contains(resolved, runtimePrompt) {
		return strings.ReplaceAll(resolved, runtimePrompt, planPrompt)
	}
	if strings.Contains(resolved, legacyRuntimePrompt) {
		return strings.ReplaceAll(resolved, legacyRuntimePrompt, planPrompt)
	}

	if strings.Contains(resolved, ".doug/ACTIVE_TASK.md") {
		resolved = strings.ReplaceAll(resolved, ".doug/ACTIVE_TASK.md", ".doug/plan/PLAN.md")
	}
	if strings.Contains(resolved, "complete the task described there.") {
		resolved = strings.ReplaceAll(resolved, "complete the task described there.", strings.TrimPrefix(planPrompt, "This is a doug-orchestrated planning run: use .doug/plan/PLAN.md as the planning workbook. "))
	}
	return resolved
}
