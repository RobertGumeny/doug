// Package templates holds the embedded template files used by the orchestrator.
// All templates are compiled into the binary at build time via //go:embed.
package templates

import _ "embed"

// SessionResult is the embedded session result template.
// It is copied to the session file path before each agent invocation,
// with the task_id field pre-filled by CreateSessionFile.
//
//go:embed session_result.md
var SessionResult string
