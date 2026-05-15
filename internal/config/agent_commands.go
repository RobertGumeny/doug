// Package config defines the Pi-centric command defaults for Doug's execution model.
// Doug routes agent interactions through Pi (execution_mode: rpc). The command fields
// contain prompt-only payloads — no CLI binary prefix — because PiAdapter handles the
// `pi --mode rpc` invocation and sends the resolved command string as the RPC payload.
package config

// AgentCommandSet defines the prompt payload template for each Doug workflow phase.
// These are Pi RPC message payloads, not CLI invocations.
type AgentCommandSet struct {
	Run      string
	Plan     string
	Scaffold string
	Research string
}

const (
	RuntimePrompt  = "This is a doug-orchestrated run: use .doug/ACTIVE_TASK.md as the task brief and complete the task described there. When filling `## Agent Result.outcome`, use only `SUCCESS`, `FAILURE`, `BUG`, or `EPIC_COMPLETE`."
	PlanPrompt     = "This is a doug-orchestrated planning run: use .doug/ACTIVE_TASK.md as the canonical brief for this run. Read it first, then update .doug/plan/PLAN.md as the planning workbook described there. The planning intent for this session is Doug-owned context already resolved into PLAN.md before launch, so use that current intent instead of inferring the objective from stale workbook prose. Treat PLAN.md and any handoff outputs as working artifacts, not competing canonical briefs. If the repository is empty or near-empty and the user has explicit day-0 or bootstrap intent, prefer scaffold-oriented handoff data under `manifest` instead of defaulting to an implementation epic."
	ResearchPrompt = "This is a doug-orchestrated research run: use .doug/ACTIVE_TASK.md as the canonical brief for this run. Perform read-only codebase analysis as directed by the brief and write the research report to .doug/logs/research/ as instructed."
)

// DefaultCommandSet returns the Pi RPC prompt payloads used as the default command
// templates. These are sent as RPC messages by PiAdapter — not CLI invocations.
func DefaultCommandSet() AgentCommandSet {
	return AgentCommandSet{
		Run:      `[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + RuntimePrompt,
		Plan:     `[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + PlanPrompt,
		Scaffold: `[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + RuntimePrompt,
		Research: `[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. ` + ResearchPrompt,
	}
}
