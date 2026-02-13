# Todoist CLI Specification

## Overview

Go-based CLI for Todoist. Binary name: `todoist`. Designed for humans and scripts, with stable `--json`, `--plain`, and `--ndjson` outputs.

## Authentication

- **Primary**: `TODOIST_TOKEN` environment variable
- **Fallback**: `~/.config/todoist/credentials.json`
- Profiles supported via `--profile` / `TODOIST_PROFILE`
- OAuth PKCE login supported via `todoist auth login --oauth` (client ID from `--client-id` or `TODOIST_OAUTH_CLIENT_ID`)
- OAuth endpoint/listen overrides: `TODOIST_OAUTH_AUTHORIZE_URL`, `TODOIST_OAUTH_TOKEN_URL`, `TODOIST_OAUTH_LISTEN`

## Command Structure

Pattern: `todoist <resource> <action> [args]`

### Top-level shortcuts

- `todoist add "text"` — Todoist quick add endpoint (full natural language; use `--strict` for REST add semantics)
- `todoist inbox` — list Inbox tasks
- `todoist today` — list tasks due today + overdue
- `todoist planner` — show/set planner command alias (same behavior as `todoist agent planner`)

### Task commands

```
todoist task list [--project X] [--label L] [--filter "query"] [--preset today|overdue|next7] [--json|--ndjson|--plain]
todoist task add --content "text" [--project X] [--labels L] [--due "text"] [--priority 1-4]
todoist task view <ref> [--full]
todoist task update --id <id> [flags]
todoist task complete --id <id>
todoist task delete --id <id> [--yes]
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
- `--json` emits raw arrays/objects; `--ndjson` emits one JSON object per line.
- `--quiet-json` emits compact single-line JSON errors (useful for agents and log pipelines).
- `todoist schema` is the output contract source of truth (for example: `task_list` and `task_item_ndjson`).

## Parsing Rules

- Global flags may appear before or after commands/subcommands.
- Subcommand flags may be interspersed with positional references (for example `todoist add "Buy milk" --project Home --dry-run`).
- Common aliases: `ls=list`, `rm/delete=delete`; plus `task show=view`.
- For destructive task deletion, `todoist task delete` requires explicit `--yes`.

## Errors

- Human errors include `request_id` when available.
- JSON errors: `{"error":"...", "meta":{"request_id":"..."}}`

## Config

Precedence: flags > env > project config > user config.

Config file: `~/.config/todoist/config.json`
