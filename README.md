# todoist-cli

A terminal companion for Todoist: capture ideas and see what needs doing without leaving the terminal. Scripts and agents get stable structured output and explicit commands; see [agent skill installation](#agent-skills), [machine output](#output), and [planner integration](#agent-planner-integration).

## Why this CLI

- Fast capture and triage without leaving the terminal.
- Scriptable output (`--json`/`--plain`/`--ndjson`/`--ids-only`) for automation and integrations.
- Agent workflows for bulk plans with safe previews and confirmations.

See `docs/SPEC.md` for the CLI contract and `docs/ROADMAP.md` for planned features.

## Quickstart

```bash
brew install agisilaos/tap/todoist-cli
todoist --version
todoist auth login                 # prompts for token (or use --token-stdin)
todoist add "Review PR 42 today"
todoist inbox                     # see the captured task in Inbox
todoist review                    # guided review; confirm before changes
```

Choose what to view:

```bash
todoist inbox                           # Inbox, including undated tasks
todoist today                           # due today + overdue, across projects
todoist upcoming                        # today + next 6 days; excludes overdue
todoist task list --all-projects --all   # active tasks, including undated tasks
```

Bare `todoist task list` is **Inbox-only**, with one page by default.
`--all-projects` changes the scope; `--all` fetches every page. `inbox`, `today`,
and `upcoming` already fetch every page. Upcoming uses UTC dates.
If Inbox cannot be resolved, `inbox` and bare `task list` stop with an error
instead of returning tasks from other projects.

Human lists show titles with due information, project, priority, and a full copyable ID. See the [everyday overview](#everyday-overview) for examples and page coverage.

If a command fails, follow [errors and recovery](#errors-and-recovery) for credential setup and safe review inspection.

For a machine client with credentials configured:

```bash
todoist task list --all-projects --all --no-input --json
```

In a terminal, successful capture prints a receipt with the saved task content,
destination, due date, recurrence, priority, labels, and a command to view the task.
For example, `todoist add "Review report #Home tomorrow p2 @work"` shows the
returned Home project, absolute due date, and `Priority: 2`. Check these values
after capture; Todoist interprets the natural-language text.

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

Copy your personal API token from Todoist settings. Run the command below and paste only the token into the hidden prompt, without quotes or a `Bearer` prefix.

```bash
todoist auth login
```

Login checks the token with a read-only Todoist request before saving it. On success, run `todoist today`. A rejected token or connection failure leaves existing credentials unchanged; follow the error guidance and rerun login. Manual login (including `--print-env`) now requires API connectivity.

Other options:

```bash
todoist auth login --token-stdin < token.txt
TODOIST_TOKEN=... todoist task list
```

New profiles use macOS Keychain by default. Explicit portable file storage uses `~/.config/todoist/credentials.json` with `0600` permissions. Set `TODOIST_TOKEN` to override stored tokens. See [credential storage and recovery](#credential-storage-and-recovery) for other platforms and migration.

For read-only OAuth access:

```bash
todoist --profile reader auth login --oauth --read-only --client-id "$TODOIST_OAUTH_CLIENT_ID"
todoist --profile reader auth status --json
todoist --profile reader task list
```

Both `--oauth` (PKCE) and the configurable `--oauth-device` accept `--read-only`, requesting exactly `data:read`. OAuth without this flag requests `data:read_write,data:delete,project:delete`. Device flow requires an explicit provider endpoint; Todoist does not currently advertise device authorization. Mock-provider tests do not establish live Todoist support. `--timeout` bounds each OAuth HTTP request, not human approval: PKCE allows three minutes for its callback, and device authorization honors the provider’s code lifetime.

### OAuth onboarding

No OAuth client ID or client secret is bundled. `--client-id` overrides
`TODOIST_OAUTH_CLIENT_ID`. Use a public client configured for PKCE/S256 and
`token_endpoint_auth_method: none`, with a redirect URI accepted by Todoist.
An ordinary confidential App Console client cannot be assumed compatible with
this secretless CLI. A maintainer-owned public registration or publicly hosted
HTTPS client metadata document could supply future default onboarding; none is
currently configured for this project.

The callback defaults to `http://127.0.0.1:8765/callback`. Match its host, port,
and path to your registered redirect, or supply matching `--oauth-listen` and
`--oauth-redirect-uri` values. The listener is started before browser launch.
Use `--no-browser` to open the printed URL manually; authorization still needs
human approval and a callback to this machine. Cancellation before persistence, rejected exchanges,
and unacceptable grants leave the prior credential unchanged. Provider response
bodies and callback error text are never echoed.

Todoist's current docs describe public-client dynamic registration and HTTPS
client metadata documents, but allow localhost redirects specifically for testing.
This project's loopback registration and completed live exchange remain unverified.
Neither command registers an application automatically.

**Refresh support is a prerequisite for new short-lived grants.** Todoist says
new apps issue one-hour access tokens. This CLI rejects refresh-bearing responses
and finite lifetimes other than the documented legacy compatibility value
(`315360000` seconds), before saving or exporting a token. An omitted expiry is
retained for legacy compatibility. The error is `OAUTH_LIFECYCLE_UNSUPPORTED`;
use manual login or a verified legacy public client until durable refresh support
exists. Authorization metadata does not extend token validity.

Sources and the exact maintainer setup/live-verification requirements are in the
[profile and OAuth contract](docs/profile-oauth-design.md#oauth-prerequisites).

Read-only credentials can read Todoist, construct plans, and run previews. The CLI blocks mutations, including `agent apply/run`, before a mutation request is sent. `--force` and `--on-error=continue` do not override this restriction. External planners are trusted programs and are not sandboxed.

Authorization metadata is saved with each profile. `auth status` reports credential presence and authorization offline; `doctor` also performs a read-only API probe. Both distinguish requested/effective scopes, evidence, credential origin, source, and write capability without exposing tokens. Raw resource JSON and IDs-only output stay unchanged.

Existing stored tokens, manual tokens, and `TODOIST_TOKEN` remain **unknown** and may attempt writes for compatibility. The environment token overrides a selected profile without inheriting its metadata. Exporting with `--print-env` also loses local scope evidence on subsequent environment use. Invalid or unsupported metadata blocks authenticated operations and can be repaired through login/logout. Logout disables the profile and removes its token and metadata; failed native deletion is reported as pending cleanup. It does not revoke the token or unset the environment.

See the [authorization contract](docs/authorization-design.md), [command classification](docs/authorization-command-inventory.md), and [compatibility decision](docs/adr/0003-preserve-write-capability-for-unknown-credentials.md).

## Credential profiles

A credential profile is a named grant, not a Todoist account. Multiple profiles
can belong to the same account. Profile commands report stored metadata offline;
they do not retrieve native secrets, verify token validity, discover identity,
or expose token fragments.

```bash
todoist --profile work auth login
todoist --profile reader auth login --token-stdin < token.txt
todoist profile list --json
todoist profile use work
todoist profile current --json
todoist profile remove reader
```

For scratch or portable storage, retain the same `--config /path/to/config.json`
on every command and explicitly select `--credential-store=file` for new login.
Configuration files in the same directory share credentials. `list` includes
disabled cleanup records and safe per-row errors; invalid rows produce exit 3
after the report. JSON and NDJSON each emit one complete report, including an
empty `profiles: []`; human/plain output explains empty results.

Selection is `--profile` > `TODOIST_PROFILE` > current directory `.todoist.json`
`default_profile` > resolved user config `default_profile` > `default`. `use NAME`
requires an enabled profile with valid or legacy metadata, and saves only
`default_profile` in the user config selected by `--config`, `TODOIST_CONFIG`,
or the default path. It preserves other fields and never changes project config.
The acknowledgement identifies the saved choice and any overriding selection.
Native accessibility remains unchecked.

`TODOIST_TOKEN` overrides the credential regardless of profile selection.
`current` reports `source: env` and external/unknown authorization in that case;
the selected profile is inactive context and its stored metadata is not read.
Without the environment override, missing/disabled selection produces a report
and exit 4; invalid metadata fails closed. `auth status` keeps its existing
compatibility contract.

`remove NAME` targets the explicit name and succeeds idempotently for absent
names. Removing a selected/default profile **retains its selection**, so the
next stored invocation fails as missing until explicit `use` or login. It does
not select another credential or revoke the remote grant. Native removal first
durably disables the profile; cleanup failure returns exit 3 with
`CREDENTIAL_CLEANUP_PENDING` and `committed: true`. Repeat `profile remove NAME`
or run `todoist --profile NAME auth repair` to retry. Failure details provide
targeted recovery commands retaining the configuration. `TODOIST_TOKEN` remains
active until deliberately changed or unset.

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
  "credential_store": "native",
  "default_inbox_labels": ["inbox"],
  "default_inbox_due": "today",
  "table_width": 120,
  "planner_cmd": ""
}
```

Precedence (high → low):

1. Flags
2. Environment variables
3. Project config (`.todoist.json`)
4. User config (`~/.config/todoist/config.json`)

Automatically discovered `.todoist.json` files cannot override `base_url` or
`planner_cmd`; those fields are ignored there. Move existing project values to
your user config, select a trusted file explicitly with `--config` or
`TODOIST_CONFIG`, or use the corresponding explicit flags/environment variables.
Other project defaults retain their precedence.

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
- `TODOIST_PLANNER_CMD` (external planner command)

## Agent skills

Install maintained Todoist guidance for **Codex** or **Claude Code**. The skill
teaches command discovery, machine output, exact IDs, credential profiles,
authorization, reviewed plans, replay, and uncertain-write recovery. Its command
reference is generated from this CLI's authoritative help; curated workflows
describe the capabilities bundled with the running binary.

Start with the inventory and focused help; these commands need no credentials:

```bash
todoist skill list --json
todoist skill install --help
```

Every write requires one target, an explicit `--scope local|global`, and the full
absolute `--path` of its skill directory. Quote paths containing spaces.

| Target | Local project placement | Global user placement |
| --- | --- | --- |
| `codex` | `/path/project/.agents/skills/todoist-cli` | `/home/user/.agents/skills/todoist-cli` |
| `claude-code` | `/path/project/.claude/skills/todoist-cli` | `/home/user/.claude/skills/todoist-cli` |

Replace the sample project or home with your chosen absolute path:

```bash
todoist skill install codex --scope local --path "/work/My Project/.agents/skills/todoist-cli" --no-input --json
todoist skill list codex --scope local --path "/work/My Project/.agents/skills/todoist-cli" --json
todoist skill update codex --scope local --path "/work/My Project/.agents/skills/todoist-cli" --json
todoist skill uninstall codex --scope local --path "/work/My Project/.agents/skills/todoist-cli" --json

todoist skill install claude-code --scope global --path /home/user/.claude/skills/todoist-cli --json
```

`list` without `--path` checks conventional locations in the current directory
and user home. It does not search ancestor projects; use their explicit paths
when needed. Installation state does not establish agent loading. Start an agent
session in the selected project: check Codex's skill selector and invoke
`$todoist-cli`; in an interactive Claude Code session, check `/skills` and invoke
`/todoist-cli`. Start a fresh session if discovery has not refreshed. Agents can
scan each other's directories, so selecting a target does not promise exclusive
visibility. Custom agent homes, enterprise overrides, and cloud loading require
separate verification. See the [target conventions and contract](docs/agent-skill.md).

The installation manifest tracks owned files by content hash. Reinstalling or
updating an unchanged package is a no-op. An unmanaged existing skill is a
conflict. Update uses only this binary's bundle, with no network fetch, background
updates, or automatic changes when the binary is upgraded. Running an older
binary's explicit update can install an older bundle.

Modified or missing managed files block update and uninstall by default. To
deliberately replace edits, `skill update --backup` saves existing originals
outside the skills loading directory first. To end management while retaining
edits, `skill uninstall --keep-modified` removes unchanged owned files and the
manifest, and reports retained paths. A retained `SKILL.md` can remain
discoverable and may reference unchanged files that uninstall removed; inspect
or move retained instructions before continued use. Unrelated files and backups survive
uninstall. Shared agent instructions and shell profiles are untouched.

Use `--json` for a lifecycle result object or inventory array; `--ndjson` emits
one record per result or inventory item. Lifecycle errors go to stderr as JSON
in either mode, with stable `SKILL_*` codes and recovery details. List succeeds
when it produces an inventory, including entries marked `conflict` or `error`;
check every entry's `status`. Schemas are available as `skill_result` and
`skill_list`. No prompts are used. `--force` and `--dry-run` are rejected; use
`list` to inspect state. A recovery-required error means preserve its reported
paths and reconcile them before retrying; see [failure recovery](docs/agent-skill.md#failure-recovery).

## Usage

```
todoist [global flags] <command> [args]
```

Global flags can appear before or after commands; command-option values stay literal even when they look like flags. `--ids-only` is restricted to supported lists:

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
--ids-only           One raw ID per line (supported lists only)
--task-output-version <1|2>  Task JSON/NDJSON: legacy (1, default) or faithful (2)
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

### Command help and typo recovery

Start with `todoist --help` for the command overview and global flags. Group help
such as `todoist task --help` lists related commands; a leaf page describes just
one operation:

```bash
todoist task complete --help
todoist help task complete
todoist task help complete
todoist task show --help           # same page as task view
todoist agent schedule print --help
```

`-h` and `--help` work before or after the command path. Global flags retain their
normal placement rules. A flag value that happens to be `--help` stays literal;
for example, `--content "--help"` supplies task content. Help is available before
authentication, with broken configuration, and without required mutation
arguments. It does not run the command or write local files.

To complete a task, run `todoist task list`, copy its ID, and run
`todoist task complete id:123456`, replacing `123456` with that ID. Use
`todoist task complete --help` to discover bulk completion and preview options.

Misspellings such as `todoist todai` or `todoist task complet` return usage exit 2,
with nearby command suggestions and a pointer to the relevant help page. No
suggestion is executed. Unknown help targets also return exit 2. Suggestions
apply to command names, not task references or option values. Explicit machine
flags retain their existing error behavior without added suggestions; use
`--json --quiet-json` for compact JSON errors (`--quiet-json` alone does not
select JSON).

## Commands

### Auth

Manage Todoist credentials and profiles.

```
todoist auth login [--token-stdin] [--print-env] [--credential-store=native|file]
todoist auth login --oauth [--read-only] [--client-id <id>] [--no-browser] [--print-env]
todoist auth login --oauth-device --oauth-device-url <url> [--read-only] [--client-id <id>] [--print-env]
                  [--oauth-authorize-url <url>] [--oauth-token-url <url>]
                  [--oauth-device-url <url>] [--oauth-listen <host:port>] [--oauth-redirect-uri <uri>]
todoist auth status
todoist auth logout
todoist auth migrate --credential-store=native|file
todoist auth repair
```

- `auth login` prompts for a token without echoing it (TTY) or reads from stdin with `--token-stdin`. New profiles default to native storage (macOS Keychain); select `--credential-store=file` explicitly for portable plaintext storage. Existing profiles retain their backend. See [credential storage and recovery](#credential-storage-and-recovery).
- OAuth defaults use Todoist’s documented `https://app.todoist.com/oauth/authorize` and `https://api.todoist.com/oauth/access_token` endpoints. Endpoint override flags and environment variables remain available.
- `auth login --oauth` runs OAuth PKCE via local callback (`http://127.0.0.1:8765/callback` by default). If browser auto-open fails, the command prints a warning and continues waiting for callback so you can open the URL manually.
- `auth login --oauth-device` needs an explicitly configured device endpoint, then prints a verification URL/code and polls until authorized. Todoist does not advertise this flow; only mock-provider behavior is tested.
- `auth status` reports the selected profile, credential source, authorization mode, scope evidence, and write capability without contacting Todoist.
- `auth logout` disables the selected profile before deleting its token and authorization metadata. An environment token remains active.
- `auth migrate --credential-store=native|file` explicitly changes the selected profile’s backend after verifying the destination.
- `auth repair` reconciles interrupted credential transactions or retries pending cleanup.
- Use `--print-env` to emit `TODOIST_TOKEN=...` for piping into other tools (`--json`/`--ndjson` return structured output with the export string).

### Tasks

List and modify tasks (IDs or names accepted where noted).

```
todoist task list [--filter <query>] [--preset today|overdue|next7] [--project <id|name>] [--section <id|name>] [--label <name>] [--completed] [--completed-by completion|due] [--since <date>] [--until <date>] [--sort <key>] [--sort-order asc|desc] [--truncate-width <cols>] [--wide] [--all-projects]
todoist task add --content <text> [flags]
todoist task view <ref> [--full] [--include-children]
todoist task update <ref> [flags]
todoist task reschedule <ref> (--due-date <date> | --due-datetime <RFC3339> | --due-local-datetime <local>)
todoist task move <ref> [--project <id|name>] [--section <id|name>] [--parent <id>]
todoist task move --filter <query> [--project <id|name>] [--section <id|name>] [--parent <id>] --yes
todoist task complete <ref> [--forever]
todoist task complete --filter <query> --yes
todoist task reopen <ref>
todoist task delete <ref> --yes
```

Task flags:

By default, `todoist task list` shows one page of your Inbox tasks. Use `--all-projects` or a filter to list across projects, and `--all` to fetch every page. Human active-task output identifies the effective selection, displayed count, and page coverage, including empty results; `--quiet` suppresses the header and summary. Redirected and explicit machine output have no scope label.

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
--yes                      Required for task deletion and bulk move/complete
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
- `todoist task add --content "--json" --json` creates a task named `--json` and returns JSON.
- `task add/update --natural` lets you pass quick-add style tokens in `--content` (for example `#Home @errands p2 due:tomorrow`) and maps them to REST fields.
- Task references also support due hints for disambiguation: `"call mom today"`, `"call mom tomorrow"`, `"call mom overdue"`.

List presentation and selection options:

```
--wide    Detailed table (API priority numbering; broad terminal recommended)
--all-projects    List tasks from all projects (default is Inbox)
--preset today|overdue|next7    Shortcut filters (ignored if --filter set)
--sort due|deadline|priority|added|updated|completed|content|order|none
--sort-order asc|desc           Direction for an explicit sort across the fetched task selection
--truncate-width <cols>         Override human output width
```

Examples:

- `todoist task list --filter "@work & today"` (human overview)
- `todoist task list --all-projects --sort priority`
- `todoist task list --completed --since "2 weeks ago" --json`
- `echo "Write launch blog #Marketing @writing p2 due:friday" | todoist add --content -`
- `todoist task move --id 123 --project "Personal" --section "Errands"`
- `todoist task view id:123456 --full`
- `todoist task complete "Pay rent"`

##### Editing tasks and preserving recurrence

```bash
todoist task update id:123456 --clear-labels --clear-assignee --clear-deadline
printf 'Exact notes\n' | todoist task update --id 123456 --description -
todoist task move id:123456 --clear-parent --clear-section
todoist task reschedule id:123456 --due-date 2026-10-15
todoist task update --id 123456 --reference=true --order 0
todoist task view id:123456 --include-children --sort order --json --task-output-version 2
todoist task complete id:123456 --forever
```

Omitted fields remain unchanged. Empty description, `--reference=false`, and
`--order 0` are explicit edits. Use `--clear-due`, `--clear-deadline`,
`--clear-labels`, `--clear-assignee`, or `--clear-description` for removal.
Description stdin preserves UTF-8 text exactly; content stdin is trimmed. Only
one may read stdin. Setters and their clear flags conflict before API reads.

Ordinary due editing stays in `task update`; ordinary completion advances a
recurring occurrence. `task reschedule` preserves recurrence and the existing
calendar date, floating wall time, or fixed-zone character. A date target keeps
an existing clock; `--due-local-datetime` changes floating wall time;
`--due-datetime` selects an exact instant for a fixed-zone task. Unknown or
contradictory returned evidence and ambiguous DST date replacements refuse the
edit. `task complete --forever` permanently completes recurrence and subtasks.

Clearing a parent keeps the current inherited section, or the current project
when sectionless. Clearing a section keeps the current project; children in an
inherited section also require `--clear-parent`. Both clears make a project root.
A project with a section scopes section resolution; the section is the move
destination. Parent destinations exclude project/section setters.

`--reference[=true|false]` adds/removes one leading `* ` and refuses repeated
prefixes or a blank remaining title. `--order` accepts signed int32 values,
including zero. `--include-children` fetches every direct active child page and
emits a distinct `task_expanded_view`/`task_expanded_view_v2` envelope. Failed
expansion emits no parent or incomplete collection. Completeness means every
page was fetched, without a snapshot guarantee.

Task collections accept `--sort due|deadline|priority|added|updated|completed|content|order|none`
and `--sort-order asc|desc`. Omission keeps existing ordering. Due/deadline use
calendar comparisons; timestamps use instants; order uses numeric sibling order.
Default directions are ascending for due/deadline/content/order, descending for
priority/timestamps. Missing values stay last in either direction; ID breaks
ties ascending. Sorting covers the fetched selection; `--all` spans fetched
pages where supported. `none` retains fetched order and disallows direction.

Task writes dispatch once without automatic retries or redirects. Accepted
writes with unavailable optional task data emit `task_write_ack` with
`result_available:false`; inspect exact state before resubmitting. Combining
`--clear-due` with other fields uses Sync clear followed by REST update. A failed
second step emits `task_partial_edit` and a nonzero exit; no rollback or retry.
Filtered completion/move batches preflight all pages, require existing
confirmation, reject duplicate/ancestor overlaps, continue definite rejections,
and stop at uncertainty. `task_batch` reports every target and dispatch count.
See [task editing contracts](docs/task-editing-design.md), focused help, and
`todoist schema` for output variants and recovery.

## Task detail

Copy a full ID from an overview and inspect that exact task:

```bash
todoist task view id:102000002
todoist task view id:102000002 --full
```

In a terminal, detail shows the complete title and description, project and
section names, Todoist P1–P4 priority, current state, absolute due information,
recurrence, labels, and exact task ID. For the synthetic launch task in the
[everyday overview](#everyday-overview):

```text
P2  Prepare launch checklist and confirm the rollback owner with the platform
    team

Project: Work
Section: Launch
State: Active
Due: 2026-09-29T16:30:00+02:00
Timezone: Europe/Berlin
Recurrence: Yes (every Tuesday at 16:30)
Deadline: Not returned
Duration: Not returned
Assignee ID: Not returned
Due language: Not returned
Reference item: No (title syntax)
Labels: "work", "release"
ID: 102000002

Description:
  Confirm the release checklist with the platform team before the launch review.
```

This example abbreviates the synthetic description; actual detail prints every
paragraph. Text wraps at the configured human width without truncation,
preserving line breaks, spacing, and literal Markdown. IDs, timestamps, and URLs
stay intact and may overflow exceptionally narrow terminals. Other long words
can split; terminal controls are escaped. Quiet and accessible output retain the
same task information.

Detail also shows deadline, duration, assignee ID, due language, and reference-item
status derived from the returned title syntax. `--full` appends exact destination
IDs, timestamps, sibling/day order, order key, actor IDs, flags, and counters.
The returned `note_count` is deprecated and currently always zero; it does not
establish the task's actual comment count. P1 remains highest
in both human views. Dates and times retain their returned values, offset, and
timezone without conversion; differing returned calendar dates and timestamps
are shown separately. A floating time is labeled when its timezone is unknown.
Recurrence follows the explicit returned flag and includes its expression when
available. Expression-only due information is labeled as an expression.

`State` follows the explicitly returned completion flag. A completion timestamp
is shown independently; it does not establish current state or prove past
mutations, especially for reopened or recurring tasks. `None` means an explicitly
empty value; `No due date` means explicit null due; `Not returned` means the
response did not establish the value. Missing state or comment count is not
reported as active or zero.

Name resolution adds a paginated projects lookup and, only when the task has
project and section IDs, a sections lookup scoped to that project unless a
complete sections collection was already loaded in this command. Successful
lookups are cached within one command, including across an interactive selection;
names changed elsewhere while the prompt is open may remain as first fetched.
There is no persistent cache. Separate
overview and detail commands repeat requests. Pagination, configured timeouts,
and existing retries apply. A failed lookup keeps the task visible with its exact
ID and `(name unavailable; lookup failed)`; a successful lookup without a matching
name uses `(name unavailable)`. Enrichment failure still exits 0 when task lookup
succeeded.

For machine clients, use `todoist task view id:102000002 --no-input --json` or
`--ndjson`. JSON emits a single task object with API priority numbering; NDJSON
emits the same object as one newline-terminated JSON record on a single line.
`--full` does not change either payload. Redirected detail and explicit `--plain`
retain the legacy labeled text. These paths perform no name lookups.
`--ids-only` remains unsupported for task view. Unknown tasks retain exit 4 with
empty stdout and an error on stderr.

To inspect all supported returned task facts in machine output, select v2:

```bash
todoist task view id:102000002 --no-input --json --task-output-version 2
todoist task view id:102000002 --no-input --ndjson --task-output-version 2
todoist schema --name task_item_v2
```

V2 retains deadline, duration, `responsible_uid` (assignee), ordering, actor IDs,
flags, counters, and due recurrence/timezone/language. Absent fields stay omitted;
explicit null, false, zero, and empty values stay intact. It never substitutes
mutation inputs for missing returned facts. See the [task-data contract](docs/task-data-fidelity.md)
for field types, reference-item derivation, supported commands, and malformed-data recovery.

### Workspaces

```
todoist workspace list
todoist project collaborators <id|name>
```

- `workspace list` shows workspaces available to the authenticated user.
- `project collaborators` lists collaborators for a shared project.

### Filters

Saved filters use the [Todoist Sync API](https://developer.todoist.com/api/v1/). List/show read the filters resource; add/update/delete check the Sync command acknowledgement before reporting success.

```
todoist filter list
todoist filter show <id|name>
todoist filter add --name <name> --query <query> [--color <color>] [--favorite]
todoist filter update <id|name> [--name <name>] [--query <query>] [--color <color>] [--favorite|--unfavorite]
todoist filter delete <id|name> --yes
```

### Inbox

List all active Inbox tasks, or add to Inbox with optional defaults.

```
todoist inbox
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

#### Capture feedback and corrections

`add`, `add --strict`, `task add`, and `inbox add` show a capture receipt in a
terminal. It reports Todoist's returned state, rather than guessing from the input:

- `No due date` means Todoist explicitly returned no due date. `Not returned`
  means the response did not establish that value. Empty returned labels show `None`.
- Dates are absolute. Times retain the returned offset or timezone without conversion
  to the computer's timezone. A time without either is labeled accordingly. Recurrence
  is reported separately; its returned expression is included when available.
- Priority numbers match Todoist's `p1`–`p4`: `p2` displays as `Priority: 2`.
  Numeric `--priority 1`–`4` flags retain their API meaning (1 = normal, 4 = urgent);
  prefer `--priority p2` with `add` or `task add`. `inbox add` accepts numeric priorities only.
- Project and section names are resolved when available, with returned IDs as a fallback.
  Full content is retained on narrow terminals; control characters are escaped.
  Text labels work with `--accessible`, `--no-color`, and screen readers.

Use the exact view command from the receipt, keeping the same global options and
environment as capture (especially `--profile`, `--config`, and `--base-url` if used).
To correct a task, substitute its full returned ID below. Change destination with `task move`; `task update --project`
only scopes assignee lookup.

```bash
todoist task view id:123456
todoist task update --id 123456 --due "next Monday" --priority p2
todoist task move --id 123456 --project Home
```

Natural-language and structured capture use the same receipt. `--strict` leaves
content literal and uses explicit fields; a `--due` expression still needs Todoist's
interpretation. Human `--dry-run` shows the submitted text or proposed fields and
says no task was created. It cannot confirm Todoist's interpretation or the saved
result, and may perform reads to resolve names.

```bash
todoist add "Review report #Home tomorrow p2 @work" --dry-run
todoist add "Review report" --strict --project Home --due tomorrow --priority p2 --label work
todoist add "Review report #Home tomorrow p2 @work" --json
```

Receipts do not change machine output: JSON remains a one-task array, NDJSON a
single task object, and plain or piped output the existing TSV row with API priority
numbers. `--ids-only` still rejects creation. `--quiet` retains the existing task
table on a terminal and the existing dry-run summary, without the new receipt or
recovery hints. Active-task overviews and human task details use Todoist P1–P4;
detailed tables and machine output retain their existing API priority display.

#### Completion and move feedback

Single-task `complete` and `move` acknowledge successful operations in a terminal
with the full task ID. For example:

```bash
todoist task complete id:810000001
todoist task move id:810000002 --project Home --section Backlog
```

Illustrative output with synthetic tasks:

```text
Completion accepted
Task before completion: Send revised estimate
ID: 810000001

Move accepted
Task: Prepare launch checklist
ID: 810000002
Project: Home
Section: Backlog
```

Completion uses context obtained before the action. If that context explicitly
identifies a recurring task, it adds `Recurring task; next due date not returned.`
Completing a recurring task advances an occurrence rather than permanently
finishing the task. Missing recurrence stays unknown; no next date is invented.
`--id` avoids the task lookup, so completion may show only its ID.

Move destination IDs come from the successful response, with names from context
already loaded during resolution. Otherwise IDs show `(name unavailable)`.
Explicit empty/null section or parent information means `None` when relevant;
missing or malformed information stays unknown. A missing returned title can
use `Task before move`. When destination details are unavailable, submitted
values are labeled `Requested project`, `Requested section`, or `Requested parent
task`, followed by `Destination details unavailable.` Requested values do not
establish the resulting destination, especially with multiple destination flags.

Feedback adds no requests or readbacks. Unreadable, incomplete, oversized, or
malformed move-response details preserve the accepted move and exit 0; unavailable
details are not a reason to repeat the mutation. Failures before acknowledgement
retain existing errors and exits and may have an uncertain remote outcome.

```bash
todoist task complete id:810000001 --dry-run
todoist task move id:810000002 --project Home --section Backlog --dry-run
```

Human single-task previews show the known task, full ID, requested destinations,
existing authorization summary, and `no task changed.` They dispatch no mutation.
Full text and IDs are retained; terminal controls are escaped and lines wrap
naturally. Quiet, JSON, NDJSON, explicit plain and redirected output keep their
existing acknowledgements and previews. Bulk output remains unchanged.

### Today

Quick list of tasks due today and overdue across projects, including Inbox. Uses the selected credential profile in every output mode and reports an authentication error when no credential is available. Accepts global flags only; use `task list` for custom filters or limits.

Today fetches every matching page, requesting up to 200 tasks per page. The page
size does not limit the number of tasks shown.

```
todoist today
```

### Daily review

```bash
todoist review
todoist review --filter "overdue | today" --out review-plan.json
todoist review --dry-run
```

Review fetches all matching pages, freezes the task set, and orders tasks by due,
priority, then ID. For each task type `keep`, `change`, `complete`, or `skip`.
`change` opens a numbered field menu: choose a field, enter its value, repeat as
needed, then type `done`. You can edit existing task fields and move to a project,
section, or parent. A later move choice replaces the earlier destination. Empty
values leave fields unchanged; new field-clearing operations are not supported.

For example, keep one task; change another with `due-date`, `2026-10-01`, `done`;
complete a third; skip a fourth. Inspect the full original/proposed preview, then
type `no` to cancel or `yes` to apply. `cancel` works at any prompt. There is no
backtracking or session resume. Natural-language due values resolve only during
application; recurring due edits may change recurrence. Completing a recurring task
advances its occurrence; ordinary completion also completes subtasks.

Dry runs and read-only credentials stop at preview. `--out` saves the finished plan
without overwriting a file, even if you subsequently decline application. Otherwise,
a recovery plan is saved under the config directory before the first mutation.
Read-only users can hand the plan to a write-capable profile for the same account.
Plans contain private task data; keep them with the same care as task exports.

Application stops on the first failure. Changed/missing tasks block pending writes.
A task can be partially applied when its update succeeds but its move fails. For a
**definite rejection**, retry the unchanged saved plan using the same config/account:

```bash
todoist agent apply --plan review-plan.json --confirm "$(jq -r .confirm_token review-plan.json)"
```

Recorded successes are skipped; pending work is checked against the reviewed or
post-update state. For an **uncertain remote outcome**, inspect the task in Todoist
and reconcile the observed state before starting a fresh review; retrying the old plan is blocked.
Follow the [inspection steps](#errors-and-recovery), including completed-history
checks. Use the current binary,
never edit recovery plans, and run only one applying process at a time. Application
is not transactional and cannot guarantee exactly-once writes.

`--no-input` and piped stdin are rejected before API access; machine clients should
use `task list` and `agent plan/apply`. Interactive `review --json` or `--ndjson`
emits a final report on stdout with prompts on stderr. Reports account for every
selected task and distinguish kept, skipped, proposed, applied, failed, partially
applied, and unattempted work. See `todoist help review`, `todoist schema --name
review_report`, and the [review contract](docs/review-design.md).

### Completed

Shortcut for listing completed tasks.

```
todoist completed [--completed-by completion|due] [--since <date>] [--until <date>] [--project <id|name>] [--section <id|name>] [--filter <query>]
```

### Upcoming

List tasks due across projects during N days including today (default 7: today and the next 6 days, using UTC dates). Excludes overdue and undated tasks.

```
todoist upcoming [days] [--project <id|name>] [--label <name>] [--sort <key>] [--sort-order asc|desc] [--wide]
```

### Projects

Create and manage projects.

```
todoist project list [--archived]
todoist project view <id|name>
todoist project browse <id|name>
todoist project add --name <name> [--description <text>] [--parent <id|name>]
todoist project create --name <name> [--description <text>] [--parent <id|name>]
todoist project update --id <project_id> [flags]
todoist project move <id|name> (--to-workspace <id|name> | --to-personal) [--visibility restricted|team|public] [--yes]
todoist project archive --id <project_id>
todoist project unarchive --id <project_id>
todoist project delete --id <project_id>
```

Examples:

- `todoist project list --archived --json`
- `todoist project view "Home"`
- `todoist project browse "Home"`
- `todoist project add --name "Side Projects" --description "Weekend hacks"`
- `todoist project create --name "Side Projects" --description "Weekend hacks"`
- `todoist project update --id 234 --name "Side Projects (2024)"`
- `todoist project move "Team Project" --to-personal --yes`
- `todoist project move "Home" --to-workspace "Acme Corp" --visibility team --yes`

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

### Reminders

Manage task reminders (Todoist Sync API).

```
todoist reminder list (<task> | --task <ref>)
todoist reminder add (<task> | --task <ref>) (--before <duration> | --at <datetime>)
todoist reminder update (<id> | --id <id>) (--before <duration> | --at <datetime>)
todoist reminder delete (<id> | --id <id>) [--yes]
```

`--before` accepts positive integer minutes or whole-number duration terms such
as `1h30m` and `120s`. The total seconds round up to a whole minute; `120s` is two
minutes and `30s30s` is one. Negative, fractional, overflowing, or partially
recognized input is rejected.

### Notifications

Manage live notifications (Sync API).

```
todoist notification list [--type <types>] [--unread|--read] [--limit <n>] [--offset <n>]
todoist notification view [id] [--id <id>]
todoist notification accept [id] [--id <id>]
todoist notification reject [id] [--id <id>]
todoist notification read [id] [--id <id>] [--all --yes]
todoist notification unread [id] [--id <id>]
```

`accept`/`reject` work for `share_invitation_sent` notifications.

### Activity

View account/project activity logs.

```
todoist activity [--since <date>] [--until <date>] [--type task|comment|project] [--event <type>] [--project <id|name>] [--by <id|me>] [--limit <n>] [--cursor <cursor>] [--all]
```

### Stats

View productivity stats and completion progress.

```
todoist stats
todoist stats goals [--daily <n>] [--weekly <n>]
todoist stats vacation (--on | --off)
```

### Settings

Manage account settings and notification preferences.

```
todoist settings
todoist settings view
todoist settings update [flags]
todoist settings themes
```

`settings view` prints human-friendly labels (for example `24h`, `DD-MM-YYYY`, theme names) and resolves `start_page` references like `project?id=<id>` to names when possible.

Settings, goals/vacation and notification changes require a successful Sync acknowledgement for every command. A failed invitation command stops before marking it read. Inspect remote state before retrying a partially accepted or unacknowledged update.

### View

Open Todoist web URLs with equivalent CLI commands.

```
todoist view <url>
```

Examples:

- `todoist view https://app.todoist.com/app/task/call-mom-6f3qg8mgqp99mFVJ`
- `todoist view https://app.todoist.com/app/project/home-2203306141`
- `todoist view https://app.todoist.com/app/settings`

For project URLs with deprecated legacy IDs, `view` attempts a slug/name fallback before using the URL ID directly.

### Agent

Integrate with an external planner to generate and apply bulk plans.

```
todoist agent plan <instruction> [--out <file>] [--planner <cmd>]
todoist agent apply <instruction> --confirm <token> [--planner <cmd>] [--policy <file>]
todoist agent apply --plan <file> --confirm <token>
todoist agent apply --plan <file> --confirm <token> --dry-run [--policy <file>]
todoist agent apply --plan <file> --confirm <token> --on-error fail|continue
todoist agent run --instruction <text> [--planner <cmd>] [--confirm <token>|--force] [--policy <file>]
todoist agent schedule print --weekly "sat 09:00" [--instruction <text>] [--planner <cmd>] [--confirm <token>|--force] [--policy <file>] [--dry-run] [--cron]
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
- `--on-error=continue` keeps applying after an individual action failure and reports statuses. Individual non-task request timeouts count as action failures. Uncertain task writes (including request timeouts) stop application under [ADR-0009](docs/adr/0009-dispatch-task-writes-once-and-retain-pending-evidence.md). Authorization denial, caller cancellation or an expired operation deadline, and replay-store load or record failures always stop application.
- Human apply/run output includes a summary block (ok/failed/skipped replay), destructive-action count, per-action-type counts, and final outcome.
- `--plan-version` enforces expected plan.version (default 1). Unknown versions are rejected.
- `agent planner` shows/sets the planner command (uses config/planner_cmd or TODOIST_PLANNER_CMD).
- Setting the planner changes only `planner_cmd` in the user configuration, preserving unrelated and unknown fields without saving project or environment overrides.
- `agent run` combines plan + apply for automation (cron/launchd).
- `agent schedule print` emits a scheduler entry (launchd by default; use `--cron`). It preserves `--dry-run` and `--force` plus profile/configuration selections; authorization is resolved when the generated command runs. Shell/XML metacharacters are escaped. Cron output escapes `%` and rejects arguments containing line breaks.
- Context flags: `--context-project`, `--context-label`, `--context-completed 7d` limit planner context.
- Planner context now includes active tasks (capped) in addition to projects/sections/labels/completed tasks.
- `--policy <file>` enforces action-policy rules (`allow_action_types`, `deny_action_types`, `max_destructive_actions`).
- `--progress-jsonl[=path]` emits JSONL progress events for `agent run/apply` (stderr by default). If the requested log file cannot be opened, the command fails before dispatching actions.
  Key lifecycle events include `agent_plan_loaded`, `agent_action_validated`, `agent_action_dispatched`,
  `agent_action_succeeded`/`agent_action_failed`, and `agent_apply_summary`.
- Agent apply/run keeps a replay journal (`agent_replay.json`) and skips already-applied actions from the same plan token. An action is reported as successful only after its Todoist mutation and replay record both succeed.
- Each successful Todoist mutation replaces the replay journal before success is emitted. For ordinary plans, skipped and failed actions do not write it. Review plans also persist pending evidence before execution and clear it after definite rejection; replay skips still do not write. Recording cost grows with the journal, which is intentionally not pruned because replay keys have no safe expiry policy.
- For ordinary plans, interruption after Todoist accepts a mutation but before its replay record is installed can leave that mutation unrecorded, so rerunning may duplicate it. Review plans retain pending evidence and block retry of that plan. Inspect Todoist and reconcile the outcome before starting a fresh review.
- Replay recording assumes one applying CLI process at a time. Same-directory replacement avoids writing partially encoded JSON into the journal, but replacement visibility follows the underlying OS and filesystem; it does not coordinate concurrent writers or promise survival from an OS or storage power loss.

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

Output JSON schemas and wire-format descriptors (always emitted as JSON):

```
todoist schema [--name task_list|task_item_ndjson|profile_list|profile_current|profile_use|profile_remove|ids_only|error|plan|plan_preview|planner_request] [--json]
```

Task schemas: `task_item` and `task_item_ndjson` describe legacy objects;
`task_list` describes legacy arrays. With `--task-output-version 2`, use
`task_item_v2` for ordinary JSON views and NDJSON records, and `task_list_v2` for JSON arrays. Expanded views use `task_expanded_view` or `task_expanded_view_v2`; add/update/reschedule result unions also admit `task_write_ack` when optional returned task data is unavailable.

## Shell Completions

Shell completions suggest command names and aliases, including `notification ls`.
Flag and value completion remain shell-specific. Regenerate
or reinstall completion scripts after upgrading to pick up inventory changes.

Generate a completion script for your shell:

```bash
todoist completion bash > /usr/local/etc/bash_completion.d/todoist
todoist completion zsh  > "${fpath[1]}/_todoist"
todoist completion fish > ~/.config/fish/completions/todoist.fish
todoist completion powershell > ~/.local/share/todoist/completions/todoist.ps1

# Or install to a sensible default location:
todoist completion install bash
todoist completion install powershell  # "pwsh" is an identical alias

# Auto-detect your shell from $SHELL:
todoist completion install

# Remove installed scripts:
todoist completion uninstall
todoist completion uninstall zsh
todoist completion uninstall powershell
```

`completion install` prints a shell-specific activation command with quoted paths, including paths containing spaces or shell metacharacters. Bash and fish use `source`; PowerShell uses `. '<path>'`. For zsh, the command initializes `compinit` and registers a completion function that loads the installed file when Tab is pressed, including custom `--path` filenames. Run the printed command to activate completion now; add it to `.zshrc` for future zsh sessions. Do not source the zsh completion file directly.

PowerShell completion supports PowerShell 7 on macOS and Linux. Its default path is `$XDG_DATA_HOME/todoist/completions/todoist.ps1`, falling back to `~/.local/share/todoist/completions/todoist.ps1`. Installation never edits `$PROFILE`; run the printed dot-source command for the current session and add that command to your chosen `$PROFILE` for future sessions.

When no shell is given, installation checks for an active PowerShell environment before falling back to `$SHELL`. If neither can be identified, pass the shell explicitly. Use `--path <file>` to override any default installation or removal path.

## Finding IDs

Some operations require IDs (for example, project archive/delete and comment update/delete). Use list commands in `--plain` or `--json` mode to locate IDs:

```bash
todoist task list --filter "content:\"Write launch blog\"" --plain
todoist project list --plain
todoist comment list --task <task_id> --plain
todoist task list --completed --since "yesterday" --json | jq -r '.[].id'
```

Where supported, name resolution is built-in (e.g., `--project <name>` and `--section <name>` on task commands, `--label <name>`), and task update/complete/delete accept IDs or text references. Use `id:<id>` to explicitly reference IDs.

Task/project/label/filter references also accept Todoist app URLs, for example:

```bash
todoist task view https://app.todoist.com/app/task/call-mom-abc123
todoist project rm --id https://app.todoist.com/app/project/home-2203306141 --dry-run
todoist filter show https://app.todoist.com/app/filter/today-f1
```

### Ambiguous task references

When a task reference matches several tasks, machine clients receive usage exit
`2`. The existing JSON ambiguity error remains on stderr with `details` containing
`type`, `entity`, `input`, and a title-only `matches` array. It contains no
candidate IDs or task context, so duplicate titles cannot support an exact-ID
retry from that error alone. Plain and NDJSON modes retain their existing text
errors.

Use the existing list command to inspect every active task across projects:

```sh
todoist task list --all-projects --all --no-input --json
```

Compare `content`, `project_id`, `due`, and any relevant `section_id`, `parent_id`,
`labels`, or `description`. The returned `id` is the full, opaque task ID. Resolve
project or section names with their existing list commands if needed. Inspect the
chosen task before retrying the intended mutation with its exact ID:

```sh
todoist task view id:abc123XYZ --full --no-input --json
todoist task complete id:abc123XYZ --no-input --json
```

Replace `abc123XYZ` with the deliberately chosen ID from your output, and retain
the same credential, configuration, profile, and API endpoint. Candidate order
or fuzzy rank alone does not establish the intended mutation target. This
recovery uses existing output contracts; adding structured ambiguity IDs or
context would require a separate compatibility decision.

## Prompts & Safety

- Ambiguous task references offer the existing numbered choice when stdin is a
  terminal. Each candidate shows its title, full ID, project, and returned concrete
  due date/time, `No due date`, or `Due unavailable`. A due expression without a
  resolved date is labeled `(date unavailable)`. Section, parent, and labels
  appear when available; `Repeats` appears only when Todoist returned a recurring
  due date. Name lookup is best-effort and falls back to the corresponding ID;
  unavailable context does not prevent a choice. Times retain the returned
  offset/timezone without local conversion.
- Choose a number explicitly. Pressing Enter cancels selection and leaves the
  ambiguity error (exit `2`); an invalid number is also a usage error. `--no-input`
  or non-terminal stdin skips the choice and returns ambiguity. Exact `id:<id>`
  resolution is unchanged. Ranking orders candidates; it never automatically
  selects a mutation target from multiple matches.
- Project archive/delete and section/label/comment delete prompt when stdin is a TTY; use `--force` to skip those prompts, including with `--no-input`. Task deletion always requires `--yes`, even with `--force` or `--dry-run`. Filter deletion requires `--yes` or `--force`.
- `--dry-run` previews the actions that would be sent to Todoist without performing them.
- `--no-input` disables all prompts (auth included). Provide required flags or env vars to continue.
- Prompts require terminal input; automatic human output requires terminal stdout.
  Null devices (such as `/dev/null`), pipes, and regular files are noninteractive.

## Output

- TTY active-task lists use the [everyday overview](#everyday-overview). Other resource lists, completed history, and saved `filter show` retain their tables. Task creation uses the [capture receipt](#capture-feedback-and-corrections), except with `--quiet`.
- Non-TTY defaults to `--plain` (tab-separated, no headers), with the existing
  [task-detail exceptions](#task-detail).
- `--json` ordinarily outputs raw resource arrays/objects. Expanded task views and accepted-write fallbacks use the separate contracts described in [task editing](#editing-tasks-and-preserving-recurrence). JSON and NDJSON lists report remaining pages on stderr with `--cursor` or `--offset` continuation hints; use `--all` where supported to fetch every page.
- `--ndjson` outputs one JSON object per line for resource lists. Single-task views, mutation acknowledgements, dry runs, auth results, doctor reports, and agent/planner results emit one record with the same payload as `--json`. Completion script generation still emits shell source.
- `--task-output-version 2` selects the [faithful task-resource contract](docs/task-data-fidelity.md)
  with `--json` or `--ndjson`. Omission defaults to legacy version 1. Explicit
  selection is restricted to returned-task commands and rejects dry runs and
  acknowledgements before side effects. There is no configuration/environment override.
- `--ids-only` outputs one raw ID followed by a newline per result. Empty results emit no stdout.
- Errors go to stderr; `--quiet` suppresses non-essential informational messages but retains list continuation notices. `--verbose` may show request IDs and more detail.
- Color is enabled by default on TTY; use `--no-color` or `NO_COLOR=1` to disable.
- `--accessible` (or `TODOIST_ACCESSIBLE=1`) retains the existing due/priority markers in task tables and plain output. Active-task overviews always have explicit text labels, with or without this flag.
- Human width uses `--truncate-width` where supported, then `TODOIST_TABLE_WIDTH` or configured `table_width`, then `COLUMNS`, then 120 columns. Existing `--wide` selects the detailed table for active lists and expands other task tables; it is intended for a broad terminal.
- Fuzzy name resolution can be enabled with `--fuzzy` or `TODOIST_FUZZY=1` (project/section/label names); `--no-fuzzy` disables.
- Explicit filter IDs and URLs never match a different filter's name.
- Assignee email matches take precedence over display names; duplicate display names require disambiguation.
- Filter names follow the same opt-in fuzzy matching rule, including the `--no-fuzzy` override.

### Everyday overview

Start with `todoist today` for overdue and due-today work across projects.
`inbox` and bare `task list` remain Inbox selections; the all-project view also
includes undated tasks. The layout never groups or reorders tasks: existing API
ordering and supported client sorting remain in effect. Filters and presets keep
API ordering (`--sort` does not apply to them); `--preset today` selects only
`today`, while the top-level `today` command selects `overdue | today`.

Illustrative human output from `todoist task list --all-projects --all`, using
synthetic tasks and a UTC reference date of 2026-09-29:

```text
All projects · Active tasks · 4 shown · All pages fetched
Due labels: 2026-09-29 (UTC) · 1 overdue · 1 due today

P1  Send revised estimate
    Overdue · 2026-09-28 · Work · ID 101000001

P2  Prepare launch checklist and confirm the rollback owner with the platform
    team
    Today · 2026-09-29 · Repeats · Work · ID 102000002

P4  Book dentist appointment
    No due date · Inbox · ID 103000003

P3  Pick up prescription
    Due 2026-09-30 · Personal · ID 104000004
```

- Titles wrap to at most three lines, ending with `…` if truncated. Headers and
  context wrap to the selected width. Full IDs and timestamps are never shortened;
  indivisible values can overflow exceptionally narrow terminals. Common CJK
  characters occupy two cells; ambiguous-width characters assume one cell.
- Priority is Todoist **P1 highest, P4 normal**, also used by human task detail.
  Numeric input flags, plain/JSON/NDJSON output, and the retained tables keep API
  priorities (4 highest).
- Relative due labels use the printed **UTC calendar date**, with the existing
  returned due-date precedence. Earlier times today still say `Today`. Returned
  dates/times and timezone information remain visible without conversion. Near
  midnight, these display labels can differ from Todoist's account-based Today
  selection; they do not change which tasks are fetched. `Repeats` appears only
  when Todoist explicitly returns recurrence. Missing due data says `Due unavailable`;
  only an explicit null due value says `No due date`.
- Counts describe tasks **shown**, including the overdue/due-today counts. A partial
  page says `More available`, with `More available. Use --cursor "..."` on stderr.
  Repeat the same `task list` selection with that cursor, or use `--all` to fetch
  every page. Continuations are labeled even at the end; they never claim earlier
  pages were shown. Empty pages with more results are distinguished from empty
  selections. `--quiet` hides headers, counts, and empty messages, but keeps tasks
  and necessary continuation notices.
- Use `todoist task view id:101000001` with an ID from your output for the full title,
  description, named context, dates, and labels; add `--full` for destination IDs
  and inspection metadata. Existing `task list --wide` and
  `upcoming --wide` show the detailed table with section names and labels, using
  the table's existing API priority numbering. `today` and `inbox` accept no new
  display flags.
- Project names reuse the existing cached collection lookup. A lookup failure
  leaves tasks visible with a project ID and `(name unavailable)`; there are no
  per-task enrichment requests. The default overview does not fetch sections.
  Resolving an Inbox selection still must succeed before any tasks are fetched.

Redirecting these commands still produces the existing untruncated, headerless
TSV. Explicit `--plain`, `--json`, `--ndjson`, and `--ids-only` preserve their
payloads, fetching behavior, and stdout/stderr contracts. No overview labels or
empty messages are added to machine output.

Plain output columns:

- `task list`: `id, content, project_id, section_id, labels, due, priority, completed`
- `project list`: `id, name, parent_id, is_archived, is_shared`
- `section list`: `id, name, project_id, is_archived`
- `label list`: `id, name, color, is_favorite`
- `comment list`: `id, content, posted_at`

### IDs for machine clients

`--ids-only` extends the machine output contract. It supports `task list`, `project list`,
`project collaborators`, `section list`, `label list`, `comment list`, `filter list`,
`workspace list`, `reminder list`, `notification list`, and `activity`, plus existing
`ls` aliases and the task-list shortcuts `completed`, `today`, `upcoming`, bare
`inbox`, and `filter show`. Collaborators emit user IDs; activity emits event IDs,
not related-object IDs. Other commands reject the flag before side effects,
including mutations, single-object views, resources without stable IDs, and `view URL`.

```sh
todoist --ids-only task list --all-projects
todoist project list --all --ids-only
todoist task list --ids-only > task-ids.txt
```

IDs are opaque and unquoted, without headings or metadata. Result order and duplicates
are preserved, including the command's existing sorting. Missing IDs or IDs containing
whitespace/control characters fail with exit `1` before any IDs are emitted.

Fetching defaults and `--all` behavior are unchanged. If another page remains,
stderr receives `More available. Use --cursor "<cursor>"` (quoted with escapes),
or `More available. Use --offset N` for notifications. Pagination never enters stdout,
even for an empty page. Exhausted results emit no continuation notice.

`--ids-only`, `--json`, `--plain`, and `--ndjson` are mutually exclusive.
Conflicts and unsupported uses of `--ids-only` return usage exit `2`, empty stdout,
and the existing JSON error envelope on stderr. ID-mode runtime errors also use
that envelope and retain existing exit codes; `--quiet-json` makes errors compact.
Existing output modes and their error behavior are unchanged.

The flag follows existing global parsing: it works before or after the command,
uses the exact `--ids-only` spelling (not `--ids-only=true`), and is not consumed
after `--`. Version takes precedence over output conflicts; conflicts precede help.
Explicit help and implicit root/command help may print normal help with this flag.
`todoist schema --name ids_only` describes the wire format; it is not a JSON payload
schema, and existing JSON schemas remain unchanged.

## Errors and recovery

Human errors include next steps for missing credentials, rejected manual tokens or
API 401 responses, unavailable requested credential storage, and command usage
errors. Explicit machine flags retain their existing payloads, messages, streams,
and exit codes; `--no-input` alone still permits human diagnostics.

`task delete` (also `task rm` / `task del`) checks for a nonblank ID or reference
before looking up credentials. A missing target exits 2 and human output gives
a short example and `task delete --help` pointer. Supply `--yes` with the target,
including for a dry run.

Retain your original `--config`, `--profile`, and `--base-url` selections when
following examples. `TODOIST_TOKEN` overrides stored credentials; logging in does
not replace it. Replace that environment value deliberately, or unset it when
switching back to a stored credential. Inspect selection offline with:

```bash
todoist auth status --no-input
```

Status does not verify token validity. For noninteractive login, provide a secret
file containing only the token:

```bash
todoist auth login --no-input --token-stdin < token.txt
```

Protect `token.txt` as a secret. For a **new or file-backed profile** requiring
portable plaintext storage, add `--credential-store=file`. An existing native
profile needs a supported native build; the login flag does not migrate it and
migration requires access to the original credential. See [credential storage
and recovery](#credential-storage-and-recovery). Rejected manual login leaves
stored credentials unchanged. An API 401 during a multi-action command does not
undo earlier actions; inspect the reported results before resubmitting mutations.

For invalid input, follow the displayed command-specific `--help` reference.
Calling `task view` without a target shows a short error and an exact-ID example;
replace `<id>` with the full task ID from a task list.
Usage errors alone do not establish that an earlier action was undone.

After an interrupted review with an **uncertain remote outcome**, a Todoist
mutation may already have happened. Keep the saved review plan and replay evidence
(`agent_replay.json` beside the selected configuration). Do not reapply the old
plan, delete evidence, or force resubmission. Using the same credential source,
configuration, account, and API endpoint, inspect each uncertain task's exact ID:

```bash
todoist task view id:123456 --full --no-input
```

Replace `123456` with the ID from the report or saved plan. Compare current fields
with the proposed changes and reported applied actions. If a task is missing, or
a recurring task's date advanced, check that exact task in Todoist's completed
history/activity; a single task lookup cannot prove whether completion happened.
Only after reconciling intended and observed state manually should you start a
fresh review for remaining changes. If you cannot establish the result, stop and
verify in Todoist. There is no automated reconciliation command. `--no-input`
can inspect tasks but cannot start an interactive review.

See the [diagnostic boundaries and failure inventory](docs/error-recovery.md).

## Exit Codes

- `0` success
- `1` generic failure
- `2` invalid usage
- `3` auth error
- `4` not found
- `5` conflict
- Errors return human-readable messages; `--json` and `--ids-only` errors include `{"error": "...", "meta": {"request_id": "..."}}`.

## Agent Planner Integration

### Why agents

- Batch changes with human review via plan/apply.
- Safer automation with `--dry-run`, `--confirm`, and `--on-error`.
- Easy scheduling without running a daemon.

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

## Test the capture/review workflow

From a clean checkout on macOS or Linux with Go, Python 3, and Make installed:

```bash
make capture-review-check
# Or test an already built binary:
make capture-review-check BINARY=/absolute/path/to/todoist
```

This runs capture, a dry run, cancelled and confirmed review, and saved-plan replay
against a temporary local API fixture using a real terminal (PTY). It verifies
capture receipt fields, saved state, and mutation counts: replay must send no
second mutation. No Todoist credentials or real tasks are used. It also checks
noninteractive refusal and deliberately broken fixtures. This proves the local
workflow contract, not live Todoist compatibility. See the
[contributor checks](CONTRIBUTING.md#capture-review-and-replay) for diagnostics and
coverage boundaries. The same check runs through `make check` in Linux/macOS CI.

## Release

Run `make verify` (the `make check` alias) while developing. Release settings are
kept in `scripts/release-config.sh`; copied helpers are pinned and checked locally.
See [release recovery](docs/release-recovery.md) after an interrupted publication.

Ask an agent to prepare the changelog from commit and PR evidence, review and commit it, then run:

```bash
make changelog-context VERSION=vX.Y.Z
make release-check VERSION=vX.Y.Z
make release-dry-run VERSION=vX.Y.Z
make release VERSION=vX.Y.Z
```

Every new changelog bullet links to its pull request or direct commit. The approved changelog section becomes the GitHub Release notes. The dry run builds both macOS archives and checksums and renders the Homebrew formula without remote writes.

See `RELEASING.md` for the full runbook. Release scripts are `scripts/changelog-context.sh`, `scripts/release-check.sh`, and `scripts/release.sh`.

## Docs

- Documentation index and maintenance checks: [docs/README.md](docs/README.md)
- CLI specification: [docs/SPEC.md](docs/SPEC.md)
- Roadmap: `docs/ROADMAP.md`
- Release runbook: `RELEASING.md`
- Release history: `CHANGELOG.md`

## Limits (from Todoist docs)

- POST body size limit: 1 MiB
- Header size limit: 65 KiB
- Processing timeout: 15 seconds for standard requests

## Notes

- This CLI uses Todoist REST API v1 endpoints under `https://api.todoist.com/api/v1`.
- Native credential storage currently supports macOS Keychain. Other platforms use explicitly selected file storage; see [credential storage and recovery](#credential-storage-and-recovery).
- Some Todoist surfaces (for example skill/update) are not implemented yet.
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

### Credential storage and recovery

New profiles require native storage by default. macOS releases use Keychain; Linux,
Windows, and macOS builds without cgo require explicit file storage for saved login:

```bash
todoist auth login --credential-store=file --token-stdin < token.txt
todoist --profile work auth migrate --credential-store=native
todoist --profile work auth repair
```

`--credential-store=native|file` selects storage for new profiles. Set
`"credential_store": "file"` in the user configuration to choose the portable
fallback by default; project `.todoist.json` cannot select credential storage.
Existing profiles retain their backend, including existing plaintext profiles.
A conflicting login selection requires explicit migration first. There is no
silent fallback when native storage is unavailable, locked, or denied.

Native tokens reside in Keychain. Non-secret authorization metadata and native
references remain in `credentials.json` with `0600` permissions. `auth status`
reads metadata without retrieving native secrets; its `configured` field does
not verify accessibility or authentication. `doctor` reports backend health and
attempts its existing read-only API probe. Keychain never opens an OS dialog;
unlock or adjust access outside the CLI and retry. `TODOIST_TOKEN` still bypasses
stored credentials and their metadata.

Migration verifies the destination before selecting it and removing the current
plaintext token. It preserves authorization evidence and other profiles. Reads
never migrate automatically. Interrupted operations require `auth repair`;
cleanup failures report whether the switch completed. Logout disables the profile
before attempting native deletion, and repeating logout retries pending cleanup.

Older binaries cannot use migrated profiles. To downgrade, first explicitly run
`todoist auth migrate --credential-store=file`. No compatibility token copy is
retained. Avoid editing migrated files with older binaries, which may discard
storage references. File removal is not forensic erasure of backups or snapshots.

Profiles are isolated by canonical configuration directory and name. Symlink
aliases share a directory; moving or copying it requires a new login and cannot
delete entries in its original namespace. See the [storage design](docs/credential-store-design.md)
and [security policy](SECURITY.md) for recovery and native-test boundaries.
