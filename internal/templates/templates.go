// Package templates holds the embedded template files used by the orchestrator.
// All templates are compiled into the binary at build time via //go:embed.
//
// The init/ subdirectory holds files stamped into a new project by `doug init`.
// Copied as-is with no filename transformations.
package templates

import "embed"

// Init holds files copied to the target project by `doug init`.
//
//go:embed init/.gitignore init/AGENTS.md init/CLAUDE.md init/DOUG_README.md
//go:embed init/BUG_REPORT_TEMPLATE.md
//go:embed init/skills
//go:embed all:init/.pi
var Init embed.FS
