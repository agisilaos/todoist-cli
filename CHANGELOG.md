# Changelog

All notable changes to this project will be documented in this file.

The format is based on *Keep a Changelog*, and this project adheres to *Semantic Versioning*.

## [v0.7.0] - 2026-02-23

### Added

- Added project command expansion with `project create`, `project move`, and `project browse`.
- Added settings command family for viewing/updating settings and managing themes.
- Added stats command expansion, including productivity summary, goals, and vacation subcommands.
- Added activity, reminders, and notifications command families with list/action coverage.
- Added `upcoming` and top-level `completed` shortcuts for planning/review workflows.
- Added Todoist URL-to-command routing and URL reference support for task/project/label/filter references.
- Added natural task-reference improvements, including due-aware lookup and shorthand add/update input mode.
- Added accessible output mode for human task views.

### Changed

- Improved project URL handling and invitation/notification UX parity.
- Improved top-level help guidance for AI/LLM-agent usage and completion install/uninstall ergonomics.
- Continued CLI/app-layer refactor to centralize validation, resolution, planning, and command wiring.
- Expanded test/CI coverage for retry policies, resolver edge cases, formatter output, and release checks.
- Standardized release-check/help-snapshot/docs workflow contracts and repository docs structure.

### Fixed

- Hardened settings/activity decoding against live API payload variations.
- Improved dry-run inbox behavior to work without requiring inbox lookup.
- Smoothed human task filtering, completed-date UX, and task text/ID completion resolution.
- Removed non-portable `rg` dependency from release checks.

## [v0.6.0] - 2026-02-14

- feat(cli): add doctor command and cached, ranked reference resolution (da4663c)
- refactor(cli): unify paginated fetch loops with shared helper (d5d5f51)
- test(cli): harden filter help and assignee/filter contract coverage (5a77f33)

## [v0.5.0] - 2026-02-13

- feat(cli): add filter commands and assignee reference resolution (b773e9a)
- feat(cli): add device auth, workspace/collab, agent policy+replay, progress, and bulk task ops (0da2b9f)
- feat(auth): add detailed auth login help and nested help routing (1fc4f2f)
- feat(auth): make --print-env honor json and ndjson modes (9ab4e67)
- fix(cli): harden oauth UX and align completion/help docs (6175ebd)
- fix(cli): route global --help to subcommand context (6dfc95f)
- feat(auth): add OAuth PKCE login flow with unit tests (71d13fd)
- feat: add json-first errors and resilient api retries (d972435)
- feat: add quiet-json mode and command aliases with contracts (508bdce)
- docs: refresh roadmap/spec and add cli behavior contracts (bf57574)
- refactor: unify subcommand flag parsing and ndjson writers (8f81765)

## [v0.4.0] - 2026-02-12

- fix: improve legacy task id errors and schema output noise (1e4ebfa)
- feat: harden cli parsing and expand ndjson support (0c5f2a9)

## [v0.3.0] - 2026-02-12

- feat: tighten quick-add validation and formalize output schemas (f88009a)

## [v0.2.0] - 2026-02-12

- feat: switch add to sync quick add and simplify json output (436389b)
- feat: allow task refs and update docs (5da29aa)
- docs: add spec and roadmap (17d7328)
- feat: add task view and ndjson output (bd3390d)
- feat: add quick-add parsing (863dccb)
- feat: add today/inbox shortcuts and id refs (80d8af0)
- docs: expand agent workflows (ccf8667)
- feat: scoped planner context and schema updates (28f59b9)
- feat: add agent run, schedule, examples (2bc19b0)
- chore: gofmt schema (40d5789)
- feat: quick add alias and docs (8e3cd93)
- docs: expand schema definitions (292792f)
- feat: harden agent workflows (f6808e6)
- feat: add schemas, agent dry-run, output snapshots (e5aa6fa)
- feat: inbox add, presets, fuzzy resolution (b6d1927)
- feat: improve completions and error output (9e426f1)
- chore(release): v0.1.1 (2344ff0)
- feat: add shell completions (31bc73c)

## [v0.1.1] - 2026-01-11

- feat: add shell completions (31bc73c)

## [v0.1.0] - 2026-01-11

### Added

- Initial public release of todoist-cli with task/project/section/label/comment management, auth, agent plan/apply workflow, JSON/plain output modes, and Homebrew tap support.
