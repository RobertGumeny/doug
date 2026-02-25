// Package templates holds the embedded template files used by the orchestrator.
// All templates are compiled into the binary at build time via //go:embed.
//
// Two subdirectories serve different purposes:
//
//   - runtime/ — templates used internally by the orchestrator at runtime (e.g. session file pre-creation).
//     These are never copied to the user's project.
//
//   - init/ — files stamped into a new project by `doug init`. Copied as-is with no filename transformations.
package templates

import "embed"

// Runtime holds templates used by the orchestrator at runtime.
//
//go:embed runtime
var Runtime embed.FS

// Init holds files copied to the target project by `doug init`.
//
//go:embed init
var Init embed.FS

// SessionResult is the content of runtime/session_result.md.
// Convenience accessor used by CreateSessionFile.
//
//go:embed runtime/session_result.md
var SessionResult string
