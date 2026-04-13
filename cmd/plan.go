package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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

var planBriefPattern = regexp.MustCompile("(?s)^" + regexp.QuoteMeta(planBriefStartTag) + "\\n.*?\\n" + regexp.QuoteMeta(planBriefEndTag) + "\\n*")

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

func planProjectContext(ctx context.Context, projectRoot string, outWriter io.Writer) error {
	paths := orchestrator.NewPaths(projectRoot)
	logger := log.New()

	cfg, err := planLoadConfig(paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	_, created, err := ensurePlanDocument(paths.DougDir)
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

func ensurePlanDocument(dougDir string) (string, bool, error) {
	planDir := filepath.Join(dougDir, "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create plan directory %s: %w", planDir, err)
	}

	planPath := filepath.Join(planDir, "PLAN.md")
	data, err := os.ReadFile(planPath)
	if err == nil {
		refreshed := refreshPlanDocument(string(data))
		if err := state.AtomicWrite(planPath, []byte(refreshed)); err != nil {
			return "", false, fmt.Errorf("write %s: %w", planPath, err)
		}
		return planPath, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, fmt.Errorf("stat %s: %w", planPath, err)
	}

	if err := state.AtomicWrite(planPath, []byte(initialPlanDocument())); err != nil {
		return "", false, fmt.Errorf("write %s: %w", planPath, err)
	}

	return planPath, true, nil
}

func initialPlanDocument() string {
	return refreshPlanDocument("" +
		"# Project Plan\n\n" +
		"## Planning Objective\n\n" +
		"Describe the planning request, intended outcome, and why it matters now.\n\n" +
		"## Current Context\n\n" +
		"Capture the relevant codebase, product, architectural, or backlog context here.\n\n" +
		"## Scope And Non-Goals\n\n" +
		"- In scope:\n" +
		"- Out of scope:\n\n" +
		"## Risks, Assumptions, And Open Questions\n\n" +
		"- Risks:\n" +
		"- Assumptions:\n" +
		"- Open questions:\n\n" +
		"## Proposed Epics\n\n" +
		"Document the intended epics, sequence, and rationale here.\n\n" +
		"## Handoff Readiness\n\n" +
		"State whether the plan is exploratory, ready for review, or ready for deterministic handoff.\n\n" +
		"## Handoff Data\n\n" +
		"```yaml\n" +
		"schema_version: 1\n" +
		"project:\n" +
		"  name: \"\"\n" +
		"  mode: \"\"\n" +
		"epics: []\n" +
		"```\n")
}

func refreshPlanDocument(existing string) string {
	body := strings.ReplaceAll(existing, "\r\n", "\n")
	body = planBriefPattern.ReplaceAllString(body, "")
	body = strings.TrimLeft(body, "\n")
	if body == "" {
		body = "# Project Plan\n"
	}
	return planBriefBlock() + "\n\n" + strings.TrimRight(body, "\n") + "\n"
}

func planBriefBlock() string {
	return strings.Join([]string{
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
		"- `doug handoff` owns generated derivatives such as `.doug/plan/epics/<EPIC-ID>/` and `.doug/plan/manifest.yaml`.",
		"",
		"When the plan is handoff-ready, ensure `## Handoff Data` contains a complete fenced YAML payload that `doug handoff` can parse without guesswork.",
		"",
		"If the workbook body is blank or contains only placeholder text, begin the planning conversation immediately by asking the user about their planning objective.",
		planBriefEndTag,
	}, "\n")
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
