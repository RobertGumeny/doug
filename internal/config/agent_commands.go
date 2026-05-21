// Package config defines Doug's built-in execution prompts.
// Command content is derived from code constants — not from operator-supplied
// config templates. This keeps the interaction model authoritative in Doug itself
// rather than in a provider-specific registry stored in doug.yaml.
package config

const (
	RuntimePrompt  = "This is a doug-orchestrated run: use .doug/ACTIVE_TASK.md as the task brief and complete the task described there. When filling `## Agent Result.outcome`, use only `SUCCESS`, `FAILURE`, `BUG`, or `EPIC_COMPLETE`."
	PlanPrompt     = "This is a doug-orchestrated planning run: use .doug/ACTIVE_TASK.md as the canonical brief for this run. Read it first, then update .doug/plan/PLAN.md as the planning workbook. The planning intent for this session is Doug-owned context already resolved into PLAN.md before launch; use that current intent instead of inferring intent from stale workbook prose. .doug/plan/PLAN.md is the source of truth for this planning cycle — do not treat derivative artifacts under .doug/plan/epics/ or .doug/plan/manifest.yaml as competing briefs. Before writing the final handoff data into .doug/plan/PLAN.md, produce an alignment summary covering resolved intent, scope decisions, epic sequence, and remaining open questions, and get explicit user confirmation. Do not write machine-consumable handoff YAML before that confirmation."
	ResearchPrompt = "This is a doug-orchestrated research run: use .doug/ACTIVE_TASK.md as the canonical brief for this run. Perform read-only codebase analysis as directed by the brief and write the research report to .doug/logs/research/ as instructed."
)

// BuildCommand constructs the agent invocation string for the given phase,
// substituting taskID and skillName into the canonical prompt. The command is
// derived from built-in constants — not from config — so the interaction model
// remains authoritative in Doug rather than in operator-supplied templates.
func BuildCommand(phase, taskID, skillName string) string {
	var prompt string
	switch phase {
	case "planning":
		prompt = PlanPrompt
	case "research":
		prompt = ResearchPrompt
	default: // runtime, scaffold, post_epic_kb
		prompt = RuntimePrompt
	}
	return "[DOUG_TASK_ID: " + taskID + "] Please activate " + skillName + ". " + prompt
}
