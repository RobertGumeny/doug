---
title: OSS Beta Repository Readiness
updated: 2026-03-12
category: Features
tags: [oss, github, templates, policy, ci, coverage]
related_articles:
  - docs/kb/infrastructure/go.md
  - docs/kb/packages/init.md
---

# OSS Beta Repository Readiness

## Overview

EPIC-10 prepared `doug` for public OSS beta visibility by adding the repository metadata and contributor-facing surfaces that GitHub users encounter first. The work combined legal/community baseline documents, standardized issue and PR intake templates, and top-level badges that expose CI, coverage, license, and Go version status.

## Implementation

The repository root now includes the standard MIT `LICENSE`, a project-specific `CONTRIBUTING.md`, Contributor Covenant v2.1 `CODE_OF_CONDUCT.md`, and `SECURITY.md` with private disclosure instructions through GitHub Security Advisories.

GitHub community workflows are standardized through:

- `.github/ISSUE_TEMPLATE/bug_report.md` for reproducible defects
- `.github/ISSUE_TEMPLATE/feature_request.md` for improvement proposals
- `.github/pull_request_template.md` for change summary, testing, and doc/changelog follow-up

The top of `README.md` exposes the repository health signals expected for OSS distribution:

- GitHub Actions CI status
- Codecov coverage badge
- MIT license badge
- Go 1.26 badge

Contributor guidance intentionally references both `CLAUDE.md` and `AGENTS.md`. `CLAUDE.md` holds implementation conventions, while `AGENTS.md` defines task execution boundaries for agent-driven contributions.

## Key Decisions

- **Use standard upstream policy text where possible**: The MIT license and Contributor Covenant v2.1 were adopted without custom rewrites so the repository matches common OSS expectations and avoids ambiguous legal wording.
- **Keep security reporting private by default**: `SECURITY.md` directs reporters to GitHub's private advisory submission flow instead of public issues, reducing accidental disclosure risk.
- **Separate contributor policy from owner-only controls**: Branch protection remains a manual GitHub setting and is not documented in contributor-facing files, which keeps public docs limited to actions contributors can actually take.
- **Make templates lightweight but structured**: The issue and PR templates ask for the minimum information needed to triage bugs, evaluate features, and review patches without imposing project-specific ceremony.

## Usage Example

```text
Open GitHub issue chooser
-> Bug report / Feature request template
Open pull request
-> pull_request_template.md checklist appears automatically
```

## Edge Cases & Gotchas

- `make lint` depends on `golangci-lint` being installed locally. Contributors following `CONTRIBUTING.md` may need to install it separately before the documented workflow passes outside CI.
- The coverage badge depends on Codecov uploads from the Ubuntu CI job. If push-triggered uploads fail, the badge can lag even when tests are green.
- `CLAUDE.md` is referenced by `CONTRIBUTING.md` but is not scaffolded by `doug init`; generated projects are expected to author their own maintainer-facing implementation guide.

## Related Topics

See [Go Infrastructure & Best Practices](../infrastructure/go.md) for the CI, lint, coverage, and Makefile behavior behind the badges and contributor checks.

See [cmd/init — Project Scaffolding Subcommand](../packages/init.md) for how new doug-managed projects scaffold `AGENTS.md`, shared skills, and `.doug/PRD.md`.
