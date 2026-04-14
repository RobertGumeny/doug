package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/state"
)

const (
	planTaskID        = "PLAN"
	planBriefStartTag = "<!-- DOUG-PLAN-BRIEF:START -->"
	planBriefEndTag   = "<!-- DOUG-PLAN-BRIEF:END -->"
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

var planBriefPattern = regexp.MustCompile("(?s)^" + regexp.QuoteMeta(planBriefStartTag) + "\\n.*?\\n" + regexp.QuoteMeta(planBriefEndTag) + "\\n*")

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

	_, created, err := ensurePlanDocument(paths.DougDir, runCtx)
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

func ensurePlanDocument(dougDir string, runCtx planRunContext) (string, bool, error) {
	planDir := filepath.Join(dougDir, "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create plan directory %s: %w", planDir, err)
	}

	planPath := filepath.Join(planDir, "PLAN.md")
	data, err := os.ReadFile(planPath)
	if err == nil {
		refreshed := refreshPlanDocument(string(data), runCtx)
		if err := state.AtomicWrite(planPath, []byte(refreshed)); err != nil {
			return "", false, fmt.Errorf("write %s: %w", planPath, err)
		}
		return planPath, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, fmt.Errorf("stat %s: %w", planPath, err)
	}

	if err := state.AtomicWrite(planPath, []byte(initialPlanDocument(runCtx))); err != nil {
		return "", false, fmt.Errorf("write %s: %w", planPath, err)
	}

	return planPath, true, nil
}

func initialPlanDocument(runCtx planRunContext) string {
	return refreshPlanDocument(""+
		"# Project Plan\n\n"+
		"## Planning Objective\n\n"+
		"Describe the planning request, intended outcome, and why it matters now.\n\n"+
		"## Current Context\n\n"+
		"Capture the relevant codebase, product, architectural, or backlog context here.\n\n"+
		"## Scope And Non-Goals\n\n"+
		"- In scope:\n"+
		"- Out of scope:\n\n"+
		"## Risks, Assumptions, And Open Questions\n\n"+
		"- Risks:\n"+
		"- Assumptions:\n"+
		"- Open questions:\n\n"+
		"## Proposed Epics\n\n"+
		"Document the intended epics, sequence, and rationale here.\n\n"+
		"## Handoff Readiness\n\n"+
		"State whether the plan is exploratory, ready for review, or ready for deterministic handoff.\n\n"+
		"## Handoff Data\n\n"+
		"```yaml\n"+
		"# Fill in this schema exactly. Do not add extra fields.\n"+
		"# Unknown fields cause `doug handoff` to fail.\n"+
		"schema_version: 1\n"+
		"project:\n"+
		"  name: \"My Project\"\n"+
		"  mode: \"brownfield\"\n"+
		"# Include `manifest` only when the plan needs greenfield scaffold output.\n"+
		"# When included, use this exact schema.\n"+
		"# manifest:\n"+
		"#   schema_version: 1\n"+
		"#   project:\n"+
		"#     name: \"My Project\"\n"+
		"#     mode: \"greenfield\"\n"+
		"#   scaffold:\n"+
		"#     language: \"typescript\"\n"+
		"#     runtime: \"node\"\n"+
		"#     framework: \"nextjs\"\n"+
		"#     package_manager: \"pnpm\"\n"+
		"#     build_system: \"npm-scripts\"\n"+
		"#   dependencies:\n"+
		"#     runtime:\n"+
		"#       - \"next\"\n"+
		"#     development:\n"+
		"#       - \"typescript\"\n"+
		"#   constraints:\n"+
		"#     - \"Describe a scaffold constraint here.\"\n"+
		"epics:\n"+
		"  - id: \"EPIC-1\"\n"+
		"    name: \"Example Epic\"\n"+
		"    prd: |\n"+
		"      # PRD\n"+
		"\n"+
		"      Describe the epic's product requirements here.\n"+
		"    tasks:\n"+
		"      - id: \"EPIC-1-001\"\n"+
		"        type: \"feature\"\n"+
		"        status: \"TODO\"\n"+
		"        description: \"Describe the task here.\"\n"+
		"        acceptance_criteria:\n"+
		"          - \"First acceptance criterion.\"\n"+
		"          - \"Second acceptance criterion.\"\n"+
		"```\n", runCtx)
}

func refreshPlanDocument(existing string, runCtx planRunContext) string {
	body := strings.ReplaceAll(existing, "\r\n", "\n")
	body = planBriefPattern.ReplaceAllString(body, "")
	body = strings.TrimLeft(body, "\n")
	if body == "" {
		body = "# Project Plan\n"
	}
	return planBriefBlock(runCtx) + "\n\n" + strings.TrimRight(body, "\n") + "\n"
}

func planBriefBlock(runCtx planRunContext) string {
	lines := []string{
		planBriefStartTag,
		"# Doug Planning Brief",
		"",
		"This file is both the Doug-owned planning brief and the editable planning workbook.",
		"",
		"Use this workbook to help the user refine scope, risks, epic sequencing, and executable tasks using repository and knowledge-base context.",
		"",
		"Rules:",
		"- Treat `.doug/plan/PLAN.md` as the single planning source of truth.",
		"- Use the narrative sections for collaborative planning notes and rationale.",
		"- Keep the plan coherent as you refine it; do not create alternate planning files or stage documents.",
		"- Keep the deterministic payload under `## Handoff Data` aligned with the surrounding narrative plan.",
		"- Fill in the seeded YAML schema exactly; do not add fields beyond the ones shown in the template.",
		"- Put greenfield scaffold data under `manifest`, not under `project`.",
		"- `doug handoff` owns generated derivatives such as `.doug/plan/epics/<EPIC-ID>/` and `.doug/plan/manifest.yaml`.",
		"",
		"When the plan is handoff-ready, ensure `## Handoff Data` contains a complete fenced YAML payload that `doug handoff` can parse without guesswork. Unknown fields are rejected.",
		"",
		"Planning Run Context:",
		"- Planning intent: " + planBriefValue(runCtx.Intent, "not provided on this CLI run"),
		"- Planning mode: " + planBriefValue(runCtx.Mode, "auto / not specified"),
		"- Target epic hint: " + planBriefValue(runCtx.Epic, "not specified"),
		"- If the existing workbook narrative disagrees with this run context, reconcile the workbook to match before continuing so CLI intent and PLAN.md do not diverge.",
		"",
		"If the workbook body is blank or contains only placeholder text, begin the planning conversation immediately by asking the user about their planning objective.",
		planBriefEndTag,
	}
	return strings.Join(lines, "\n")
}

func planBriefValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func resolvePlanAgentCommand(agentCommand, skillName, taskID string) string {
	resolved := strings.ReplaceAll(agentCommand, "{{skill_name}}", skillName)
	resolved = strings.ReplaceAll(resolved, "{{task_id}}", taskID)

	const runtimePrompt = ".doug/ACTIVE_TASK.md as the task brief and complete the task described there."
	const planPrompt = ".doug/plan/PLAN.md as the planning workbook. Read the Doug-owned briefing at the top of PLAN.md, then help the user refine the plan and complete the workbook there."

	if strings.Contains(resolved, runtimePrompt) {
		return strings.ReplaceAll(resolved, runtimePrompt, planPrompt)
	}

	if strings.Contains(resolved, ".doug/ACTIVE_TASK.md") {
		resolved = strings.ReplaceAll(resolved, ".doug/ACTIVE_TASK.md", ".doug/plan/PLAN.md")
	}
	if strings.Contains(resolved, "complete the task described there.") {
		resolved = strings.ReplaceAll(resolved, "complete the task described there.", "read the Doug-owned briefing at the top of PLAN.md, then help the user refine the plan and complete the workbook there.")
	}
	return resolved
}
