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
	"github.com/robertgumeny/doug/internal/types"
)

const (
	planTaskID = "PLAN"
)

var (
	planLoadConfig               = config.LoadConfig
	planRunAgent   agent.Backend = agent.DefaultBackend{}
)

var planFlags struct {
	intent string
	mode   string
	epic   string
}

var planCmd = &cobra.Command{
	Use:   "plan [planning-intent...]",
	Short: "Create or refine .doug/plan/PLAN.md with the configured planning skill",
	Long:  "Create .doug/plan/PLAN.md when missing, brief the configured provider through .doug/ACTIVE_TASK.md, and keep PLAN.md as the editable planning workbook while reserving deterministic derivative artifacts for doug handoff.",
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

	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:      planTaskID,
		TaskType:    types.TaskType("plan"),
		DougDir:     paths.DougDir,
		Description: "Refine .doug/plan/PLAN.md as the planning workbook for this Doug-managed run.",
		AcceptanceCriteria: []string{
			"Update `.doug/plan/PLAN.md` directly as the planning workbook for this run.",
			"Keep the workbook narrative and `## Handoff Data` aligned when the plan is handoff-ready.",
			"Treat `PLAN.md` and any generated handoff artifacts as downstream working artifacts rather than competing canonical briefs.",
		},
		Attempts:   1,
		MaxRetries: 1,
		ContextSections: []agent.ActiveTaskSection{
			{
				Heading: "Planning Workbook",
				Body: "" +
					"- Canonical brief for this run: `.doug/ACTIVE_TASK.md`\n" +
					"- Editable planning workbook: `.doug/plan/PLAN.md`\n" +
					"- Read the Doug-owned planning context already written into `PLAN.md`, then update that workbook directly.\n" +
					"- Keep backlog packages, `manifest.yaml`, and any other generated outputs downstream from this brief and workbook.\n",
			},
		},
	}, logger); err != nil {
		return fmt.Errorf("write planning active task: %w", err)
	}

	resolvedCmd := resolvePlanAgentCommand(cfg.PlanAgentCommand, skillName, planTaskID)

	logger.Info("invoking agent for planning")
	planPath := filepath.Join(projectRoot, ".doug", "plan", "PLAN.md")
	activeTaskPath := filepath.Join(projectRoot, ".doug", "ACTIVE_TASK.md")
	_, err = planRunAgent.Run(ctx, agent.RunRequest{
		Phase: agent.RunPhasePlanning,
		Task: agent.TaskContext{
			ID:         planTaskID,
			Type:       "plan",
			Attempt:    1,
			MaxRetries: 1,
		},
		Brief: agent.CanonicalBrief{
			Path:      activeTaskPath,
			Format:    agent.BriefFormatMarkdown,
			Authority: "doug",
		},
		ContextLoadOrder: []agent.ContextInput{
			{Kind: agent.ContextInputProjectInstructions, Path: filepath.Join(projectRoot, "AGENTS.md"), Required: false},
			{Kind: agent.ContextInputProductContext, Path: filepath.Join(projectRoot, ".doug", "PRD.md"), Required: false},
			{Kind: agent.ContextInputCanonicalBrief, Path: activeTaskPath, Required: true},
			{Kind: agent.ContextInputWorkingArtifact, Path: planPath, Required: true},
		},
		Routing: agent.RoutingInputs{
			Workflow:  "plan",
			SkillName: skillName,
		},
		Policy: agent.PolicyInputs{},
		Restrictions: agent.RestrictionHooks{
			Read:  agent.RestrictionHook{Mode: agent.RestrictionModeInherit},
			Write: agent.RestrictionHook{Mode: agent.RestrictionModeAllowList, Paths: []string{activeTaskPath, planPath}},
		},
		Command:     resolvedCmd,
		ProjectRoot: projectRoot,
	})
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
	return resolved
}
