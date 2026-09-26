# Todoist CLI Specification

## Overview

Go-based CLI for Todoist. Binary name: `todoist`. Designed for humans and scripts, with stable `--json`, `--plain`, and `--ndjson` outputs.

## Authentication

- **Primary**: `TODOIST_TOKEN` environment variable
- **Stored profile**: when `TODOIST_TOKEN` is absent, load the selected profile from its recorded backend. New profiles default to macOS Keychain; portable file storage requires explicit selection. Profile metadata and native references remain in `~/.config/todoist/credentials.json`; only file-backed profiles store tokens there. See [credential storage](#credential-storage-contract).
- Profiles supported via `--profile` / `TODOIST_PROFILE`
- OAuth PKCE login supported via `todoist auth login --oauth` (client ID from `--client-id` or `TODOIST_OAUTH_CLIENT_ID`)
- The existing configurable device-flow client is available via `todoist auth login --oauth-device`; live Todoist device support is unverified.
- OAuth endpoint/listen overrides: `TODOIST_OAUTH_AUTHORIZE_URL`, `TODOIST_OAUTH_TOKEN_URL`, `TODOIST_OAUTH_DEVICE_URL`, `TODOIST_OAUTH_LISTEN`

## Authorization

- Both OAuth flows accept `--read-only`: `todoist auth login --oauth --read-only` or `todoist auth login --oauth-device --read-only`. The flag requires an OAuth flow and cannot assert scopes for manually supplied tokens.
- Read-only requests use `data:read`. Default read-write requests use `data:read_write,data:delete,project:delete`.
- Successful exchanges store per-profile authorization metadata version 1: `mode`, `origin`, `requested_scopes`, `effective_scopes`, and `scope_evidence`. Explicit returned scopes take precedence; an omitted response scope uses the requested set according to OAuth. Broader read-only grants and unsupported grants fail without replacing the credential. Reduced read-write grants may produce read-only credentials.
- `auth status` remains offline. Its existing `profile`, `configured`, and `source` fields are retained; `authorization` adds safe evidence and effective `write_capable`/`write_capability_reason`. Missing credentials have null mode, unknown scopes are null rather than an empty grant, and source remains `env` or `credentials` when configured.
- The versioned format and complete reporting fields are specified in [authorization-design.md](authorization-design.md). Published schemas include `authorization`, `auth_status`, and `doctor`.
- Legacy credentials load as unknown without rewriting the file. Manual and environment tokens are also unknown; all three permit attempted writes for compatibility, without inferring OAuth scopes. Environment credentials override profile metadata.
- Present but invalid/unsupported metadata blocks authenticated operations. Status and doctor remain available for inspection. Login/logout can repair a selected record in a structurally readable credentials file. Failed persistence preserves the old file, and saving one profile preserves other profiles and unknown fields.
- Read-only mode blocks every Todoist mutation through a central API guard. Resource reads, local operations, planning, and dry-run previews remain allowed. See the [complete command classification](authorization-command-inventory.md); notification `read`, account settings, goals, and invitation responses are mutations.
- Agent apply/run check pending actions before the first mutation; denial is fatal even with `--force` or `--on-error=continue`. Replay-only reruns perform no new mutation and do not acquire a new application timestamp. No read/local action types are added to agent plans.
- Planner requests, previews, and agent status expose current authorization. It is advisory for planning and never persisted as permission in an executable plan. Application resolves the current credential again. The existing replay journal remains scoped to the configuration directory, not an account.
- Schedules preserve profile/config selections and relevant explicit endpoint/policy settings, without embedding tokens. `TODOIST_TOKEN` still takes precedence at execution. Authorization is checked when the scheduled command runs.
- `--print-env` remains an explicit secret-bearing export without persistence. Subsequent environment use has unknown authorization. Logout removes stored credential metadata without revocation or environment changes.
- Read-only and unknown authorization alone are healthy doctor states. A doctor API probe establishes acceptance at probe time, not scope discovery. Token refresh is outside this feature; authorization metadata does not establish permanent token validity.
- Local metadata is not a security boundary against editing credential files or running an older binary. External planners run independently and are not sandboxed by the CLI's mutation guard.

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
- `todoist view <url>` — open Todoist web URLs with equivalent CLI commands

### Task commands

```
todoist task list [--project X] [--label L] [--filter "query"] [--preset today|overdue|next7] [--json|--ndjson|--plain]
todoist task add --content "text" [--project X] [--label L] [--due "text"] [--priority 1-4] [--assignee <id|me|name|email>]
todoist task view <ref> [--full]
todoist task update --id <id> [flags]
todoist task complete --id <id>
todoist task delete --id <id> --yes
```

### Filter commands

Saved filters are read and mutated through `/sync`. Mutations require a successful per-command acknowledgement; deleted filters are omitted from lists.

```
todoist filter list
todoist filter show <id|name>
todoist filter add --name <name> --query <query>
todoist filter update <id|name> [--name <name>] [--query <query>]
todoist filter delete <id|name> --yes
```

### Project commands

```
todoist project list [--archived]
todoist project view <id|name>
todoist project browse <id|name>
todoist project collaborators <id|name>
todoist project add --name <name> [flags]
todoist project create --name <name> [flags]
todoist project update --id <id> [flags]
todoist project move <id|name> (--to-workspace <id|name> | --to-personal) [--visibility restricted|team|public] [--yes]
todoist project archive --id <id>
todoist project unarchive --id <id>
todoist project delete --id <id>
```

### Reminder commands

```
todoist reminder list (<task> | --task <ref>)
todoist reminder add (<task> | --task <ref>) (--before <duration> | --at <datetime>)
todoist reminder update (<id> | --id <id>) (--before <duration> | --at <datetime>)
todoist reminder delete (<id> | --id <id>) [--yes]
```

### Notification commands

```
todoist notification list [--type <types>] [--unread|--read] [--limit <n>] [--offset <n>]
todoist notification view [id] [--id <id>]
todoist notification accept [id] [--id <id>]
todoist notification reject [id] [--id <id>]
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
todoist stats goals [--daily <n>] [--weekly <n>]
todoist stats vacation (--on | --off)
```

### Settings command

```
todoist settings
todoist settings view
todoist settings update [--timezone <tz>] [--time-format <12|24>] [--date-format <us|intl>] [--start-day <day>] [--theme <name>] [--auto-reminder <minutes>] [--next-week <day>] [--start-page <page>] [--reminder-push <on|off>] [--reminder-desktop <on|off>] [--reminder-email <on|off>] [--completed-sound-desktop <on|off>] [--completed-sound-mobile <on|off>]
todoist settings themes
```

Notes:
- `settings view` uses human-friendly labels for time/date/day/theme values.
- Start-page refs (`project?id=...`, `label?id=...`, `filter?id=...`) are resolved to display names on a best-effort basis.

### View command

```
todoist view <url>
```

Notes:
- Supports Todoist entity URLs for task/project/label/filter.
- Supports page URLs like `/app/inbox`, `/app/today`, `/app/upcoming`, `/app/completed`, `/app/settings`, `/app/activity`.
- Project URLs use best-effort slug/name fallback when legacy URL IDs are rejected by API v1.

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
- `--json` emits raw arrays/objects; empty lists are `[]`, never `null`. `--ndjson` emits one JSON object per line. Mutation acknowledgements, dry runs, doctor reports, and agent/planner results emit one record with the same payload as `--json`. Completion script generation still emits shell source.
- `--ids-only` is an additive machine output contract: one raw, opaque ID followed by LF per result, without headings, metadata, quoting, or empty-state text. Empty results emit zero stdout bytes.
- Supported commands: `task list`, `project list`, `project collaborators`, `section list`, `label list`, `comment list`, `filter list`, `workspace list`, `reminder list`, `notification list`, `activity`, `completed`, `today`, `upcoming`, bare `inbox`, and `filter show`, including existing `ls` aliases. Collaborators emit user IDs; activity emits event IDs, not object IDs.
- Preserve command result order (including existing sorting), duplicates, fetching defaults, and `--all` behavior. Validate the whole fetched collection before output: missing IDs or IDs containing whitespace/control characters fail with exit 1.
- In ID mode only, remaining pages produce stderr notices: `More available. Use --cursor "<cursor>"` (quoted with escapes) or `More available. Use --offset N` for notifications. Empty pages may have notices; exhausted collections do not. Pagination never appears on stdout.
- `--ids-only` conflicts with `--json`, `--plain`, and `--ndjson`. Conflicts and unsupported commands fail before side effects with usage exit 2, empty stdout, and the existing JSON error envelope on stderr. Mutations, single-object views, resources without stable IDs, and `view URL` are unsupported.
- `todoist schema --name ids_only` returns a wire-format descriptor, not a JSON payload schema. Existing payload schemas and existing output modes remain unchanged.
- `--quiet-json` emits compact single-line JSON errors (useful for agents and log pipelines).
- `todoist schema` is the output contract source of truth (for example: `task_list` and `task_item_ndjson`).
- `--progress-jsonl[=path]` emits agent progress events as JSONL (stderr or file).
  Event stream includes planner/apply lifecycle markers such as `agent_plan_loaded`,
  `agent_action_validated`, `agent_action_dispatched`, `agent_action_succeeded`,
  `agent_action_failed`, and `agent_apply_summary`.
- An agent action succeeds only after the Todoist mutation and its replay record both succeed. `agent_action_complete` and `agent_action_succeeded` are emitted only after durable recording.
- A replay-record failure emits `agent_action_error` and `agent_action_failed` with `stage: "replay_record"` and `remote_succeeded: true`, then terminates application even under `--on-error=continue`.
- Successful Todoist mutations are recorded immediately by replacing `agent_replay.json`; replay skips, action failures, and duplicate records do not write the file. Interruption after Todoist accepts a mutation but before its replay record is installed can leave that mutation unrecorded and make a rerun duplicate it. The journal is unbounded, assumes a single applying process, and delegates replacement visibility to the underlying OS and filesystem; it does not promise OS- or storage-power-loss durability.
- Human agent apply/run output includes a compact summary block with success/failure/replay counts,
  destructive-action count, per-action-type totals, and final outcome.
- In human mode, `--accessible` adds explicit `due:` and `p<priority>` task markers.

## Parsing Rules

- Global flags may appear before or after commands/subcommands.
- `--ids-only` is parsed globally and validated against command eligibility. Like existing boolean global flags, it accepts the exact spelling, not `--ids-only=true`; global parsing stops at `--`.
- Help remains available when configuration cannot be loaded. `doctor` reports a failing config check with its path and skips dependent checks; repair the JSON or file access before retrying other commands.
- Explicit `--help`/`-h` requests show command usage before validating mutation arguments or contacting the API.
- Existing informational precedence applies: version wins over output conflicts, conflicts precede help, and root/command help remains available with `--ids-only` (an exception to ID-only stdout).
- Subcommand flags may be interspersed with positional references (for example `todoist add "Buy milk" --project Home --dry-run`).
- Common aliases: `ls=list`, `rm/del=delete`; plus `task show=view`.
- For destructive task deletion, `todoist task delete` requires explicit `--yes`.

## Errors

- Human errors include `request_id` when available.
- JSON errors retain `{"error":"...", "meta":{"request_id":"..."}}`. Authorization failures additionally expose a stable `code` and safe `details` containing profile, source, and authorization; they exit 3 and remain on stderr. `--json --quiet-json` compacts the same payload.
- `READ_ONLY`: `Todoist mutation blocked: the active credential is read-only.`
- `AUTH_METADATA_INVALID`: `Stored authorization metadata is invalid; log in again or remove the affected profile.`
- `AUTH_METADATA_UNSUPPORTED`: `Stored authorization metadata uses an unsupported version; upgrade the CLI or replace the affected credential.`
- `OAUTH_SCOPE_INVALID`: `OAuth returned an unacceptable scope grant; the stored credential was not changed.`
- Doctor retains its existing diagnostic-failure exit behavior and report shape.
- ID-mode errors use the same JSON envelope on stderr, including global parsing, output conflicts, unsupported commands, and runtime failures. `--quiet-json` compacts the envelope; existing runtime exit codes remain 1 (generic), 3 (auth), 4 (not found), and 5 (conflict). Usage errors return 2. Invocations without `--ids-only` retain their existing error behavior.

## Config

Precedence: flags > env > project config > user config.

Config file: `~/.config/todoist/config.json`

## Credential storage contract

- New profiles default to native storage; the first native backend is macOS Keychain through cgo/Security.framework. Unsupported platforms/builds return a stable unavailable error. Explicit file storage remains portable under ADR-0001; native failure never selects plaintext automatically.
- Login accepts `--credential-store=native|file`. The user-only `credential_store` configuration selects the default for new profiles. Project config cannot override it. Existing profile descriptors select their backend; changing defaults does not migrate them. Conflicting login flags are usage errors.
- `todoist auth migrate --credential-store=native` explicitly migrates the selected profile; `--credential-store=file` explicitly restores portable plaintext storage. Verify the native destination before atomically publishing its reference and removing the plaintext token. Preserve authorization metadata, unrelated profiles, and unknown fields.
- `todoist auth repair` reconciles an interrupted transaction or retries cleanup for the selected profile. Writers serialize with a bounded wait. Recovery records contain identifiers but no tokens. Before publication the original remains active; after a confirmed switch the new credential remains active even if obsolete-entry cleanup fails. Uncertain publication returns recovery required.
- Logout durably disables the profile before native deletion. A failed deletion is cleanup pending, never successful removal; repeated logout retries it. Environment tokens remain independent.
- Native records use a versioned per-profile storage descriptor in credentials.json alongside the existing authorization object, with no plaintext token. Descriptors bind opaque native entries to canonical configuration directory and profile. Moving/copying the directory requires a new login; symlink aliases share the namespace. Unsupported or malformed descriptors are not legacy records.
- `auth status` keeps its existing fields and adds `backend`, `accessibility` (`unchecked`), and `recovery` (empty, `cleanup`, or `rollback`). Configured means a profile is recorded. Status, help and local commands never retrieve native secrets. Doctor reports backend health and retains its API probe; neither operation may trigger Keychain UI. Environment overrides bypass native access.
- Auth/storage failures exit 3 with stable symbolic codes: `CREDENTIAL_MISSING`, `CREDENTIAL_STORE_UNAVAILABLE`, `CREDENTIAL_STORE_DENIED`, `CREDENTIAL_STORE_INTERACTION_REQUIRED`, `CREDENTIAL_STORE_LOCKED`, `CREDENTIAL_STORE_CORRUPT`, `CREDENTIAL_STORE_UNSUPPORTED`, `CREDENTIAL_STORE_NAMESPACE_MISMATCH`, `CREDENTIAL_STORE_BUSY`, `CREDENTIAL_STORE_IO`, `CREDENTIAL_CLEANUP_PENDING`, and `CREDENTIAL_RECOVERY_REQUIRED`. Conflicting backend selection is `CREDENTIAL_STORE_SELECTION_CONFLICT`, exit 2. Cleanup errors expose `details.committed=true` only after the switch is established. Existing JSON/quiet-JSON envelopes, human/NDJSON error conventions and doctor exit rules remain intact.
- Native errors are classified before rendering; raw OS errors, malformed metadata and tokens never appear in diagnostics, subprocess arguments or test failures. Existing `auth login --print-env` remains the explicit token-output exception, including its structured export format, without persistence.
- No plaintext downgrade copy is retained. Older binaries cannot use migrated profiles and may discard references if they rewrite their files. Removing the current token does not erase external backups/snapshots.

The complete accepted transaction, isolation, and verification contract is in [credential-store-design.md](credential-store-design.md).

Auth status and logout accept no positional arguments or command-specific flags other than help. Select profiles with `--profile`. Help is informational and never removes credentials; invalid arguments fail with usage exit 2 before mutation.

Successful auth login and logout support `--ndjson`, emitting one JSON record with the same fields as `--json`. Tokens are excluded except for the explicit login `--print-env` export.
