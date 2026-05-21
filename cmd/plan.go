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
	"github.com/robertgumeny/doug/internal/interactive"
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
	planRunPiInteractive         piInteractiveLauncher // nil in production; tests inject a stub
	planNewPiInteractiveLauncher = func() piInteractiveLauncher { return agent.NewPiInteractiveLauncher() }
	planIsInteractive            = interactive.IsInteractive
	planNewPrompter              = func() planningIntentPrompter { return interactive.New() }
)

type planningIntentPrompter interface {
	Compose(header string, defaultVal string) (string, error)
}

type piInteractiveLauncher interface {
	Run(context.Context, agent.PiInteractiveLaunchRequest) (agent.RunResponse, error)
}

var planFlags struct {
	intent string
	mode   string
	epic   string
}

var planCmd = &cobra.Command{
	Use:   "plan [planning-intent...]",
	Short: "Shape work in an optional planning workbook",
	Long:  "Use an optional planning workbook to explore scope, break work into epics, and prepare execution-ready tasks before you run them. Doug keeps that workbook in .doug/plan/PLAN.md; use doug handoff when you're ready to package approved plan output for execution.",
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

	prep, err := agent.PrepareExecution(string(agent.RunPhasePlanning), "plan", planTaskID, cfg.Policy)
	if err != nil {
		return fmt.Errorf("prepare plan execution: %w", err)
	}

	if created {
		writef(outWriter, "Created %s\n", filepath.ToSlash(filepath.Join(".doug", "plan", "PLAN.md")))
	} else {
		writef(outWriter, "Using existing %s\n", filepath.ToSlash(filepath.Join(".doug", "plan", "PLAN.md")))
	}

	contextSections := []agent.ActiveTaskSection{
		{
			Heading: "Planning Workbook",
			Body: "" +
				"- Canonical brief for this run: `.doug/ACTIVE_TASK.md`\n" +
				"- Editable planning workbook: `.doug/plan/PLAN.md`\n" +
				"- Read the Doug-owned planning context already written into `PLAN.md`, then update that workbook directly.\n" +
				"- `.doug/plan/PLAN.md` is the source of truth for this planning cycle; do not treat derivative artifacts under `.doug/plan/epics/` or `.doug/plan/manifest.yaml` as competing briefs.\n" +
				"- Before writing final handoff data into `.doug/plan/PLAN.md`, produce an alignment summary covering resolved intent, scope decisions, epic sequence, and remaining open questions; do not write machine-consumable handoff YAML until the user has explicitly confirmed the summary.\n",
		},
	}
	if ws := agent.WriteScopeSection(prep.Exec.WriteScopes); ws != nil {
		contextSections = append(contextSections, *ws)
	}

	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:      planTaskID,
		TaskType:    types.TaskType("plan"),
		DougDir:     paths.DougDir,
		Description: "Refine .doug/plan/PLAN.md as the planning workbook for this Doug-managed run.",
		AcceptanceCriteria: []string{
			"Update `.doug/plan/PLAN.md` directly as the planning workbook for this run.",
			"Prefer codebase and KB lookup over asking the user when clarifying scope, constraints, or intent; ask one question at a time when material ambiguity remains after lookup.",
			"Before finalizing handoff-ready epics and tasks, produce an explicit alignment summary — resolved intent, scope decisions, epic sequence, and remaining open questions — and advance to handoff data only after the user confirms.",
			"Promote execution-relevant constraints, risks, or architectural decisions discovered during planning into the epic PRD or task contracts rather than leaving them only in workbook narrative.",
			"Keep the workbook narrative and `## Handoff Data` aligned when the plan is handoff-ready.",
			"Treat `PLAN.md` and any generated handoff artifacts as downstream working artifacts rather than competing canonical briefs.",
		},
		Attempts:        1,
		MaxRetries:      1,
		ContextSections: contextSections,
	}, logger); err != nil {
		return fmt.Errorf("write planning active task: %w", err)
	}

	logger.Info("launching interactive Pi for planning")
	taskCtx := agent.TaskContext{
		ID:         planTaskID,
		Type:       "plan",
		Attempt:    1,
		MaxRetries: 1,
	}
	launcher := planRunPiInteractive
	if launcher == nil {
		launcher = planNewPiInteractiveLauncher()
	}
	_, err = launcher.Run(ctx, agent.PiInteractiveLaunchRequest{
		ProjectRoot:   projectRoot,
		SessionDir:    agent.PiInteractiveSessionDir(projectRoot, agent.RunPhasePlanning, taskCtx),
		Phase:         agent.RunPhasePlanning,
		Task:          taskCtx,
		InitialPrompt: "Read .doug/ACTIVE_TASK.md and follow it for this Doug planning session.",
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
	if intent == "" {
		if !planIsInteractive() {
			return planRunContext{}, fmt.Errorf("planning intent required in non-interactive mode; provide positional text or --intent")
		}

		captured, err := promptPlanningIntent(planNewPrompter())
		if err != nil {
			return planRunContext{}, err
		}
		intent = captured
	}

	return planRunContext{
		Intent: intent,
		Mode:   mode,
		Epic:   strings.TrimSpace(planFlags.epic),
	}, nil
}

func promptPlanningIntent(p planningIntentPrompter) (string, error) {
	intent, err := p.Compose(
		"Planning intent required. Describe what this `doug plan` session should accomplish.\nEnter submits. Shift+Enter inserts a newline.",
		"",
	)
	if err != nil {
		return "", fmt.Errorf("capture planning intent: %w", err)
	}

	intent = strings.TrimSpace(intent)
	if intent == "" {
		return "", fmt.Errorf("planning intent is required; provide positional text, --intent, or enter it in the interactive prompt")
	}

	return intent, nil
}
