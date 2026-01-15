# Todoist CLI Specification

## Overview

Go-based CLI for Todoist. Binary name: `todoist`. Designed for humans and scripts, with stable `--json`, `--plain`, and `--ndjson` outputs.

## Authentication

- **Primary**: `TODOIST_TOKEN` environment variable
- **Fallback**: `~/.config/todoist/credentials.json`
- Profiles supported via `--profile` / `TODOIST_PROFILE`

## Command Structure

Pattern: `todoist <resource> <action> [args]`

### Top-level shortcuts

- `todoist add "text"` — quick add with parsing (`#Project`, `@label`, `p1..p4`, `due:<text>`)
- `todoist inbox` — list Inbox tasks
- `todoist today` — list tasks due today + overdue

### Task commands

```
todoist task list [--project X] [--label L] [--filter "query"] [--preset today|overdue|next7] [--json|--ndjson|--plain]
todoist task add --content "text" [--project X] [--labels L] [--due "text"] [--priority 1-4]
todoist task view <ref> [--full]
todoist task update --id <id> [flags]
todoist task complete --id <id>
todoist task delete --id <id>
```

### Agent commands

```
todoist agent plan <instruction> [--out <file>] [--planner <cmd>]
todoist agent apply --plan <file> --confirm <token> [--on-error fail|continue] [--dry-run]
todoist agent run --instruction <text> [--confirm <token>|--force]
todoist agent schedule print --weekly "sat 09:00" [--cron]
todoist agent planner --set --cmd "<cmd>"
```

## References

- Use `id:<id>` to explicitly reference IDs.
- Fuzzy name resolution is opt-in via `--fuzzy` / `TODOIST_FUZZY=1`.

## Output

- Human default for TTY; `--plain` (tab-separated) for stable text.
- `--json` emits envelope `{data, meta}`; `--ndjson` emits one JSON object per line.

## Errors

- Human errors include `request_id` when available.
- JSON errors: `{"error":"...", "meta":{"request_id":"..."}}`

## Config

Precedence: flags > env > project config > user config.

Config file: `~/.config/todoist/config.json`

