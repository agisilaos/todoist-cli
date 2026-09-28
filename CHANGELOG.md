# Changelog

All notable changes to this project will be documented in this file.

The format is based on *Keep a Changelog*, and this project adheres to *Semantic Versioning*.

## [v0.9.0] - 2026-09-28

### Upgrade notes

- **Manual login now requires connectivity:** `auth login`, including `--print-env`, verifies the supplied token with Todoist before saving or exporting it. Rejected tokens and connection failures leave existing credentials unchanged. [#6](https://github.com/agisilaos/todoist-cli/pull/6)
- Apply saved review plans only with this version or newer, using the same configuration, account, and API endpoint. Do not edit recovery plans or apply them with older binaries; uncertain writes require inspection and a fresh review. Run only one applying process at a time. [#11](https://github.com/agisilaos/todoist-cli/pull/11)
- Unknown help targets now return usage exit 2 instead of successfully falling back to broader help. Valid help remains available without credentials and with broken configuration. [#8](https://github.com/agisilaos/todoist-cli/pull/8)

### Added

- Added `todoist review` to work through overdue and today's tasks, choose what to keep, change, complete, or skip, and inspect a plan before confirming changes. Read-only credentials and dry runs stop at preview; interactive review requires terminal input. [#11](https://github.com/agisilaos/todoist-cli/pull/11)
- Daily review saves recovery plans before applying changes, checks for stale tasks, and reports applied, failed, partial, and unattempted work. Retrying a definite failure skips recorded successes; application is not transactional and does not guarantee exactly-once writes. [#11](https://github.com/agisilaos/todoist-cli/pull/11)

### Changed

- Task capture now prints a human-readable receipt with returned content, destination, due date, recurrence, priority, labels, and correction guidance. Dry-run feedback distinguishes submitted values from Todoist's interpretation; JSON, NDJSON, plain, and quiet capture output retain their existing contracts. [#9](https://github.com/agisilaos/todoist-cli/pull/9)
- Subcommand help now focuses on the selected operation, and human command errors suggest nearby names without executing them. Root help and the quickstart lead with everyday workflows and explain Inbox scope versus pagination. [#8](https://github.com/agisilaos/todoist-cli/pull/8), [#10](https://github.com/agisilaos/todoist-cli/pull/10)
- Implicit Inbox lists now show an `Inbox` label in human output, including empty results; quiet and machine output remain unchanged. [#10](https://github.com/agisilaos/todoist-cli/pull/10)

### Fixed

- Bare `inbox` now reports an authentication error instead of panicking when credentials are missing. Inbox lookup failures stop before fetching tasks rather than silently returning tasks from other projects. [#10](https://github.com/agisilaos/todoist-cli/pull/10)

## [v0.8.0] - 2026-09-26

### Upgrade notes

- **Breaking for new saved logins:** macOS builds now use Keychain by default. On Linux, Windows, or macOS builds without native storage, use `auth login --credential-store=file`. Existing profiles keep their current storage; native failures never silently fall back to plaintext. [#5](https://github.com/agisilaos/todoist-cli/pull/5)
- Move an existing profile to Keychain with `auth migrate --credential-store=native`; use `auth repair` after an interrupted migration. Before downgrading, migrate back with `auth migrate --credential-store=file` because older binaries cannot read native profiles. Migration cannot erase backups or snapshots. [#5](https://github.com/agisilaos/todoist-cli/pull/5)

### Added

- Added `auth login --oauth --read-only` for read-only profiles whose mutations the CLI blocks, including agent actions. Manual and environment tokens retain their existing permission to attempt writes. Token refresh remains unsupported; live device authorization support is unverified. [#4](https://github.com/agisilaos/todoist-cli/pull/4)
- Added `--ids-only` to supported lists and shortcuts, emitting one ID per line for scripts. Empty results produce no stdout, and pagination hints go to stderr. [#2](https://github.com/agisilaos/todoist-cli/pull/2)
- Added PowerShell 7 completion on macOS and Linux, including installation and removal through `completion powershell` and its `pwsh` alias. [#3](https://github.com/agisilaos/todoist-cli/pull/3)
- Added agent progress events and apply summaries showing successful, failed, replay-skipped, and destructive actions. [593d415](https://github.com/agisilaos/todoist-cli/commit/593d415d03ec44279d9b7de9eb113cb77df9760a), [56e97e7](https://github.com/agisilaos/todoist-cli/commit/56e97e7730a8c67c780e73783fbf84b280b271d6)

### Changed

- JSON/NDJSON lists now report remaining pages on stderr, and empty JSON resource lists return `[]`. Commands that previously emitted prose under `--ndjson` now return structured records. [#5](https://github.com/agisilaos/todoist-cli/pull/5)

### Security

- Token entry no longer echoes secrets in the terminal, and cancellation restores terminal settings. [#5](https://github.com/agisilaos/todoist-cli/pull/5)
- Fixed migration and logout leaving stale credential copies in files containing alternate spellings of credential fields. [#5](https://github.com/agisilaos/todoist-cli/pull/5)

### Fixed

- Agent actions now record replay protection before reporting success and stop if recording fails. Interrupted remote writes can still be repeated on retry; use one applying process at a time. [#1](https://github.com/agisilaos/todoist-cli/pull/1)
- Generated schedules now preserve literal arguments, selected profiles, configuration, and dry-run settings. [#4](https://github.com/agisilaos/todoist-cli/pull/4)
- Agent commands now stop before dispatching actions if the requested progress log cannot be opened. [#5](https://github.com/agisilaos/todoist-cli/pull/5)
- OAuth approval no longer expires at the individual HTTP request timeout. [#5](https://github.com/agisilaos/todoist-cli/pull/5)
- Fixed `today` with stored credentials; unsupported options and positional arguments now return usage exit 2. [#5](https://github.com/agisilaos/todoist-cli/pull/5)
- Restored filter operations through Todoist's supported Sync API. [#5](https://github.com/agisilaos/todoist-cli/pull/5)
- Help remains available with broken configuration or missing mutation arguments, and diagnostics report configuration failures. [#5](https://github.com/agisilaos/todoist-cli/pull/5)
- Preserved option values such as `--content '--json'` literally instead of interpreting them as global flags. [#5](https://github.com/agisilaos/todoist-cli/pull/5)
- Fixed zsh activation instructions, including installations with custom filenames, spaces, or quotes. [e981b30](https://github.com/agisilaos/todoist-cli/commit/e981b300bd98c2fc2893dfedde4020099dbc1b92)
- Fixed invalid Homebrew formula generation and added the MIT license to both macOS archives. [4f45a2a](https://github.com/agisilaos/todoist-cli/commit/4f45a2aecdc54731f8699a97c14941265113ffd7)

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
