---
name: "prep-for-release"
description: "Prepare a feature or hotfix release of the doug CLI: confirm the release branch, pick the next version, stamp the CHANGELOG, then commit and tag. Pushing the resulting tag triggers the goreleaser GitHub Action that builds and publishes the release."
---

# Release Prep Workflow

doug is a single Go module shipped as one versioned CLI binary. There is no version field in source to edit — the version is derived entirely from the git tag.

| Thing | Mechanism |
|---|---|
| Version source | git tag only. goreleaser injects it at build time via ldflags (`-X github.com/robertgumeny/doug/cmd.version={{.Version}}`). The `version` var in `cmd/root.go` defaults to `dev` and falls back to build info. |
| Tag format | `vX.Y.Z` (plain, no prefix), created on `main`. |
| Publish | Pushing a `v*` tag fires `.github/workflows/release.yml` → goreleaser builds cross-platform archives + checksums and creates the GitHub Release. Pushing the tag **is** the publish. |

## Phase 1: Confirm the release branch

Releases are cut from `main`. Run `git branch --show-current` and `git status`.

- If you are on `main` and clean → proceed.
- If you are on a feature branch (doug's normal working state) → the feature work must be merged to `main` first. Tagging a feature branch publishes code that isn't on the release line. Surface this to the user and let them merge (or confirm they want to release from where they are) before continuing.

## Phase 2: Determine the next version

Look at the most recent tag: `git tag --list 'v*' | sort -V | tail -1`.

- **Hotfix** (bug fix, no new behavior): bump patch (Z).
- **Minor feature** (new commands/flags/behavior, backwards-compatible): bump minor (Y), reset patch.
- **Breaking change**: bump major (X), reset minor and patch.

Pre-1.0, lean conservative: additive feature work bumps the minor, not the major. Do **not** jump to `v1.0.0` casually — doug's v1.0.0 has explicit prerequisites tracked in the research backlog (e.g. `multi-module-orchestration`); 1.0.0 is a deliberate milestone, not "the next number."

State the resulting version explicitly before proceeding (e.g. last tag `v0.7.0` + a batch of new features → `v0.8.0`), and confirm it with the user if the change type is at all ambiguous.

## Phase 3: Stamp the CHANGELOG

`CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/). Changes accumulate under `## [Unreleased]` as work lands. Releasing means converting that section into a version section and opening a fresh empty one.

1. Rename `## [Unreleased]` → `## [X.Y.Z]` (bracketed version, **no date** — match the existing released sections).
2. Review its entries against the diff, tidy them, and drop any empty `### Added/Changed/Fixed/Removed` subsections.
3. Add a fresh, empty `## [Unreleased]` block above the new version section, with empty `### Added`, `### Changed`, `### Fixed`, `### Removed` subsections ready for the next cycle.

There must end up with exactly one section per version, and never both a stamped version and a non-empty leftover Unreleased for the same work.

Derive and sanity-check entries from the diff — `git log <last-tag>..HEAD --oneline` for the full range, `git show <commit>` for detail. Do not ask the user to describe the changes. Subsections:

- `### Added` — new commands, flags, or behavior
- `### Changed` — non-breaking behavioral changes
- `### Fixed` — bug fixes; name the symptom, the root cause, and the change
- `### Removed` — removed features or surfaces

## Phase 4: Commit and tag

```sh
git add CHANGELOG.md
git commit -m "chore: release vX.Y.Z"
git tag vX.Y.Z
```

The tag must be plain `vX.Y.Z` so it matches the existing series and is picked up by the release workflow's `v*` trigger.

Do **not** push — pushing is the publish, and that is the user's call (Phase 5).

## Phase 5: Publish (the user's call)

Pushing the tag triggers the GitHub Action that builds and publishes the release, so this step is outward-facing and irreversible-ish (a published release is public immediately). Leave the push to the user. Tell them to run:

```sh
git push && git push --tags
```

Both are needed: `git push` alone does not push tags, and `git push --tags` alone does not push the release commit. Pushing the `vX.Y.Z` tag is what kicks off `.github/workflows/release.yml` → goreleaser → the published GitHub Release.

## Phase 6: Confirm

Tell the user:
- The version and the tag created.
- That CHANGELOG `[Unreleased]` was stamped to `[X.Y.Z]` and a fresh `[Unreleased]` opened.
- That pushing the tag will trigger the release build, and they can watch it under the repo's **Actions** tab (Release workflow).
