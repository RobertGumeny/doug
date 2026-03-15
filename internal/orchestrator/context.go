// Package orchestrator contains the core orchestration logic for the doug
// binary: bootstrapping state from tasks, managing task pointers, and
// validating state consistency.
package orchestrator

import "github.com/robertgumeny/doug/internal/types"

// LoopContext is an alias for types.LoopContext. It is defined here so that
// existing callers (including handler tests) that reference orchestrator.LoopContext
// continue to work without change while the type lives in internal/types.
type LoopContext = types.LoopContext

// AgentResult captures the output of a single agent invocation. It is
// constructed in Orchestrator.Run after the agent process returns and passed
// explicitly to outcome handlers so that LoopContext has no post-construction
// field mutations.
type AgentResult struct {
	SessionResult   *types.SessionResult
	DurationSeconds int
}
