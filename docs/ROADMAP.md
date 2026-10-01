# Todoist CLI Roadmap

The following capabilities are implemented in the current source tree. Release
availability is recorded in [CHANGELOG.md](../CHANGELOG.md).

## Implemented

- Guided daily review with task edits, moves, confirmed plans, and replay recovery
- Everyday entry points in help and quickstart, with human Inbox scope labels
- Quick add with natural language parsing
- Human capture receipts with returned task details and explicit dry-run parsing limits
- NDJSON task/project/section/label/comment lists and output schemas
- Focused leaf-command help and human command-typo recovery
- IDs-only output for supported lists
- Task updates by ID or text reference
- Credential profile listing, selection, inspection, and recoverable removal
- OAuth PKCE login with explicit public-client setup and token-lifetime checks
- Configurable device-flow client; live Todoist device support is unverified
- Workspace listing and project collaborators
- Agent progress events, action policies, and replay protection
- Reference disambiguation and bulk task operations with previews
- Filters, reminders, notifications, activity, productivity stats, and settings

## Future work

Planned work is tracked in [GitHub Issues](https://github.com/agisilaos/todoist-cli/issues).
This file summarizes implemented capabilities; issues track changing priorities
and acceptance criteria.
