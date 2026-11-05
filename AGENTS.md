## Navigation

- For implementation or review, start with [architecture](docs/ARCHITECTURE.md) for code responsibilities and the [docs index](docs/README.md) for behavior contracts and command-change dependencies.
- For validation or handoff, follow the [contributor gate](CONTRIBUTING.md#ready-for-handoff).
- For releases, follow the [release runbook](RELEASING.md).
- Before exploring or changing domain terminology, read the [domain guide](docs/agents/domain.md) for this repository's single-context layout, glossary, and ADR requirements.
- For issue work, use this repository's GitHub Issues via the [issue-tracker guide](docs/agents/issue-tracker.md) and the five canonical [triage labels](docs/agents/triage-labels.md) without overrides.

## Sessions

- For a new implementation chat, prepare a scoped brief and run `scripts/session start <name>` for its own worktree off fresh `origin/main`. Check for overlapping work before editing.
- Commit when asked; push when asked. For authorized direct-to-main work, `scripts/session land` rebases, reviews the committed change, runs the contributor gate and pushes without a PR. Follow the [session guide](docs/agents/sessions.md) for kickoff, landing failures and cleanup.
