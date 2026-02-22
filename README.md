# todoist-cli

Agentic CLI for Todoist using the official Todoist API v1 (REST). It supports task, project, section, label, and comment management, plus an agent plan/apply workflow that can be wired to an external planner.

## Why this CLI

- Fast capture and triage without leaving the terminal.
- Scriptable output (`--json`/`--plain`/`--ndjson`) for automation and integrations.
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
- `TODOIST_OAUTH_CLIENT_ID` (OAuth client ID used by `auth login --oauth`)
- `TODOIST_OAUTH_AUTHORIZE_URL` (override OAuth authorize URL)
- `TODOIST_OAUTH_TOKEN_URL` (override OAuth token URL)
- `TODOIST_OAUTH_DEVICE_URL` (override OAuth device-code URL)
- `TODOIST_OAUTH_LISTEN` (override OAuth callback listen address)
- `TODOIST_FUZZY` (1 to enable fuzzy name resolution)
- `TODOIST_ACCESSIBLE` (1 to add screen-reader-friendly labels in human output)
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
--quiet-json         Compact single-line JSON errors (for scripts/agents)
-v, --verbose        Enable verbose output
--accessible         Add text markers for screen-reader-friendly task output
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
--fuzzy               Enable fuzzy name resolution
--no-fuzzy            Disable fuzzy name resolution
--progress-jsonl      Emit progress events as JSONL to stderr or file
--base-url <url>      Override API base URL
```

Flag parsing notes:

- Global flags can appear before or after commands/subcommands.
- Subcommand flags can be mixed with positional refs/content (for example `todoist add "Buy milk" --project Home --dry-run`).
- Common aliases: `ls`=`list`, `rm`/`del`=`delete` (`task`, `project`, `section`, `label`, `comment`), and `show`=`view` (`task`).
- Prefer `--json` or `--ndjson` for scripts/agents.

## Commands

### Auth

Manage Todoist credentials and profiles.

```
todoist auth login [--token-stdin] [--print-env]
todoist auth login --oauth [--client-id <id>] [--no-browser] [--print-env]
todoist auth login --oauth-device [--client-id <id>] [--print-env]
                  [--oauth-authorize-url <url>] [--oauth-token-url <url>]
                  [--oauth-device-url <url>] [--oauth-listen <host:port>] [--oauth-redirect-uri <uri>]
todoist auth status
todoist auth logout
```

- `auth login` prompts for a token (TTY) or reads from stdin with `--token-stdin`. Stores tokens in `~/.config/todoist/credentials.json` (0600).
- `auth login --oauth` runs OAuth PKCE via local callback (`http://127.0.0.1:8765/callback` by default). If browser auto-open fails, the command prints a warning and continues waiting for callback so you can open the URL manually.
- `auth login --oauth-device` runs OAuth Device Flow (good for headless/CI/SSH); it prints verification URL/code and polls until authorized.
- `auth status` prints active profile and whether a token is present.
- `auth logout` deletes stored credentials for the active profile.
- Use `--print-env` to emit `TODOIST_TOKEN=...` for piping into other tools (`--json`/`--ndjson` return structured output with the export string).

### Tasks

List and modify tasks (IDs or names accepted where noted).

```
todoist task list [--filter <query>] [--preset today|overdue|next7] [--project <id|name>] [--section <id|name>] [--label <name>] [--completed] [--completed-by completion|due] [--since <date>] [--until <date>] [--sort due|priority] [--truncate-width <cols>] [--wide] [--all-projects]
todoist task add --content <text> [flags]
todoist task view <ref> [--full]
todoist task update <ref> [flags]
todoist task move <ref> [--project <id|name>] [--section <id|name>] [--parent <id>]
todoist task move --filter <query> [--project <id|name>] [--section <id|name>] [--parent <id>] --yes
todoist task complete <ref>
todoist task complete --filter <query> --yes
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
--assignee <ref>           Assignee reference (id, me, name, email)
--natural                  Parse quick-add style tokens in content (#project @label p1..p4 due:...)
--yes                      Skip delete confirmation
```

Completed task listing:

```
todoist task list --completed [--completed-by completion|due] [--since <date>] [--until <date>]
```

Notes:
- `--since`/`--until` accept `YYYY-MM-DD`, RFC3339, `today`, `yesterday`, weekday names (for example `monday`), and relative forms like `2 weeks ago`.
- If you pass `--since` without `--until`, `--until` defaults to today.
- Bulk commands using `--filter` accept Todoist query syntax; plain text is treated as search text.
- `--strict` is a flag on `todoist add` (quick-add command), not on `todoist task add`.
- `task add/update --natural` lets you pass quick-add style tokens in `--content` (for example `#Home @errands p2 due:tomorrow`) and maps them to REST fields.
- Task references also support due hints for disambiguation: `"call mom today"`, `"call mom tomorrow"`, `"call mom overdue"`.

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

### Workspaces

```
todoist workspace list
todoist project collaborators <id|name>
```

- `workspace list` shows workspaces available to the authenticated user.
- `project collaborators` lists collaborators for a shared project.

### Filters

```
todoist filter list
todoist filter show <id|name>
todoist filter add --name <name> --query <query> [--color <color>] [--favorite]
todoist filter update <id|name> [--name <name>] [--query <query>] [--color <color>] [--favorite|--unfavorite]
todoist filter delete <id|name> --yes
```

### Inbox

Quick add to Inbox with optional defaults.

```
todoist inbox add --content <text> [--label <name> ...] [--due <string>|--due-date <date>|--due-datetime <datetime>] [--priority <1-4>] [--description <text>] [--section <id|name>]
```

Notes:
- Reads content from stdin with `--content -`.
- Applies defaults from config: `default_inbox_labels`, `default_inbox_due`.
- `todoist add <text>` uses Todoist quick add parsing for `#Project`, `@label`, `p1..p4`, and natural language dates.
- In quick-add mode, `--section` and project IDs are rejected; use `--strict` to fall back to REST add.
- In `--strict` mode, use REST-style flags (`--project Home`, `--label errands`, `--due tomorrow`) without `#`, `@`, or `due:` prefixes.

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
todoist agent apply <instruction> --confirm <token> [--planner <cmd>] [--policy <file>]
todoist agent apply --plan <file> --confirm <token>
todoist agent apply --plan <file> --confirm <token> --dry-run [--policy <file>]
todoist agent apply --plan <file> --confirm <token> --on-error fail|continue
todoist agent run --instruction <text> [--planner <cmd>] [--confirm <token>|--force] [--policy <file>]
todoist agent schedule print --weekly "sat 09:00" [--instruction <text>] [--planner <cmd>] [--confirm <token>|--force] [--cron]
todoist agent examples
todoist agent planner
todoist agent planner --set --cmd "<planner>"
todoist agent planner --json
todoist planner --json
todoist agent status
```

- `agent plan` sends context + instruction to an external planner command. Use `--out` to save the plan JSON.
- `agent apply` executes a plan from `--plan` or re-runs the planner; requires the `--confirm` token from the plan.
- `agent status` is safe on first run; it reports planner configuration and whether a last plan exists.
- `--dry-run` with `agent apply` prints the plan without applying actions.
- In `--dry-run`, no-action plans are allowed (useful for CI/pipeline contract checks).
- `--on-error=continue` keeps applying actions after a failure and reports statuses.
- `--plan-version` enforces expected plan.version (default 1). Unknown versions are rejected.
- `agent planner` shows/sets the planner command (uses config/planner_cmd or TODOIST_PLANNER_CMD).
- `agent run` combines plan + apply for automation (cron/launchd).
- `agent schedule print` emits a scheduler entry (launchd by default; use `--cron`).
- Context flags: `--context-project`, `--context-label`, `--context-completed 7d` limit planner context.
- Planner context now includes active tasks (capped) in addition to projects/sections/labels/completed tasks.
- `--policy <file>` enforces action-policy rules (`allow_action_types`, `deny_action_types`, `max_destructive_actions`).
- `--progress-jsonl[=path]` emits JSONL progress events for `agent run/apply` (stderr by default).
- Agent apply/run keeps a replay journal (`agent_replay.json`) and skips already-applied actions from the same plan token.

Planner contract checklist:
- Emit valid JSON to stdout matching `todoist schema --name plan`.
- Include `confirm_token` and `actions` with supported types.
- Optional: include `reason` per action for richer human review output.
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

### Doctor

Run environment and auth checks:

```bash
todoist doctor
todoist doctor --strict
```

`doctor` validates config/credentials health, token presence, API reachability, planner setup, policy parsing, and replay journal readability.

### Schema

Output JSON schemas (use `--json`):

```
todoist schema [--name task_list|task_item_ndjson|error|plan|plan_preview|planner_request] [--json]
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
- `--ndjson` outputs one JSON object per line (streaming friendly) across task/project/section/label/comment lists.
- Errors go to stderr; `--quiet` suppresses non-error informational messages. `--verbose` may show request IDs and more detail.
- Color is enabled by default on TTY; use `--no-color` or `NO_COLOR=1` to disable.
- `--accessible` (or `TODOIST_ACCESSIBLE=1`) adds explicit text markers for task due/priority values in human output.
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
    { "type": "task_move", "task_id": "123", "project_id": "2203306141" },
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

Action field notes:

- Task/section/comment actions accept explicit IDs (`project_id`, `section_id`) or reference fields (`project`, `section`) where applicable.
- `comment_add` must include `content` and one target: `task_id` or `project`/`project_id`.

## Release

```bash
make release-check VERSION=vX.Y.Z
make release-dry-run VERSION=vX.Y.Z
make release VERSION=vX.Y.Z
```

Release scripts:
- `scripts/docs-check.sh` validates release-related docs coverage in README.
- `scripts/release-check.sh` validates version/tag preconditions, runs tests/vet/docs/format checks, and verifies stamped version output.
- `scripts/release.sh` runs `release-check`, updates changelog from git history, builds darwin archives, publishes GitHub release/tag, and updates the Homebrew tap formula.

## Docs

- CLI specification: `docs/SPEC.md`
- Roadmap: `docs/ROADMAP.md`
- Release runbook: `RELEASING.md`
- Release history: `CHANGELOG.md`

## Limits (from Todoist docs)

- POST body size limit: 1 MiB
- Header size limit: 65 KiB
- Processing timeout: 15 seconds for standard requests

## Notes

- This CLI uses Todoist REST API v1 endpoints under `https://api.todoist.com/api/v1`.
- Keychain integration is not implemented; tokens are stored in a local credentials file.
- Some Todoist surfaces (for example reminders/notifications/stats/activity) are not implemented yet.
- Todoist is a trademark of Doist; this project is an independent, unofficial CLI.
- Shell completions are bundled via `todoist completion`.

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
