// Package config defines Doug's built-in execution prompts.
// Command content is derived from code constants — not from operator-supplied
// config templates. This keeps the execution model authoritative in Doug itself
// rather than in a provider-specific registry stored in doug.yaml.
package config

const (
	RuntimePrompt  = "This is a doug-orchestrated run: use .doug/ACTIVE_TASK.md as the task brief and complete the task described there. When filling `## Agent Result.outcome`, use only `SUCCESS`, `FAILURE`, `BUG`, or `EPIC_COMPLETE`."
	PlanPrompt     = "This is a doug-orchestrated planning run: use .doug/ACTIVE_TASK.md as the canonical brief for this run. Read it first, then update .doug/plan/PLAN.md as the planning workbook described there. The planning intent for this session is Doug-owned context already resolved into PLAN.md before launch, so use that current intent instead of inferring the objective from stale workbook prose. Treat PLAN.md and any handoff outputs as working artifacts, not competing canonical briefs. When clarification is needed, check the codebase and KB first; ask the user only when the repository cannot answer the question. When material ambiguity remains after lookup, ask one high-leverage question at a time. Before finalizing handoff-ready epics and tasks, produce an explicit alignment summary — resolved intent, scope decisions, epic sequence, and remaining open questions — and get confirmation before writing the final handoff data. Promote execution-relevant constraints, risks, or architectural decisions discovered during planning into the epic PRD or task contracts rather than leaving them only in workbook narrative. If the repository is empty or near-empty and the user has explicit day-0 or bootstrap intent, prefer scaffold-oriented handoff data under `manifest` instead of defaulting to an implementation epic."
	ResearchPrompt = "This is a doug-orchestrated research run: use .doug/ACTIVE_TASK.md as the canonical brief for this run. Perform read-only codebase analysis as directed by the brief and write the research report to .doug/logs/research/ as instructed."
)

// BuildCommand constructs the agent invocation string for the given phase,
// substituting taskID and skillName into the canonical prompt. The command is
// derived from built-in constants — not from config — so the execution model
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
