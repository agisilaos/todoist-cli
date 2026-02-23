# Todoist CLI Specification

## Overview

Go-based CLI for Todoist. Binary name: `todoist`. Designed for humans and scripts, with stable `--json`, `--plain`, and `--ndjson` outputs.

## Authentication

- **Primary**: `TODOIST_TOKEN` environment variable
- **Fallback**: `~/.config/todoist/credentials.json`
- Profiles supported via `--profile` / `TODOIST_PROFILE`
- OAuth PKCE login supported via `todoist auth login --oauth` (client ID from `--client-id` or `TODOIST_OAUTH_CLIENT_ID`)
- OAuth device login supported via `todoist auth login --oauth-device`
- OAuth endpoint/listen overrides: `TODOIST_OAUTH_AUTHORIZE_URL`, `TODOIST_OAUTH_TOKEN_URL`, `TODOIST_OAUTH_DEVICE_URL`, `TODOIST_OAUTH_LISTEN`

## Command Structure

Pattern: `todoist <resource> <action> [args]`

### Top-level shortcuts

- `todoist add "text"` — Todoist quick add endpoint (full natural language; use `--strict` for REST add semantics)
- `todoist inbox` — list Inbox tasks
- `todoist today` — list tasks due today + overdue
- `todoist completed` — shortcut for completed task history (`task list --completed`)
- `todoist upcoming [days]` — list tasks due from today through the next N days
- `todoist planner` — show/set planner command alias (same behavior as `todoist agent planner`)
- `todoist doctor` — run local environment/auth/API health checks

### Task commands

```
todoist task list [--project X] [--label L] [--filter "query"] [--preset today|overdue|next7] [--json|--ndjson|--plain]
todoist task add --content "text" [--project X] [--labels L] [--due "text"] [--priority 1-4] [--assignee <id|me|name|email>]
todoist task view <ref> [--full]
todoist task update --id <id> [flags]
todoist task complete --id <id>
todoist task delete --id <id> [--yes]
```

### Filter commands

```
todoist filter list
todoist filter show <id|name>
todoist filter add --name <name> --query <query>
todoist filter update <id|name> [--name <name>] [--query <query>]
todoist filter delete <id|name> --yes
```

### Reminder commands

```
todoist reminder list [task] [--task <ref>]
todoist reminder add [task] [--task <ref>] (--before <duration> | --at <datetime>)
todoist reminder update [id] [--id <id>] (--before <duration> | --at <datetime>)
todoist reminder delete [id] [--id <id>] [--yes]
```

### Notification commands

```
todoist notification list [--type <types>] [--unread|--read] [--limit <n>] [--offset <n>]
todoist notification read [id] [--id <id>] [--all --yes]
todoist notification unread [id] [--id <id>]
```

### Activity command

```
todoist activity [--since <date>] [--until <date>] [--type task|comment|project] [--event <type>] [--project <id|name>] [--by <id|me>] [--limit <n>] [--cursor <cursor>] [--all]
```

### Stats command

```
todoist stats
```

### Agent commands

```
todoist agent plan <instruction> [--out <file>] [--planner <cmd>]
todoist agent apply --plan <file> --confirm <token> [--on-error fail|continue] [--dry-run] [--policy <file>]
todoist agent run --instruction <text> [--confirm <token>|--force] [--policy <file>]
todoist agent schedule print --weekly "sat 09:00" [--cron]
todoist agent planner --set --cmd "<cmd>"
```

Planner action schema notes:

- `task_move` accepts either `project`/`section` references or explicit `project_id`/`section_id`.
- `section_add` accepts `project` or `project_id`.
- `comment_add` requires `content` plus `task_id` or `project`/`project_id`.
- `reason` is an optional action field for explanation in human plan previews.

Planner context notes:

- Planner request context includes `projects`, `sections`, `labels`, `active_tasks` (capped), and optional `completed_tasks`.

## References

- Use `id:<id>` to explicitly reference IDs.
- Task/project/label/filter refs also accept Todoist app URLs (`https://app.todoist.com/app/<entity>/...`).
- Fuzzy name resolution is opt-in via `--fuzzy` / `TODOIST_FUZZY=1`.
- Accessibility labels for human task output are opt-in via `--accessible` / `TODOIST_ACCESSIBLE=1`.

## Output

- Human default for TTY; `--plain` (tab-separated) for stable text.
- `--json` emits raw arrays/objects; `--ndjson` emits one JSON object per line.
- `--quiet-json` emits compact single-line JSON errors (useful for agents and log pipelines).
- `todoist schema` is the output contract source of truth (for example: `task_list` and `task_item_ndjson`).
- `--progress-jsonl[=path]` emits agent progress events as JSONL (stderr or file).
- In human mode, `--accessible` adds explicit `due:` and `p<priority>` task markers.

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
