# todoist-cli

Agentic CLI for Todoist using the official Todoist API v1 (REST). It supports task, project, section, label, and comment management, plus an agent plan/apply workflow that can be wired to an external planner.

## Why this CLI

- Fast capture and triage without leaving the terminal.
- Scriptable output (`--json`/`--plain`) for automation and integrations.
- Agent workflows for bulk plans with safe previews and confirmations.

See `docs/SPEC.md` for the CLI contract and `docs/ROADMAP.md` for planned features.

## Why agents

- Batch changes with human review via plan/apply.
- Safer automation with `--dry-run`, `--confirm`, and `--on-error`.
- Easy scheduling without running a daemon.

## Quickstart

```bash
brew install agisilaos/tap/todoist-cli
todoist --version
todoist auth login                 # prompts for token (or use --token-stdin)
todoist add "Review PR 42"
todoist task list                  # lists Inbox tasks in a table
```

## Install

```bash
go build ./cmd/todoist
./todoist --help
```

Homebrew (recommended):

```bash
brew tap agisilaos/tap
brew install todoist-cli
```

## Auth

Use a personal API token from Todoist settings.

```bash
todoist auth login
```

Other options:

```bash
todoist auth login --token-stdin < token.txt
TODOIST_TOKEN=... todoist task list
```

Tokens are stored in `~/.config/todoist/credentials.json` with `0600` permissions. Set `TODOIST_TOKEN` to override stored tokens.

## Config

User config (non-secrets):

- `~/.config/todoist/config.json`

Project config (non-secrets only):

- `./.todoist.json`

Example `config.json`:

```json
{
  "base_url": "https://api.todoist.com/api/v1",
  "timeout_seconds": 10,
  "default_profile": "default",
  "default_inbox_labels": ["inbox"],
  "default_inbox_due": "today",
  "table_width": 120
}
```

Precedence (high → low):

1. Flags
2. Environment variables
3. Project config (`.todoist.json`)
4. User config (`~/.config/todoist/config.json`)

Environment variables:

- `TODOIST_TOKEN`
- `TODOIST_PROFILE`
- `TODOIST_CONFIG`
- `TODOIST_TIMEOUT`
- `TODOIST_BASE_URL`
- `TODOIST_FUZZY` (1 to enable fuzzy name resolution)
- `TODOIST_TABLE_WIDTH` (override table width for human output)

## Usage

```
todoist [global flags] <command> [args]
```

Global flags apply to every command:

```
-h, --help           Show help
--version            Show version
-q, --quiet          Suppress non-essential output
-v, --verbose        Enable verbose output
--json               JSON output
--plain              Plain text output
--ndjson             NDJSON output
--no-color           Disable color
--no-input           Disable prompts
--timeout <seconds>  Request timeout (default 10)
--config <path>      Config file path
--profile <name>     Profile name (default "default")
-n, --dry-run         Preview changes without applying
-f, --force           Skip confirmation prompts
--base-url <url>      Override API base URL
```

## Commands

### Auth

Manage Todoist credentials and profiles.

```
todoist auth login [--token-stdin] [--print-env]
todoist auth status
todoist auth logout
```

- `auth login` prompts for a token (TTY) or reads from stdin with `--token-stdin`. Stores tokens in `~/.config/todoist/credentials.json` (0600).
- `auth status` prints active profile and whether a token is present.
- `auth logout` deletes stored credentials for the active profile.
- Use `--print-env` to emit `TODOIST_TOKEN=...` for piping into other tools.

### Tasks

List and modify tasks (IDs or names accepted where noted).

```
todoist task list [--filter <query>] [--preset today|overdue|next7] [--project <id|name>] [--section <id|name>] [--label <name>] [--completed] [--completed-by completion|due] [--since <date>] [--until <date>] [--sort due|priority] [--truncate-width <cols>] [--wide] [--all-projects]
todoist task add --content <text> [flags]
todoist task view <ref> [--full]
todoist task update <ref> [flags]
todoist task move <ref> [--project <id|name>] [--section <id|name>] [--parent <id>]
todoist task complete <ref>
todoist task reopen <ref>
todoist task delete <ref> [--yes]
```

Task flags:

By default, `todoist task list` shows your Inbox tasks. Use `--all-projects` or a filter to list across projects.

```
--content <text>           Task content ("-" reads stdin)
--description <text>       Task description
--project <id|name>        Project reference
--section <id|name>        Section reference
--parent <id>              Parent task ID
--label <name>             Label name (repeatable)
--priority <1-4>           Priority (accepts p1..p4)
--due <string>             Natural language due
--due-date <YYYY-MM-DD>    Due date
--due-datetime <RFC3339>   Due date/time
--due-lang <code>          Due language
--duration <minutes>       Duration in minutes
--duration-unit <unit>     Duration unit (minute/day)
--deadline <YYYY-MM-DD>    Deadline date
--assignee <id>            Assignee ID
--yes                      Skip delete confirmation
```

Completed task listing:

```
todoist task list --completed [--completed-by completion|due] [--since <date>] [--until <date>]
```

Table options:

```
--wide    Wider columns for table output
--all-projects    List tasks from all projects (default is Inbox)
--preset today|overdue|next7    Shortcut filters (ignored if --filter set)
--sort due|priority             Client-side sort for active tasks
--truncate-width <cols>         Override table width (human output)
```

Examples:

- `todoist task list --filter "@work & today"` (human table)
- `todoist task list --preset today --sort priority`
- `todoist task list --completed --since "2 weeks ago" --json`
- `echo "Write launch blog #Marketing @writing p2 due:friday" | todoist add --content -`
- `todoist task move --id 123 --project "Personal" --section "Errands"`
- `todoist task view id:123456 --full`
 - `todoist task complete "Pay rent"`

### Inbox

Quick add to Inbox with optional defaults.

```
todoist inbox add --content <text> [--label <name> ...] [--due <string>|--due-date <date>|--due-datetime <datetime>] [--priority <1-4>] [--description <text>] [--section <id|name>]
```

Notes:
- Reads content from stdin with `--content -`.
- Applies defaults from config: `default_inbox_labels`, `default_inbox_due`.
- `todoist add <text>` uses Sync API quick add parsing for `#Project`, `@label`, `p1..p4`, and natural language dates (no `--section` or project IDs).

Examples:
- `echo "Capture idea" | todoist inbox add --content -`
- `todoist inbox add --content "Pay rent" --label finance --due "1st"`
- `todoist add "Pay rent #Home p2 due:tomorrow"`
- `todoist inbox` (list inbox tasks)

### Today

Quick list of tasks due today and overdue.

```
todoist today
```

### Projects

Create and manage projects.

```
todoist project list [--archived]
todoist project add --name <name> [--description <text>] [--parent <id|name>]
todoist project update --id <project_id> [flags]
todoist project archive --id <project_id>
todoist project unarchive --id <project_id>
todoist project delete --id <project_id>
```

Examples:

- `todoist project list --archived --json`
- `todoist project add --name "Side Projects" --description "Weekend hacks"`
- `todoist project update --id 234 --name "Side Projects (2024)"`

### Sections

Create and manage sections within projects.

```
todoist section list [--project <id|name>]
todoist section add --name <name> --project <id|name>
todoist section update --id <section_id> --name <name>
todoist section delete --id <section_id>
```

Example: `todoist section add --name "Backlog" --project "Side Projects"`

### Labels

Create and manage labels.

```
todoist label list
todoist label add --name <name> [--color <color>] [--favorite]
todoist label update --id <label_id> [--name <name>] [--color <color>] [--favorite | --unfavorite]
todoist label delete --id <label_id>
```

Example: `todoist label add --name focus --color red --favorite`

### Comments

Create and manage comments for tasks or projects.

```
todoist comment list --task <id> | --project <id>
todoist comment add --content <text> (--task <id> | --project <id>)
todoist comment update --id <comment_id> --content <text>
todoist comment delete --id <comment_id>
```

Examples:

- `todoist comment list --task 123 --json`
- `todoist comment add --task 123 --content "Need QA sign-off"`

### Agent

Integrate with an external planner to generate and apply bulk plans.

```
todoist agent plan <instruction> [--out <file>] [--planner <cmd>]
todoist agent apply <instruction> --confirm <token> [--planner <cmd>]
todoist agent apply --plan <file> --confirm <token>
todoist agent apply --plan <file> --confirm <token> --dry-run
todoist agent apply --plan <file> --confirm <token> --on-error fail|continue
todoist agent run --instruction <text> [--planner <cmd>] [--confirm <token>|--force]
todoist agent schedule print --weekly "sat 09:00" [--instruction <text>] [--planner <cmd>] [--confirm <token>|--force] [--cron]
todoist agent examples
todoist agent planner show|--set --cmd "<planner>"
todoist agent planner --json
todoist agent status
```

- `agent plan` sends context + instruction to an external planner command. Use `--out` to save the plan JSON.
- `agent apply` executes a plan from `--plan` or re-runs the planner; requires the `--confirm` token from the plan.
- `agent status` shows planner command and last run status.
- `--dry-run` with `agent apply` prints the plan without applying actions.
- `--on-error=continue` keeps applying actions after a failure and reports statuses.
- `--plan-version` enforces expected plan.version (default 1). Unknown versions are rejected.
- `agent planner` shows/sets the planner command (uses config/planner_cmd or TODOIST_PLANNER_CMD).
- `agent run` combines plan + apply for automation (cron/launchd).
- `agent schedule print` emits a scheduler entry (launchd by default; use `--cron`).
- Context flags: `--context-project`, `--context-label`, `--context-completed 7d` limit planner context.

Planner contract checklist:
- Emit valid JSON to stdout matching `todoist schema --name plan`.
- Include `confirm_token` and `actions` with supported types.
- Use stable action fields (IDs or names as documented).

Scheduling example (macOS launchd):

```bash
todoist agent schedule print --weekly "sat 09:00" --instruction "Move 3 articles from Learning to Today" > ~/Library/LaunchAgents/com.todoist.agent.weekly.plist
launchctl load ~/Library/LaunchAgents/com.todoist.agent.weekly.plist
```

Cron example:

```bash
todoist agent schedule print --weekly "sat 09:00" --instruction "Move 3 articles from Learning to Today" --cron
```

Context scoping example:

```bash
todoist agent run --instruction "Pick 3 articles for today" --context-project "Learning" --context-label article --context-completed 7d
```

### Schema

Output JSON schemas (use `--json`):

```
todoist schema [--name task_list|error|plan|plan_preview|planner_request] [--json]
```

## Shell Completions

Generate a completion script for your shell:

```bash
todoist completion bash > /usr/local/etc/bash_completion.d/todoist
todoist completion zsh  > "${fpath[1]}/_todoist"
todoist completion fish > ~/.config/fish/completions/todoist.fish

# Or install to a sensible default location:
todoist completion install bash
```

Restart your shell or `source` the generated file to enable completions.

## Finding IDs

Some operations require IDs (e.g., task update/complete/delete; project archive/delete). Use list commands in `--plain` or `--json` mode to locate IDs:

```bash
todoist task list --filter "content:\"Write launch blog\"" --plain
todoist project list --plain
todoist comment list --task <task_id> --plain
todoist task list --completed --since "yesterday" --json | jq -r '.[].id'
```

Where supported, name resolution is built-in (e.g., `--project <name>` and `--section <name>` on task commands, `--label <name>`), but task IDs are required for update/complete/delete. Use `id:<id>` to explicitly reference IDs.

## Prompts & Safety

- Destructive commands (delete/archive) prompt when stdin is a TTY; `todoist task delete` requires `--yes` (no prompt). Use `--force` to skip prompts. In non-interactive mode (`--no-input`), destructive commands fail unless `--force` is set.
- `--dry-run` previews the actions that would be sent to Todoist without performing them.
- `--no-input` disables all prompts (auth included). Provide required flags or env vars to continue.

## Output

- TTY defaults to a human-readable table with truncated columns for readability and resolves project/section IDs to names when possible.
- Non-TTY defaults to `--plain` (tab-separated, no headers).
- `--json` outputs raw JSON arrays/objects (no envelope).
- `--ndjson` outputs one JSON object per line (streaming friendly).
- Errors go to stderr; `--quiet` suppresses non-error informational messages. `--verbose` may show request IDs and more detail.
- Color is enabled by default on TTY; use `--no-color` or `NO_COLOR=1` to disable.
- `--truncate-width` or `TODOIST_TABLE_WIDTH` lets you set table width; `--wide` expands columns.
- Fuzzy name resolution can be enabled with `--fuzzy` or `TODOIST_FUZZY=1` (project/section/label names); `--no-fuzzy` disables.

Plain output columns:

- `task list`: `id, content, project_id, section_id, labels, due, priority, completed`
- `project list`: `id, name, parent_id, is_archived, is_shared`
- `section list`: `id, name, project_id, is_archived`
- `label list`: `id, name, color, is_favorite`
- `comment list`: `id, content, posted_at`

## Exit Codes

- `0` success
- `1` generic failure
- `2` invalid usage
- `3` auth error
- `4` not found
- `5` conflict
- Errors return human-readable messages; `--json` errors include `{"error": "...", "meta": {"request_id": "..."}}`.

## Agent Planner Integration

`todoist agent plan` delegates planning to an external command defined by `TODOIST_PLANNER_CMD` or `--planner`. The command must read JSON from stdin and output a plan JSON document to stdout.

Planner input schema:

```json
{
  "instruction": "Move overdue tasks to Catch Up",
  "profile": "default",
  "now": "2025-02-08T12:34:56Z",
  "context": {
    "projects": [ ... ],
    "sections": [ ... ],
    "labels": [ ... ]
  }
}
```

Plan output schema:

```json
{
  "version": 1,
  "instruction": "Move overdue tasks to Catch Up",
  "created_at": "2025-02-08T12:34:56Z",
  "confirm_token": "6f2b",
  "summary": { "tasks": 12, "projects": 0, "sections": 0, "labels": 1, "comments": 0 },
  "actions": [
    { "type": "task_move", "task_id": "123", "project": "Catch Up" },
    { "type": "task_update", "task_id": "123", "labels": ["overdue"] }
  ]
}
```

Supported action types:

- `task_add`, `task_update`, `task_move`, `task_complete`, `task_reopen`, `task_delete`
- `project_add`, `project_update`, `project_archive`, `project_unarchive`, `project_delete`
- `section_add`, `section_update`, `section_delete`
- `label_add`, `label_update`, `label_delete`
- `comment_add`, `comment_update`, `comment_delete`

## Limits (from Todoist docs)

- POST body size limit: 1 MiB
- Header size limit: 65 KiB
- Processing timeout: 15 seconds for standard requests

## Notes

- This CLI uses Todoist REST API v1 endpoints under `https://api.todoist.com/api/v1`.
- Keychain integration is not implemented; tokens are stored in a local credentials file.
- Sync API-only features (filters/reminders/workspaces) are not implemented in this version.
- Todoist is a trademark of Doist; this project is an independent, unofficial CLI.
- Shell completions: not bundled yet; generate with your shell’s standard tools if desired.

## Examples

```bash
# List Inbox tasks with defaults
todoist task list

# List completed tasks finished this week in JSON
todoist task list --completed --since "monday" --json

# Add a task from stdin content
echo "Write release notes" | todoist task add --content -

# Move a task to a project/section by name
todoist task move --id 123456 --project "Side Projects" --section "Backlog"

# Add a label and favorite it
todoist label add --name focus --favorite

# Add a comment to a task
todoist comment add --task 123456 --content "Need QA sign-off"

# Generate a plan with an external planner and apply it
todoist agent plan "Clean up overdue tasks" --out plan.json
todoist agent apply --plan plan.json --confirm "$(jq -r .confirm_token plan.json)"
```
