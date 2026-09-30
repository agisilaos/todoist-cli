# Todoist CLI Specification

## Overview

A terminal companion for Todoist. Binary name: `todoist`. Everyday capture and viewing workflows share stable `--json`, `--plain`, `--ndjson`, and `--ids-only` contracts with machine clients.

## Authentication

- **Primary**: `TODOIST_TOKEN` environment variable
- **Stored profile**: when `TODOIST_TOKEN` is absent, load the selected profile from its recorded backend. New profiles default to macOS Keychain; portable file storage requires explicit selection. Profile metadata and native references remain in `~/.config/todoist/credentials.json`; only file-backed profiles store tokens there. See [credential storage](#credential-storage-contract).
- Manual login rejects embedded whitespace, control characters, surrounding quotes, and token-assignment input before HTTP dispatch (exit 2). It verifies the entered token with `GET /projects?limit=1` before saving or `--print-env`, independently of stored credentials and `TODOIST_TOKEN`. Rejection (401/403) returns exit 3; connectivity/server/response failures return exit 1. Failed validation leaves credentials unchanged and never echoes candidate tokens or provider response bodies. OAuth continues to validate through its exchange.
- Successful manual validation establishes authentication only; scopes remain unknown. JSON/NDJSON success fields are unchanged; human output confirms connection and storage with a next command.
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
- `todoist inbox` — list every page of active Inbox tasks; missing credentials return authentication exit 3
- `todoist today` — list tasks due today + overdue
- `todoist completed` — shortcut for completed task history (`task list --completed`)
- `todoist upcoming [days]` — list tasks due across N dates including today (default 7: today and the next 6 days, UTC), excluding overdue and undated tasks
- `todoist planner` — show/set planner command alias (same behavior as `todoist agent planner`)
- `todoist doctor` — run local environment/auth/API health checks
- `todoist view <url>` — open Todoist web URLs with equivalent CLI commands

### Daily review

```
todoist review [--filter <query>] [--out <file>] [--dry-run] [--json|--ndjson|--plain]
```

The [daily-review contract](review-design.md) defines selection, edits/moves,
confirmation, stale checks, cancellation, recovery, and complete task accounting.
Default selection is `overdue | today` across all projects and pages. Input must
be a terminal; `--no-input`, piped input, and `--force` fail with exit 2 before
API access. Empty sets succeed. Review plans extend agent plan version 1 with
optional versioned review metadata; existing resource outputs remain unchanged.
Actual application of review plans through `agent apply/run` emits `review_report`,
including on failure. Dry-run agent output keeps its existing preview envelope.
Review plans require fail-fast application. Pending remote uncertainty is retained
in the replay journal and blocks blind retries. All other exit conventions remain.

### Task commands

```
todoist task list [--project X] [--label L] [--filter "query"] [--preset today|overdue|next7] [--all-projects] [--all] [--json|--ndjson|--plain]
todoist task add --content "text" [--project X] [--label L] [--due "text"] [--priority 1-4] [--assignee <id|me|name|email>]
todoist task view <ref> [--full]
todoist task update --id <id> [flags]
todoist task complete --id <id>
todoist task delete --id <id> --yes
```

`task list` defaults to one page of active Inbox tasks when no project, section,
parent, label, task IDs, filter, preset, completed selection, or `--all-projects`
is supplied. `--all-projects` changes project scope; `--all` fetches every page.
Use `todoist task list --all-projects --all` for every active task across projects,
including undated tasks. `inbox`, `today`, and `upcoming` fetch every page.

Human active-task lists identify the effective selection, shown count, and page
coverage, including empty selections. `--quiet` suppresses headers and summaries.
Redirected default output and explicit machine output retain their existing
formats. Inbox lookup failure stops before fetching tasks and preserves
the underlying error exit code. A successful project lookup without an Inbox returns exit 4.
Both failures leave stdout empty; neither falls back to all projects.

Root help introduces everyday commands before organization, automation, and
setup/reference commands. Every supported root command remains listed, with a
concise machine-client entry point. Help uses plain text without requiring color
or a pager.

### Task capture feedback

Successful `add`, `add --strict`, `task add` (including `--natural`/`--quick`), and
`inbox add` produce a capture receipt in human output unless `--quiet` is set.
The receipt uses returned task state: content, project, optional section, due,
recurrence, priority, labels, and full ID. Name lookup is best-effort; failures show
returned IDs with `name unavailable` without failing creation. It adds no mutation
or follow-up task fetch, and introduces no prompt.

- Display priorities match Todoist's p1–p4 (`p2` → `Priority: 2`). API priorities
  remain 4–1 respectively. Numeric input flags and machine output remain unchanged.
- An explicit null due shows `No due date`; omitted due or other unavailable values
  show `Not returned`. An empty labels array shows `None`; missing/null labels are unknown.
- Dates/times are shown as returned, with the returned timezone where supplied,
  without local conversion. A time without an offset or timezone is labeled as such.
  Recurrence uses the returned boolean and, when recurring, its expression; an
  omitted recurrence flag stays unknown. A due expression alone is not a resolved date.
- Full text is retained and terminal control characters are escaped. Labeled lines
  wrap naturally without column truncation and do not depend on color or symbols.
- Recovery points to `task view id:<full-id>`, `task update --help`, and `task move
  --help`. No URL is invented. A response without an ID asks the user to verify in
  Todoist before retrying, rather than claiming confirmed creation.
- JSON remains a one-item task array, NDJSON one object, and plain/default piped
  output the existing TSV row. Response facts retained for receipts do not add
  serialized fields. IDs-only eligibility/rejection, errors, aliases, and exit codes
  are unchanged. Quiet retains its previous output, including the human table.
- Human capture dry runs show submitted text or structured fields and state that no
  task was created. Quick-add interpretation and natural-language due expressions
  remain unverified until submitted. Machine and quiet preview payloads/wording,
  authorization checks, and required name-resolution reads are unchanged.

Active-task overviews and human task detail use Todoist P1–P4; retained tables
and machine output keep their existing API priority rendering.

### Single-task action feedback

Terminal human single-task `task complete` and `task move` acknowledge accepted
mutations unless `--quiet` is set. Completion says `Completion accepted`, shows
the full ID and any already-resolved title as `Task before completion`. Known
pre-action `is_recurring=true` adds `Recurring task; next due date not returned.`
Unknown recurrence never becomes nonrecurring. Completion does not provide a
returned next date or establish that a recurring task is permanently finished.

Move says `Move accepted` and uses individually usable returned title and
destination IDs. Names come only from already-loaded context matching those IDs;
otherwise retain IDs with `name unavailable`. A returned section name must match
the returned project when known. A missing title may use `Task before move`.
Explicit empty/null section or parent information means none; omitted or malformed
fields remain unknown. Empty unrequested section/parent context is omitted.
Unusable response identity invalidates the advisory facts.

Missing destination details show submitted values labeled `Requested`, with
`Destination details unavailable.` Submitted and pre-action destination fields
are never substituted as confirmed outcomes. Accepted move-body processing is
optional, bounded to 256 KiB and the existing request timeout: read failure,
truncation, excessive size or invalid JSON cannot turn acceptance into failure
or trigger a retry. Required mutation failures retain their existing paths.

No additional reads, mutations, prompts or retries are introduced. Positional
exact references keep their existing task GET; explicit `--id` still bypasses it.
Named destinations retain their existing resolution reads and pagination. Existing
retry behavior may still make up to three attempts with the same request ID.

Human single-task dry runs show identity, requested changes and the existing
authorization summary, explicitly stating no task changed. Full text/IDs are
retained and terminal controls escaped. Quiet, JSON/NDJSON `{id,status}` success,
plain/default redirected acknowledgements, machine preview payloads, IDs-only
rejection, bulk summaries, agent-plan reporting, capture receipts, task detail and
selection are unchanged. No shared presentation framework or Task serialization
change is introduced.

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
- Active-task overviews always use text labels. Legacy task table/plain markers remain opt-in via `--accessible` / `TODOIST_ACCESSIBLE=1`.

### Task ambiguity

The existing numbered task choice shows each candidate's title and full ID,
project, returned concrete due date/time, `No due date`, or `Due unavailable`, and
available section, parent, and labels. A due expression without a resolved date
is labeled `(date unavailable)`. `Repeats` appears only when Todoist returned a
recurring due date. Project/section name lookup is best-effort with returned-ID
fallback. Displayed context does not alter matching,
candidate order/cap, or the selected task ID. Times retain returned offsets and
timezones without local conversion; unavailable context stays unknown.

The caller must choose a number explicitly. Enter cancels selection and returns
the existing ambiguity error; invalid selection returns usage exit 2. `--no-input`
or non-TTY stdin skips the choice and preserves ambiguity failure. Exact-ID and
Todoist task URL resolution remain unchanged. Fuzzy ranking may order multiple
candidates but never selects a mutation target automatically.

Machine ambiguity outputs remain unchanged: usage exit 2, empty stdout, and
the existing stderr error behavior. JSON `details.matches` remains an array of
titles without candidate IDs/context; plain and NDJSON errors remain text.
Consequently the ambiguity error alone cannot identify an exact retry target.
Machine clients use `task list --all-projects --all --no-input --json` to inspect
full IDs and task fields, then `task view id:<chosen-id> --full --no-input --json`
as needed, before deliberately retrying the intended mutation with that exact
ID. Structured ambiguity enrichment requires a separate compatibility decision.

## Output

- Terminal detection queries the descriptor's terminal status. Non-terminal
  devices such as `/dev/null` do not enable prompts or select
  automatic human output. Real terminals retain their interactive behavior.
- Human default for TTY; `--plain` (tab-separated) for stable text.
- `--json` emits raw arrays/objects; empty lists are `[]`, never `null`. `--ndjson` emits one JSON object per line. Single-task views, mutation acknowledgements, dry runs, doctor reports, and agent/planner results emit one record with the same payload as `--json`. Completion script generation still emits shell source.
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
- `--accessible` retains `due:` and `p<priority>` markers in legacy task tables/plain output; overviews already label dates and priorities explicitly.

### Active-task overview

The human default for `today`, `upcoming`, `inbox`, and active `task list` results
(including filters and presets) is a title followed by context. Titles wrap to
three lines maximum with an ellipsis; context includes Todoist P1–P4 priority,
due date/time, explicitly returned recurrence, project, and full ID. Existing
width-setting precedence applies. Context and headers wrap; indivisible metadata
may overflow exceptionally narrow terminals rather than lose IDs or timestamps.
Common wide Unicode characters count as two terminal cells; ambiguous characters
count as one. Terminal controls in overview fields are escaped.

Relative labels compare validated returned calendar dates with an explicit UTC
reference date. Display keeps existing due-date precedence, and a datetime-only
value is classified by its UTC day. Raw due times and returned timezone information
remain visible. A prior day is overdue; an earlier time today is still today.
Unparseable/missing information remains unknown; explicit null due means undated.
These display rules never change selection, sorting, or Todoist filter semantics.
Counts refer only to displayed tasks. No grouping or new client sorting occurs.

The overview reports all pages fetched only for an exhausted selection without
an incoming cursor. Continuations say so, including at the end. Partial pages
show `More available` and use the existing quoted cursor notice on stderr.
Empty selections have scope-specific messages; empty partial pages and exhausted
continuations do not claim the entire selection is empty. Quiet suppresses scope,
summary, and empty messages, but retains tasks and cursor notices.

Today requests up to 200 tasks per page and follows every cursor until exhaustion
in all output modes. This internal page size does not change selection, task
order, or explicit `task list --limit` behavior. A failed page remains an error;
Today does not emit a successful partial collection.

Project names reuse cached collection fetching with explicit ID/name-unavailable
fallback. No per-task fetches or default section enrichment are introduced.
`--wide` keeps the detailed table and API priority numbering while adding scope
and coverage. Completed history, saved `filter show`, mutations,
and quiet capture receipts keep their legacy rendering. Machine payloads and
stdout/stderr behavior are unchanged, including accessible plain output.

Implementation ownership: active command handlers supply effective scope and
pagination context to `task_overview.go`; `task_output.go` remains the legacy
renderer and machine-output path. Filters ignore other selection flags as before.
Upcoming carries its selection's UTC reference day into the header so a midnight
boundary during rendering cannot misstate its window. No domain vocabulary or
architecture decision changes are required.

### Task detail

Human `task view` renders the full title and description, named project/section,
Todoist P1–P4 priority, explicit current state, absolute due information,
recurrence, labels, and exact task ID. `--full` appends project/section/parent IDs,
added/updated timestamps, completion-time availability, and returned comment
count; it does not revert to API priority numbering.

Text wraps without truncation using the existing human width and terminal-cell
conventions. Preserve spacing, paragraph boundaries, explicit line breaks, and
literal Markdown. Escape terminal controls; metadata line breaks are escaped.
IDs, timestamps, and URLs stay indivisible and may overflow narrow widths.

Dates, times, offsets, and named timezone information remain as returned, without
relative labels or conversion. Preserve both calendar date and datetime when they
differ; identify naive timestamps without a returned timezone. Do not treat an
expression-only due object as a resolved date, or infer recurrence from wording.
Explicit null due means `No due date`; missing/incomplete due is `Not returned`.
Explicit empty description, labels, section/parent IDs, and completion time use
`None` where established; unknown information remains `Not returned`.

State uses the explicit returned `checked` flag. Keep any completion timestamp
independently; a timestamp cannot establish current state or prove earlier
mutations. Missing/null state and comment count remain unknown rather than false
or zero. Response-presence facts are nonserialized and leave existing payloads
unchanged.

Human enrichment reuses all-page project fetching and, only with nonempty project
and section IDs, exact-project-scoped section fetching unless a complete global
sections collection is already cached. Reuse includes successful empty collections;
failed or incomplete fetches cannot supply enrichment. Section names still require
both the returned section ID and project ID to match. Successful collection
lookups use the existing per-invocation cache, including across selection prompts;
external name changes during a prompt may remain unseen. There is no persistent cache.
Pagination, per-page timeout, and existing GET retries apply. Normally one project
and one applicable section collection request are added. Attempt section lookup
independently of project-name failure. Failed lookup retains ID plus
`(name unavailable; lookup failed)`; successful lookup without a name retains ID
plus `(name unavailable)`. These fallbacks keep task detail successful.

Nonhuman paths branch before enrichment. JSON remains the existing task object;
NDJSON emits the same object as one newline-terminated JSON record on a single
line. `--full` does not alter either payload. Redirected/default and explicit
plain retain their legacy labeled text, including API priority under `--full`.
Preserve stdout/stderr separation, existing exit codes, and IDs-only rejection.
Quiet/accessibility flags retain all detail fields.
Task selection, reference resolution, and mutations are unchanged.

Ownership: `task_output.go` dispatches human detail to `task_detail.go` and retains
the existing nonhuman path. No general presentation framework is introduced.

## Parsing Rules

- Global flags may appear before or after commands/subcommands.
- `--ids-only` is parsed globally and validated against command eligibility. Like existing boolean global flags, it accepts the exact spelling, not `--ids-only=true`; global parsing stops at `--`.
- Help remains available when configuration cannot be loaded. `doctor` reports a failing config check with its path and skips dependent checks; repair the JSON or file access before retrying other commands.
- Explicit `--help`/`-h` requests show command usage before validating mutation arguments or contacting the API.
- Existing informational precedence applies: version wins over output conflicts, conflicts precede help, and root/command help remains available with `--ids-only` (an exception to ID-only stdout).
- Subcommand flags may be interspersed with positional references (for example `todoist add "Buy milk" --project Home --dry-run`).
- Common aliases: `ls=list`, `rm/del=delete`; plus `task show=view`.
- For destructive task deletion, `todoist task delete` requires explicit `--yes`.

## Help and command recovery

- Root and explicit group help remain command overviews. All dispatched child
  leaves, including completion shell selectors and `agent schedule print`, have
  focused usage, command-specific flags, examples, and relevant recovery advice.
  Existing standalone command pages remain available. Each leaf points to root
  help for the complete global flag inventory.
- `task complete --help`, `--help task complete`, `help task complete`, and
  `task help complete` select the same page. `-h` also works. Existing aliases
  select canonical leaf pages; operands after a leaf do not become command paths.
  This does not introduce a positional `task complete help` syntax or change
  command-option placement and `--` parsing.
- Genuine help requests return 0 before configuration loading, credential access,
  progress-file creation, or command dispatch. They need no required mutation
  arguments and make no local writes or API calls. Flag-shaped option values stay
  literal; global parse errors, version, output conflicts, and IDs-only eligibility
  retain their existing precedence.
- Unknown help targets return usage exit 2, using the existing error rendering for
  the selected output mode. This replaces successful fallback to a broader page.
  The historical `help examples` topic remains supported.
- Ordinary unknown-command errors retain exit 2. Human recovery prints up to three
  equally closest canonical command paths within the current group, followed by
  a pointer to that group's help. Aliases participate in matching but duplicate
  canonical results are collapsed. No command is corrected or executed automatically.
- Matching counts insertions, deletions, substitutions, and adjacent transpositions.
  Input names of 3–5 characters allow one edit, and names of 6–64 characters allow
  two. Shorter/longer names and flag-shaped input get no suggestions. More than
  three best matches suppresses suggestions. Ties use alphabetical order.
- Suggestions are suppressed for explicit `--json`, `--ndjson`, `--plain`,
  `--ids-only`, or `--quiet-json`. Existing ordinary execution error messages,
  envelopes, streams, and codes remain unchanged in those modes, including root
  help on existing text-mode unknown-root errors. `--quiet-json` alone does not
  select JSON. Ordinary piped invocations without these flags can receive hints.
- Unknown execution commands retain existing configuration-error precedence.
  Reference resolution, unknown options, shell operands for completion
  install/uninstall, and unrelated runtime diagnostics are unchanged.

## Errors

- Human errors include `request_id` when available.
- JSON errors retain `{"error":"...", "meta":{"request_id":"..."}}`. Authorization failures additionally expose a stable `code` and safe `details` containing profile, source, and authorization; they exit 3 and remain on stderr. `--json --quiet-json` compacts the same payload.
- `READ_ONLY`: `Todoist mutation blocked: the active credential is read-only.`
- `AUTH_METADATA_INVALID`: `Stored authorization metadata is invalid; log in again or remove the affected profile.`
- `AUTH_METADATA_UNSUPPORTED`: `Stored authorization metadata uses an unsupported version; upgrade the CLI or replace the affected credential.`
- `OAUTH_SCOPE_INVALID`: `OAuth returned an unacceptable scope grant; the stored credential was not changed.`
- Doctor retains its existing diagnostic-failure exit behavior and report shape.
- ID-mode errors use the same JSON envelope on stderr, including global parsing, output conflicts, unsupported commands, and runtime failures. `--quiet-json` compacts the envelope; existing runtime exit codes remain 1 (generic), 3 (auth), 4 (not found), and 5 (conflict). Usage errors return 2. Invocations without `--ids-only` retain their existing error behavior.

### Human recovery diagnostics

Missing credentials, malformed/rejected manual login, API 401, unavailable native
storage, and classified command usage errors may add human guidance on stderr.
Explicit `--json`, `--ndjson`, `--plain`, `--ids-only`, and `--quiet-json` suppress
these additions, retaining the existing machine output contract and exit codes.
`--no-input` does not suppress human advice; login examples use `--token-stdin`.
Uncertain review actions include read-only inspection guidance and require manual
reconciliation before a fresh review. No recovery command or persistence change is
introduced. See [errors and recovery](error-recovery.md) for the bounded inventory.

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
