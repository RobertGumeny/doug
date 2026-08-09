package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
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
	Intent           string
	Mode             string
	Epic             string
	ModeAutoDetected bool
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
	if runCtx.Mode == "" && !planModeFlagSupplied(cmd) && isGreenfieldPlanningRepo(projectRoot) {
		runCtx.Mode = "greenfield"
		runCtx.ModeAutoDetected = true
	}

	return planProjectContext(cmd.Context(), projectRoot, cmd.OutOrStdout(), runCtx)
}

func planProjectContext(ctx context.Context, projectRoot string, outWriter io.Writer, runCtx planRunContext) error {
	paths := orchestrator.NewPaths(projectRoot)
	logger := log.New()
	if runCtx.ModeAutoDetected {
		logger.Info("auto-detected greenfield planning mode for near-empty repository")
	}

	reportedBugs, err := plan.LoadReportedBugContext(projectRoot, log.Warning)
	if err != nil {
		return fmt.Errorf("load reported bug planning context: %w", err)
	}
	researchReports, err := plan.LoadResearchReports(projectRoot, log.Warning)
	if err != nil {
		return fmt.Errorf("load research report planning context: %w", err)
	}

	_, created, err := plan.EnsurePlanDocument(paths.DougDir, plan.WorkbookContext{
		PlanningIntent: runCtx.Intent,
		PlanningMode:   runCtx.Mode,
		TargetEpicHint: runCtx.Epic,
		IntakeSections: planIntakeSections(reportedBugs, researchReports),
	})
	if err != nil {
		return err
	}

	if _, err := agent.PrepareExecution(string(agent.RunPhasePlanning), string(types.TaskTypePlan), planTaskID); err != nil {
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

	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:      planTaskID,
		TaskType:    types.TaskTypePlan,
		ProjectRoot: projectRoot,
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
		Type:       string(types.TaskTypePlan),
		Attempt:    1,
		MaxRetries: 1,
	}
	launcher := planRunPiInteractive
	if launcher == nil {
		launcher = planNewPiInteractiveLauncher()
	}
	agentResp, err := launcher.Run(ctx, agent.PiInteractiveLaunchRequest{
		ProjectRoot:   projectRoot,
		SessionDir:    agent.PiInteractiveSessionDir(projectRoot, agent.RunPhasePlanning, taskCtx),
		Phase:         agent.RunPhasePlanning,
		Task:          taskCtx,
		InitialPrompt: "Read .doug/ACTIVE_TASK.md and follow it for this Doug planning session.",
	})
	persistRunStats(logger, paths.LogsDir, runCtx.Epic, agent.RunPhasePlanning, planTaskID, 1, agentResp)
	if err != nil {
		return err
	}

	return nil
}

func planIntakeSections(reportedBugs []plan.ReportedBugContext, researchReports []plan.ResearchReportContext) []plan.IntakeSection {
	sections := reportedBugIntakeSections(reportedBugs)
	sections = append(sections, researchReportIntakeSections(researchReports)...)
	return sections
}

func reportedBugIntakeSections(reportedBugs []plan.ReportedBugContext) []plan.IntakeSection {
	if len(reportedBugs) == 0 {
		return nil
	}

	bullets := make([]string, 0, len(reportedBugs))
	for _, bug := range reportedBugs {
		bullets = append(bullets, bug.PlanningBullet())
	}
	return []plan.IntakeSection{
		{
			Header:  "**Reported bugs** (from `.doug/intake/bugs/`) — treat blocking reports as work that interrupted an earlier task because acceptance criteria could not be verified or the next change would have been unsafe; treat non-blocking reports as deferred findings to plan intentionally:",
			Bullets: bullets,
		},
	}
}

func researchReportIntakeSections(researchReports []plan.ResearchReportContext) []plan.IntakeSection {
	if len(researchReports) == 0 {
		return nil
	}

	bullets := make([]string, 0, len(researchReports))
	for _, report := range researchReports {
		bullets = append(bullets, report.PlanningBullet())
	}
	return []plan.IntakeSection{
		{
			Header:  "**Recent research** (from `.doug/intake/research/`) — treat these as planning candidates:",
			Bullets: bullets,
		},
	}
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

func planModeFlagSupplied(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	flag := cmd.Flags().Lookup("mode")
	return flag != nil && flag.Changed
}

func isGreenfieldPlanningRepo(projectRoot string) bool {
	return config.DetectBuildSystem(projectRoot) == "" && hasShallowGitHistory(projectRoot) && hasFewNonDougFiles(projectRoot)
}

func hasShallowGitHistory(projectRoot string) bool {
	cmd := exec.Command("git", "-C", projectRoot, "rev-list", "--count", "--max-count=2", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return true
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return true
	}
	return count <= 1
}

func hasFewNonDougFiles(projectRoot string) bool {
	const maxGreenfieldFiles = 3
	count := 0
	err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == projectRoot {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".doug", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		count++
		if count > maxGreenfieldFiles {
			return filepath.SkipAll
		}
		return nil
	})
	return err == nil && count <= maxGreenfieldFiles
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
