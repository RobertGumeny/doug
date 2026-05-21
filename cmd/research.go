package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/types"
)

const (
	researchTaskID = "RESEARCH"
)

var (
	researchRunAgent   agent.Backend      // nil in production; tests inject a stub
	researchNewBackend = agent.NewBackend // func() agent.Backend
)

var researchFlags struct {
	topic string
	scope string
}

var researchCmd = &cobra.Command{
	Use:   "research [topic...]",
	Short: "Run a read-only research pass and save the report under .doug/logs/research/",
	Long:  "Run Doug's read-only research workflow for a topic, file, feature, or the whole codebase, then save the report under .doug/logs/research/.",
	Args:  cobra.ArbitraryArgs,
	RunE:  runResearch,
}

type researchRunContext struct {
	Topic string
	Scope string
}

func init() {
	researchCmd.Flags().StringVar(&researchFlags.topic, "topic", "", "explicit research topic for this run")
	researchCmd.Flags().StringVar(&researchFlags.scope, "scope", "", "scope type hint (feature|file|codebase)")
}

func runResearch(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	runCtx, err := resolveResearchRunContext(cmd, args)
	if err != nil {
		return err
	}

	return researchProjectContext(cmd.Context(), projectRoot, cmd.OutOrStdout(), runCtx)
}

func researchProjectContext(ctx context.Context, projectRoot string, outWriter io.Writer, runCtx researchRunContext) error {
	paths := orchestrator.NewPaths(projectRoot)
	logger := log.New()

	prep, err := agent.PrepareExecution(string(agent.RunPhaseResearch), string(types.TaskTypeResearch), researchTaskID)
	if err != nil {
		return fmt.Errorf("prepare research execution: %w", err)
	}

	contextSections := []agent.ActiveTaskSection{
		{
			Heading: "Research Output",
			Body: "" +
				"- Canonical brief for this run: `.doug/ACTIVE_TASK.md`\n" +
				"- Write the research report directly to `.doug/logs/research/report_[scope]-[timestamp].md`\n" +
				"- Do not create `RESEARCH_REPORT.md` in the project root.\n" +
				"- Use read-only tools (Glob, Grep, Read) to explore the codebase; do not modify product code, docs, or task files.\n",
		},
	}

	description := buildResearchDescription(runCtx)
	acceptanceCriteria := buildResearchAcceptanceCriteria(runCtx)

	if err := agent.WriteActiveTask(agent.ActiveTaskConfig{
		TaskID:             researchTaskID,
		TaskType:           types.TaskTypeResearch,
		DougDir:            paths.DougDir,
		Description:        description,
		AcceptanceCriteria: acceptanceCriteria,
		Attempts:           1,
		MaxRetries:         1,
		ContextSections:    contextSections,
	}, logger); err != nil {
		return fmt.Errorf("write research active task: %w", err)
	}

	writef(outWriter, "Research output: %s\n", filepath.ToSlash(filepath.Join(".doug", "logs", "research")))

	logger.Info("invoking agent for research")
	contract := agent.ResearchContract(projectRoot, paths.DougDir)
	researchBackend := researchRunAgent
	if researchBackend == nil {
		researchBackend = researchNewBackend()
	}
	_, err = researchBackend.Run(ctx, agent.RunRequest{
		Phase: agent.RunPhaseResearch,
		Task: agent.TaskContext{
			ID:         researchTaskID,
			Type:       string(types.TaskTypeResearch),
			Attempt:    1,
			MaxRetries: 1,
		},
		Brief:            contract.Brief,
		ContextLoadOrder: contract.ContextLoadOrder,
		Artifacts:        contract.Artifacts,
		Routing: agent.RoutingInputs{
			Workflow:        "research",
			SkillName:       prep.SkillName,
			InteractionMode: prep.InteractionMode,
		},
		Restrictions:  contract.Restrictions,
		InitialPrompt: prep.InitialPrompt,
		ProjectRoot:   projectRoot,
		Output:        nil,
	})
	if err != nil {
		return err
	}

	writef(outWriter, "Research complete. Report written to %s\n", filepath.ToSlash(filepath.Join(".doug", "logs", "research")))
	return nil
}

func buildResearchDescription(runCtx researchRunContext) string {
	if runCtx.Topic != "" {
		return fmt.Sprintf("Perform read-only codebase analysis on: %s. Write the research report to `.doug/logs/research/`.", runCtx.Topic)
	}
	return "Perform read-only codebase analysis and write the research report to `.doug/logs/research/`."
}

func buildResearchAcceptanceCriteria(runCtx researchRunContext) []string {
	criteria := []string{
		"Write the research report to `.doug/logs/research/report_[scope]-[timestamp].md`.",
		"Do not create `RESEARCH_REPORT.md` in the project root or modify any product code.",
	}
	if runCtx.Scope != "" {
		criteria = append(criteria, fmt.Sprintf("Scope the analysis to: %s.", runCtx.Scope))
	}
	return criteria
}

func resolveResearchRunContext(cmd *cobra.Command, args []string) (researchRunContext, error) {
	topicFromArgs := strings.TrimSpace(strings.Join(args, " "))
	topicFromFlag := strings.TrimSpace(researchFlags.topic)
	if topicFromArgs != "" && topicFromFlag != "" && topicFromArgs != topicFromFlag {
		return researchRunContext{}, fmt.Errorf("research topic provided twice with different values; use either positional args or --topic")
	}

	scope := strings.ToLower(strings.TrimSpace(researchFlags.scope))
	if scope != "" {
		validScopes := []string{"feature", "file", "codebase"}
		valid := false
		for _, s := range validScopes {
			if s == scope {
				valid = true
				break
			}
		}
		if !valid {
			return researchRunContext{}, fmt.Errorf("invalid scope %q; want one of: %s", scope, strings.Join(validScopes, ", "))
		}
	}

	topic := topicFromFlag
	if topic == "" {
		topic = topicFromArgs
	}

	return researchRunContext{
		Topic: topic,
		Scope: scope,
	}, nil
}
