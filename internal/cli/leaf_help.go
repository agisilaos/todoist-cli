package cli

// Detailed help is separate from discovery metadata; flags are checked against
// parser registrations by TestLeafHelpFlagsMatchRegistrations.

const taskSortFlags = `  --sort <key>                     due|deadline|priority|added|updated|completed|content|order|none
  --sort-order <asc|desc>           Direction for an explicit sort
`
const taskSortNotes = `  Sorts the fetched selection; --all spans every fetched page where supported.
  Defaults: due/deadline/content/order asc; priority/added/updated/completed desc.
  Missing values go last in both directions; equal values use opaque ID ascending.
  --sort none retains fetched order and rejects --sort-order; omission keeps default ordering.
`

var leafHelpPages = map[string]commandHelp{
	"skill install": {
		usage: `<codex|claude-code> --scope <local|global> --path <absolute-directory>`,
		flags: `  --scope <local|global>             Required placement: project or user
  --path <absolute-directory>       Required full skill directory`,
		examples: `  todoist skill list --json
  todoist skill install codex --scope local --path /work/project/.agents/skills/todoist-cli --no-input --json
  todoist skill install claude-code --scope global --path /home/user/.claude/skills/todoist-cli --json`,
		notes: `  Installs only the running binary's bundle; repeated identical installation is unchanged.
  An existing unmanaged file is a conflict; install never adopts or overwrites it.
  Path must be absolute and end in the selected target's skills/todoist-cli layout.
  Local paths belong to a project; global paths belong to the selected user's home.
  Named placement does not guarantee exclusive visibility: agents may scan other agents' directories.
  Codex: start in the project, check its skill selector, and invoke $todoist-cli.
  Claude Code: start an interactive project session, check /skills, and invoke /todoist-cli.
  Start a fresh agent session if discovery has not refreshed. Custom homes and cloud agents need separate verification.
  No credentials or prompts. --force and --dry-run are unsupported; inspect with skill list.`,
		globals: `  --json, --ndjson       Structured lifecycle output
  --no-input            Noninteractive operation`,
	},
	"skill list": {
		usage: `[codex|claude-code] [--scope <local|global>] [--path <absolute-directory>]`,
		flags: `  --scope <local|global>             Restrict placement
  --path <absolute-directory>       Inspect one path; target and scope required`,
		examples: `  todoist skill list --json
  todoist skill list codex --scope local --path /work/project/.agents/skills/todoist-cli --ndjson`,
		notes: `  Reports conventional current-directory and user-home locations without installing anything.
  Status is absent, installed, outdated, modified, conflict, or error; inspect each row's status.
  List succeeds when inventory is produced, including per-location conflicts/errors.
  Filesystem state does not prove that the agent loaded the skill. For an ancestor project or custom home, inspect its explicit path.`,
		globals: `  --json, --ndjson       Array or one installation per line
  --no-input            Noninteractive operation`,
	},
	"skill update": {
		usage: `<codex|claude-code> --scope <local|global> --path <absolute-directory> [--backup]`,
		flags: `  --scope <local|global>             Required placement
  --path <absolute-directory>       Required managed skill directory
  --backup                         Preserve modified originals before replacement`,
		examples: `  todoist skill update codex --scope local --path /work/project/.agents/skills/todoist-cli --json
  todoist skill update codex --scope local --path /work/project/.agents/skills/todoist-cli --backup --json`,
		notes: `  Updates one owned installation from this binary. No network fetch or background update.
  Modified or missing owned files block update by default; unrelated files are preserved.
  --backup explicitly preserves modified originals outside the skills loading directory before replacement.
  Keep customization in separate, unrelated files when possible; older binaries can install older bundles explicitly.
  If failure reports recovery_required, preserve reported paths and reconcile them before retrying.
  --force does not bypass ownership or customization protection.`,
		globals: `  --json, --ndjson       Structured result; JSON errors on stderr
  --no-input            Noninteractive operation`,
	},
	"skill uninstall": {
		usage: `<codex|claude-code> --scope <local|global> --path <absolute-directory> [--keep-modified]`,
		flags: `  --scope <local|global>             Required placement
  --path <absolute-directory>       Required managed skill directory
  --keep-modified                  Retain edits while removing ownership`,
		examples: `  todoist skill uninstall codex --scope local --path /work/project/.agents/skills/todoist-cli --json
  todoist skill uninstall codex --scope local --path /work/project/.agents/skills/todoist-cli --keep-modified --json`,
		notes: `  Removes unchanged owned files and management metadata; repeated absent uninstall is unchanged.
  Modified or missing files block removal by default. --keep-modified explicitly retains edits and ends management.
  Unrelated files and saved backups are preserved. Retained SKILL.md may remain discoverable by the agent.
  Retained instructions may reference removed unchanged files; inspect or move them before continued use.
  An unowned directory is never recursively deleted. Agent instructions and shell profiles are untouched.`,
		globals: `  --json, --ndjson       Structured result; JSON errors on stderr
  --no-input            Noninteractive operation`,
	},
	"profile list": {
		usage:    ``,
		examples: `  todoist profile list --json`,
		notes: `  Enumerates credential profiles offline, including disabled profiles with pending cleanup.
  Reads metadata without retrieving native secrets or checking token validity. Account identity is unknown.
  Invalid rows include safe error codes; the command reports all rows, then exits 3.
  TODOIST_TOKEN overrides every stored credential without inheriting its authorization evidence.`,
		globals: `  --config <path>       Choose the configuration directory
  --json, --ndjson       Structured report output`,
	},
	"profile current": {
		usage:    ``,
		examples: `  todoist profile current --json`,
		notes: `  Reports profile selection and the effective credential source offline.
  TODOIST_TOKEN reports external/unknown authorization; the selected profile is overridden context.
  Without an environment token, missing or disabled selection is reported and exits 4.
  Native accessibility stays unchecked; invalid metadata fails closed.`,
		globals: `  --profile <name>      Override profile selection for this invocation
  --config <path>       Choose the user configuration
  --json, --ndjson       Structured report output`,
	},
	"profile use": {
		usage: `<name>`,
		examples: `  todoist profile use work
  todoist --config ./scratch/config.json profile use reader --json`,
		notes: `  Requires an existing, enabled profile with valid or legacy metadata; native accessibility is unchecked.
  Saves only default_profile in the resolved user config, preserving unrelated and unknown fields.
  Precedence remains --profile > TODOIST_PROFILE > project .todoist.json > user config > default.
  Reports when project, environment, or flag selection shadows the saved choice.
  TODOIST_TOKEN still overrides the credential. Project config is never modified.`,
		globals: `  --config <path>       Configuration receiving the saved default
  --json, --ndjson       Structured acknowledgement output`,
	},
	"profile remove": {
		usage: `<name>`,
		examples: `  todoist profile remove scratch
  todoist --profile scratch auth repair`,
		notes: `  Removes only the explicitly named credential profile; absent names succeed idempotently.
  Keeps default_profile unchanged: a removed selection stays missing until explicit use or login.
  Disables the profile before native deletion; cleanup failure returns exit 3 with committed=true.
  Repeat remove or auth repair to retry cleanup. Cross-store deletion is not atomic.
  Does not revoke the grant or unset TODOIST_TOKEN. Invalid authorization metadata may be removed.`,
		globals: `  --config <path>       Choose the configuration directory
  --json, --ndjson       Structured acknowledgement output`,
	},
	"auth login": {
		usage: `[--token-stdin] [--credential-store <native|file>] [flags]
--oauth [--read-only] [--client-id <id>] [flags]
--oauth-device [--read-only] [--client-id <id>] [flags]`,
		flags: `  --credential-store <native|file>  Storage for a new profile: native or file
  --read-only                       Request read-only OAuth access
  --token-stdin                     Read token from stdin
  --print-env                       Print export command instead of saving
  --oauth                           Authenticate via OAuth PKCE flow
  --oauth-device                    Authenticate via OAuth device flow
  --no-browser                      Do not auto-open browser for OAuth flow
  --client-id <id>                  OAuth client ID (or TODOIST_OAUTH_CLIENT_ID)
  --oauth-authorize-url <url>       OAuth authorize URL
  --oauth-token-url <url>           OAuth token URL
  --oauth-device-url <url>          OAuth device code URL
  --oauth-listen <host:port>        OAuth callback address (default 127.0.0.1:8765)
  --oauth-redirect-uri <uri>        OAuth redirect URI (default http://<listen>/callback)`,
		examples: `  todoist auth login
  todoist auth login --token-stdin < token.txt
  todoist --profile reader auth login --oauth --read-only`,
		notes: `  Paste only the API token, without spaces, quotes, or Bearer. Manual login verifies with Todoist before saving or exporting; failed verification leaves credentials unchanged. Rerun login to retry.
  New profiles use native storage; select file explicitly for portable storage. Existing profiles retain their backend; use auth migrate to change it.
  OAuth defaults to read-write. --read-only requires OAuth and blocks Todoist mutations; planning and previews remain available.
  Manual, environment, and legacy tokens have unknown scopes and permit write attempts. --print-env exports a secret and loses local scope evidence on later environment use.
  Supply your own public PKCE client and registered redirect; no client ID or secret is bundled.
  Short-lived or refresh-bearing grants are rejected before replacement; token refresh is not implemented.
  Device flow support by Todoist is unverified. See README OAuth onboarding for prerequisites.`,
		globals: `  --profile <name>      Select the credential profile
  --no-input            Disable prompts`,
	},
	"auth migrate": {
		usage:    `--credential-store <native|file>`,
		flags:    `  --credential-store <native|file>  Destination: native or file`,
		examples: `  todoist --profile work auth migrate --credential-store native`,
		notes: `  Verifies the destination before switching storage. Existing authorization is preserved.
  Native migration removes the plaintext token; no automatic fallback occurs.
  Older binaries cannot use native profiles; migrate explicitly to file before downgrading.
  If obsolete-entry cleanup is pending, use todoist auth repair. File storage is plaintext.`,
		globals: `  --profile <name>      Select the credential profile
  --no-input            Disable prompts`,
	},
	"auth repair": {
		usage:    ``,
		examples: `  todoist --profile work auth repair`,
		notes: `  Retries recognized interrupted credential transactions and pending cleanup.
  Preserve corrupt files for deliberate recovery; repair does not repair arbitrary corrupt data
  or access another configuration directory's native entries.`,
		globals: `  --profile <name>      Select the credential profile
  --no-input            Disable prompts`,
	},
	"auth status": {
		usage:    ``,
		examples: `  todoist auth status --json`,
		notes: `  Reports the selected profile offline without retrieving native secrets.
  Configured does not verify token accessibility or authentication; use todoist doctor for health checks.`,
		globals: `  --profile <name>      Select the credential profile
  --no-input            Disable prompts`,
	},
	"auth logout": {
		usage:    ``,
		examples: `  todoist --profile work auth logout`,
		notes: `  Disables the selected profile and removes its stored credential. Does not revoke the token or unset TODOIST_TOKEN.
  If native cleanup fails, retry logout or use todoist auth repair.`,
		globals: `  --profile <name>      Select the credential profile
  --no-input            Disable prompts`,
	},
	"task list": {
		usage: `[flags]`,
		flags: `  --filter <query>                  Filter query
  --project <ref>                   Project
  --section <ref>                   Section
  --parent <id>                     Parent task
  --label <name>                    Label
  --id <id>                         Comma-separated task IDs
  --cursor <cursor>                 Cursor
  --limit <n>                       Limit
  --all                             Fetch all pages
  --all-projects                    List tasks from all projects
  --completed                       List completed tasks
  --completed-by <completion|due>   History by completion or due date
  --since <date>                    Start date (RFC3339 or YYYY-MM-DD)
  --until <date>                    End date (RFC3339 or YYYY-MM-DD)
  --wide                            Detailed table (API priorities)
  --preset <today|overdue|next7>    Shortcut filter: today, overdue, next7
  --truncate-width <cols>           Override human output width
` + taskSortFlags,
		examples: `  todoist task list
  todoist task list --all-projects --all --ids-only
  todoist task list --completed --since yesterday --json`,
		notes: `  Lists one page of Inbox tasks by default; --all-projects changes scope, --all fetches every page.
  Human active lists show scope, count, coverage, and titles wrapping to three lines.
  P1 is highest; due labels use the printed UTC date. Selection and ordering are unchanged.
  --wide retains the detailed table (4 is highest); task view id:<id> shows full text.
  --quiet hides summaries, but retains tasks and cursor notices.
  Failed or missing Inbox lookup stops without fetching tasks from other projects.
  Use --project or --filter for an explicit selection; default filters/presets retain API order.
  Completed listing accepts date hints such as yesterday and "2 weeks ago"; --since without --until ends today.
  Use --all to fetch all pages where supported, or follow the returned cursor.
` + taskSortNotes,
		globals: `  --no-input            Disable prompts
  --ids-only            Print one raw ID per result`,
	},
	"task add": {
		usage: `--content <text> [flags]`,
		flags: `  --content <text>                  Task content
  --description <text|->            Exact stdin text with -, including trailing newline
  --project <ref>                   Project
  --section <ref>                   Section
  --parent <id>                     Parent task
  --label <name>                    Label (repeatable)
  --priority <1-4>                  Priority (accepts p1..p4)
  --due <text>                      Due string
  --due-date <YYYY-MM-DD>           Due date
  --due-datetime <RFC3339>          Due datetime
  --due-lang <code>                 Due language
  --duration <n>                    Duration
  --duration-unit <minute|day>      Duration unit
  --deadline <YYYY-MM-DD>           Deadline date
  --assignee <ref>                  Assignee reference (id, me, name, email)
  --quick                           Quick add using inbox defaults
  --reference[=true|false]           Add/remove one leading "* " title prefix
  --order <n>                       Sibling order (signed int32, including zero)
  --natural                         Parse quick-add style tokens in content (#project @label p1..p4 due:...)`,
		examples: `  todoist task add --content "Review launch notes" --project Home --due tomorrow`,
		notes: `  Use --content - for trimmed stdin, or --description - for exact stdin; never both.
  Reference control refuses repeated prefixes and blank remaining titles.
  Unavailable optional task data emits task_write_ack; inspect before retrying creation.
  Use --content - to read stdin. --quick delegates to Inbox defaults; use top-level add for --strict.
  Duration requires --duration and --duration-unit together.
  Priority accepts 1-4 (4 is highest) or p1-p4 (p1 is highest). Labels are repeatable.
  Human capture prints returned task details and a view command, except with --quiet.
  Receipt priority matches p1-p4; machine output and numeric input retain API priorities.
  --dry-run shows proposed fields, not a saved task; due expressions still need Todoist interpretation.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"task update": {
		usage: `<ref> [flags]
--id <id> [flags]`,
		flags: `  --clear-due                       Clear due and recurrence
  --clear-deadline                  Clear deadline
  --clear-labels                    Clear all labels
  --clear-assignee                  Clear assignment
  --clear-description               Clear description
  --id <id>                         Task ID
  --content <text>                  Task content
  --description <text|->            Exact stdin text with -, including trailing newline
  --label <name>                    Label (repeatable)
  --priority <1-4>                  Priority (accepts p1..p4)
  --due <text>                      Due string
  --due-date <YYYY-MM-DD>           Due date
  --due-datetime <RFC3339>          Due datetime
  --due-lang <code>                 Due language
  --duration <n>                    Duration
  --duration-unit <minute|day>      Duration unit
  --deadline <YYYY-MM-DD>           Deadline date
  --assignee <ref>                  Assignee reference (id, me, name, email)
  --project <ref>                   Project (used for assignee name/email resolution)
  --reference[=true|false]           Add/remove one leading "* " title prefix
  --order <n>                       Sibling order (signed int32, including zero)
  --natural                         Parse quick-add style tokens in content (#project @label p1..p4 due:...)`,
		examples: `  todoist task update id:123456 --due tomorrow`,
		notes: `  Use id:<id> for an exact task or a quoted text reference; list tasks first if a reference is ambiguous.
  Duration requires --duration and --duration-unit together.
  Omitted fields remain unchanged; empty description, reference=false, and order=0 are edits.
  Use --description - for exact stdin; empty stdin clears it. Content stdin is trimmed.
  Setters conflict with their clear flags; due setters and natural tokens cannot overlap.
  Use task move to change location; --project here scopes assignee name/email resolution.
  --clear-due with other fields runs two writes: Sync clear, then REST edit.
  A second-step failure emits a partial report and nonzero exit; inspect before resubmitting.
  Acceptance with unavailable optional task data emits task_write_ack, not a task resource.
  Use task reschedule to preserve recurrence; ordinary due updates retain their existing behavior.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"task move": {
		usage: `<ref> [--project <ref>] [--section <ref>] [--parent <id>]
--filter <query> --yes [--project <ref>] [--section <ref>] [--parent <id>]`,
		flags: `  --id <id>                         Task ID
  --project <ref>                   Project
  --section <ref>                   Section
  --parent <id>                     Parent
  --filter <query>                  Filter query for bulk move
  --clear-parent                    Detach to current section, or project if sectionless
  --clear-section                   Remove section, remaining in current project
  --yes                             Required for bulk move`,
		examples: `  todoist task move id:123456 --project Home
  todoist task move id:123456 --project Home --section Backlog --dry-run
  todoist task move --filter "@work & overdue" --project Home --yes --dry-run`,
		notes: `  Supply a destination or hierarchy clear flags. Parent excludes project/section setters.
  Project accompanying section scopes resolution; section is the sole destination.
  Clearing an inherited section from a child requires explicit --clear-parent.
  Clearing both makes a root task in its current project. Already-satisfied clears dispatch no write.
  Filtered batches fully preflight every page and reject duplicate/ancestor overlaps.
  Definite rejections continue; uncertainty stops. Inspect per-target accounting before retrying.
  Use id:<id> for an exact task; use task list to resolve ambiguous references.
  Terminal feedback acknowledges the move and shows returned destination details when available.
  Missing details use labeled requested values; unavailable names retain IDs. No extra lookups.
  Single-task dry runs show task identity and requested destinations; no task is changed.
  Quiet, machine, and redirected output retain their existing acknowledgements and previews.
  Bulk selection uses --filter and requires --yes (or global --force); plain text filters become search text.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"task view": {
		usage: `<ref> [--full]
--id <id> [--full]`,
		flags: `  --id <id>                         Task ID
  --include-children                Fetch every page of direct active children
  --full                            Add exact destination IDs and inspection metadata
` + taskSortFlags,
		examples: `  todoist task view id:123456
  todoist task view id:123456 --full
  todoist task view id:123456 --no-input --json
  todoist task view id:123456 --no-input --ndjson
  todoist task view id:123456 --no-input --json --task-output-version 2`,
		notes: `  --include-children emits task_expanded_view (v1/v2), with task, children, children_complete.
  Expansion failures emit no task collection; completeness means all pages, not a snapshot.
  Sorting applies only to expanded children and requires --include-children.
  Use id:<id> for an exact task or a quoted text reference.
  Multiple matches offer a numbered choice with task context in a terminal; Enter cancels selection.
  With --no-input or piped stdin, use task list --all-projects --all --no-input --json and retry id:<id>.
  Human detail keeps full text, names, P1 highest, absolute dates, recurrence, state, labels, and ID.
  Deadline, duration, assignee ID, due language, and title-derived reference-item status are shown.
  Text wraps without truncation; returned offsets/timezones are not converted.
  --include-children                Fetch every page of direct active children
  --full adds destination IDs, timestamps, order, actor IDs, flags, and returned counters.
  The deprecated note_count value is not a reliable comment count.
  None/No due date means known absence; Not returned means unknown information.
  Name lookups add collection requests; failure retains IDs and says lookup failed.
  Machine output has no enrichment. --json emits a task object; --ndjson emits the same object on one line.
  --include-children                Fetch every page of direct active children
  --full does not change either payload; plain/redirected retain legacy labeled text.
  --task-output-version 2 preserves supported returned fields, including absent/null/false/zero.
  Use schema --name task_item_v2 for JSON view and NDJSON records; task_list_v2 for JSON arrays.
  Version selection requires --json or --ndjson and rejects previews and acknowledgement commands.
` + taskSortNotes,
		globals: `  --no-input            Disable prompts`,
	},
	"task reschedule": {
		usage: `<ref> (--due-date <date> | --due-datetime <RFC3339> | --due-local-datetime <local>)`,
		flags: `  --id <id>                         Exact task ID instead of a positional reference
  --due-date <YYYY-MM-DD>            Replace date retaining existing clock and time character
  --due-datetime <RFC3339>           Exact instant for existing fixed-zone due time
  --due-local-datetime <local>       YYYY-MM-DDTHH:MM:SS for existing floating due time`,
		examples: `  todoist task reschedule id:123456 --due-date 2026-10-15
  todoist task reschedule --id 123456 --due-datetime 2026-10-15T14:00:00Z`,
		notes: `  Preserves explicit recurrence and existing date-only, floating, or fixed-zone character.
  Requires returned primary date, recurrence, and timezone evidence; refuses unknown/contradictory facts.
  A recurring task also requires its returned expression and language. No type conversions.
  Date replacement in a DST gap/fold refuses; an explicit instant resolves a fixed-zone fold.
  Uses native Sync item_update and requires its exact command acknowledgement.
  JSON success is a returned task array, or task_write_ack when optional task data is unavailable.`,
		globals: `  -n, --dry-run          Preview without mutations
  --no-input            Disable prompts`,
	},
	"task complete": {
		usage: `<ref>
--id <id>
--filter <query> --yes`,
		flags: `  --id <id>                         Task ID
  --filter <query>                  Filter query for bulk complete
  --forever                         Native permanent completion, including subtasks
  --yes                             Required for bulk complete`,
		examples: `  todoist task complete id:123456
  todoist task complete id:123456 --dry-run
  todoist task complete --filter "@work & overdue" --yes --dry-run`,
		notes: `  Use task list to find a task, then id:<id> for an exact reference; quoted task text also works.
  Terminal feedback says Completion accepted and includes known pre-action task context and full ID.
  --forever permanently completes recurrence and subtasks using a native operation.
  Filtered batches preflight all pages; uncertainty stops with explicit target accounting.
  Recurring completion advances an occurrence; it does not permanently finish the task. No next date is returned.
  --id avoids a task lookup and may show only the ID. Human single-task dry runs show intent without changes.
  Quiet, machine, and redirected output retain their existing acknowledgements and previews.
  Bulk completion requires --yes (or global --force). Preview with --dry-run; previews may read Todoist.
  For an ordinary completed task, use todoist task reopen --id <id> to reopen it.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"task reopen": {
		usage: `<ref>
--id <id>`,
		flags:    `  --id <id>                         ID`,
		examples: `  todoist task reopen --id 123456`,
		notes:    `  Use an ID from todoist task list --completed. Text references search active tasks, so prefer a known ID for a completed task.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"task delete": {
		usage: `<ref> --yes
--id <id> --yes`,
		flags: `  --id <id>                         Task ID
  --yes                             Required confirmation, including previews`,
		examples: `  todoist task delete id:123456 --yes --dry-run`,
		notes: `  Deletion requires --yes even with --force or --dry-run. Preview before deleting.
  Use task list to resolve ambiguous references; use task complete if the task is finished.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"project list": {
		usage: `[--archived] [flags]`,
		flags: `  --archived                        List archived projects
  --cursor <cursor>                 Cursor
  --limit <n>                       Limit
  --all                             Fetch all pages`,
		examples: `  todoist project list --all --json`,
		notes:    `  Use --archived for archived projects. Use --all or the returned cursor to read additional pages.`,
		globals: `  --no-input            Disable prompts
  --ids-only            Print one raw ID per result`,
	},
	"project view": {
		usage: `<id|name>
--id <id|name>`,
		flags:    `  --id <id>                         Project ID or name`,
		examples: `  todoist project view "Home"`,
		notes:    `  Quote multiword names. Use an ID from project list if the name is ambiguous.`,
		globals:  `  --no-input            Disable prompts`,
	},
	"project browse": {
		usage: `<id|name>
--id <id|name>`,
		flags:    `  --id <id>                         Project ID or name`,
		examples: `  todoist project browse "Home"`,
		notes:    `  Opens the project URL in your browser; --dry-run previews the URL without opening it.`,
		globals:  `  --no-input            Disable prompts`,
	},
	"project collaborators": {
		usage: `<id|name> [flags]`,
		flags: `  --id <id>                         Project ID or name
  --cursor <cursor>                 Cursor
  --limit <n>                       Limit
  --all                             Fetch all pages`,
		examples: `  todoist project collaborators "Home" --json`,
		notes:    `  Quote multiword names. Use --all or the returned cursor for more collaborators.`,
		globals: `  --no-input            Disable prompts
  --ids-only            Print one raw ID per result`,
	},
	"project add": {
		usage: `--name <name> [flags]`,
		flags: `  --name <name>                     Project name
  --description <text>              Description
  --parent <id>                     Parent project
  --color <color>                   Color
  --favorite                        Favorite
  --view <style>                    View style
  --workspace <id>                  Workspace ID`,
		examples: `  todoist project add --name "Side Projects" --description "Weekend work"`,
		notes:    `  Use --parent for a parent project and --workspace for a workspace ID. Use workspace list to find IDs.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"project update": {
		usage: `--id <id> [flags]`,
		flags: `  --id <id>                         Project ID
  --name <name>                     Project name
  --description <text>              Description
  --color <color>                   Color
  --favorite                        Favorite
  --view <style>                    View style`,
		examples: `  todoist project update --id 234 --name "Home Projects"`,
		notes:    `  Requires --id; find IDs with project list. Use project move to change workspace.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"project move": {
		usage: `<id|name> (--to-workspace <ref> | --to-personal) [flags]`,
		flags: `  --id <id>                         Project ID or name
  --to-workspace <ref>              Target workspace
  --to-personal                     Move project to personal
  --visibility <restricted|team|public>Workspace visibility (restricted|team|public)
  --yes                             Confirm move`,
		examples: `  todoist project move "Team Project" --to-personal --yes`,
		notes: `  Choose exactly one destination. Confirm with --yes, global --force, or an interactive prompt.
  Use workspace list to find workspace IDs; visibility applies to workspace moves.`,
		globals: `  --no-input            Disable prompts`,
	},
	"project archive": {
		usage:    `--id <id>`,
		flags:    `  --id <id>                         ID`,
		examples: `  todoist project archive --id 234 --dry-run`,
		notes:    `  Find the ID with project list. Prompts in a terminal; use --force to confirm without prompting.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"project unarchive": {
		usage:    `--id <id>`,
		flags:    `  --id <id>                         ID`,
		examples: `  todoist project unarchive --id 234`,
		notes:    `  Find the ID with project list --archived.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"project delete": {
		usage:    `--id <id>`,
		flags:    `  --id <id>                         ID`,
		examples: `  todoist project delete --id 234 --dry-run`,
		notes:    `  Find the ID with project list. Prompts in a terminal; use --force to confirm without prompting.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"filter list": {
		usage:    ``,
		examples: `  todoist filter list --json`,
		notes:    `  Lists saved filters. Use filter show to list tasks matching a saved filter.`,
		globals: `  --no-input            Disable prompts
  --ids-only            Print one raw ID per result`,
	},
	"filter show": {
		usage:    `<id|name> [--sort <key>]`,
		flags:    taskSortFlags,
		examples: `  todoist filter show "Work today"`,
		notes: `  Uses a saved filter's query to list tasks. Find names and IDs with filter list.
` + taskSortNotes,
		globals: `  --no-input            Disable prompts
  --ids-only            Print one raw ID per result`,
	},
	"filter add": {
		usage: `--name <name> --query <query> [flags]`,
		flags: `  --name <name>                     Filter name
  --query <query>                   Filter query
  --color <color>                   Color
  --favorite                        Favorite`,
		examples: `  todoist filter add --name "Work today" --query "@work & today"`,
		notes:    `  Quote Todoist queries to protect shell characters such as &.`,
		globals:  `  --no-input            Disable prompts`,
	},
	"filter update": {
		usage: `<id|name> [flags]
--id <id|name> [flags]`,
		flags: `  --id <id>                         Filter ID or name
  --name <name>                     Filter name
  --query <query>                   Filter query
  --color <color>                   Color
  --favorite                        Favorite
  --unfavorite                      Unfavorite`,
		examples: `  todoist filter update "Work today" --query "@work & (today | overdue)"`,
		notes:    `  Find saved filters with filter list; quote query text and multiword names.`,
		globals:  `  --no-input            Disable prompts`,
	},
	"filter delete": {
		usage: `<id|name> --yes
--id <id|name> --yes`,
		flags: `  --id <id>                         Filter ID or name
  --yes                             Skip confirmation`,
		examples: `  todoist filter delete "Work today" --yes --dry-run`,
		notes:    `  Requires --yes or global --force. Removes the saved filter, not its matching tasks.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"workspace list": {
		usage:    ``,
		examples: `  todoist workspace list --json`,
		notes:    `  Use workspace IDs with project creation or project move.`,
		globals: `  --no-input            Disable prompts
  --ids-only            Print one raw ID per result`,
	},
	"section list": {
		usage: `[--project <ref>] [flags]`,
		flags: `  --project <ref>                   Project
  --cursor <cursor>                 Cursor
  --limit <n>                       Limit
  --all                             Fetch all pages`,
		examples: `  todoist section list --project Home --all`,
		notes:    `  Use project list to find project references. Follow the returned cursor or use --all for more results.`,
		globals: `  --no-input            Disable prompts
  --ids-only            Print one raw ID per result`,
	},
	"section add": {
		usage: `--name <name> --project <ref>`,
		flags: `  --name <name>                     Section name
  --project <ref>                   Project`,
		examples: `  todoist section add --name Backlog --project "Side Projects"`,
		notes:    `  Use project list to find the destination project.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"section update": {
		usage: `--id <id> --name <name>`,
		flags: `  --id <id>                         Section ID
  --name <name>                     Section name`,
		examples: `  todoist section update --id 345 --name "Ready for review"`,
		notes:    `  Requires a section ID from section list.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"section delete": {
		usage:    `--id <id>`,
		flags:    `  --id <id>                         ID`,
		examples: `  todoist section delete --id 345 --dry-run`,
		notes:    `  Requires a section ID from section list. Prompts in a terminal; --force confirms without prompting.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"label list": {
		usage: `[flags]`,
		flags: `  --cursor <cursor>                 Cursor
  --limit <n>                       Limit
  --all                             Fetch all pages`,
		examples: `  todoist label list --all --json`,
		notes:    `  Use --all or the returned cursor for additional labels.`,
		globals: `  --no-input            Disable prompts
  --ids-only            Print one raw ID per result`,
	},
	"label add": {
		usage: `--name <name> [flags]`,
		flags: `  --name <name>                     Label name
  --color <color>                   Label color
  --order <n>                       Order
  --favorite                        Favorite`,
		examples: `  todoist label add --name focus --color red --favorite`,
		notes:    `  Use label list to inspect existing labels.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"label update": {
		usage: `--id <id> [flags]`,
		flags: `  --id <id>                         Label ID
  --name <name>                     Label name
  --color <color>                   Label color
  --order <n>                       Order
  --favorite                        Favorite
  --unfavorite                      Unfavorite`,
		examples: `  todoist label update --id 456 --name deep-work`,
		notes:    `  Requires a label ID from label list. --favorite and --unfavorite are mutually exclusive.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"label delete": {
		usage:    `--id <id>`,
		flags:    `  --id <id>                         ID`,
		examples: `  todoist label delete --id 456 --dry-run`,
		notes:    `  Requires a label ID from label list. Prompts in a terminal; --force confirms without prompting.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"comment list": {
		usage: `(--task <id> | --project <ref>) [flags]`,
		flags: `  --task <ref>                      Task ID
  --project <ref>                   Project ID
  --cursor <cursor>                 Cursor
  --limit <n>                       Limit
  --all                             Fetch all pages`,
		examples: `  todoist comment list --task 123456 --all --json`,
		notes:    `  Choose a task or project target. Use --all or the returned cursor for additional comments.`,
		globals: `  --no-input            Disable prompts
  --ids-only            Print one raw ID per result`,
	},
	"comment add": {
		usage: `--content <text> (--task <id> | --project <ref>)`,
		flags: `  --content <text>                  Comment content
  --task <ref>                      Task ID
  --project <ref>                   Project ID`,
		examples: `  todoist comment add --task 123456 --content "Ready for review"`,
		notes:    `  Choose one target. Comment content is passed literally; quote spaces and shell characters.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"comment update": {
		usage: `--id <id> --content <text>`,
		flags: `  --id <id>                         Comment ID
  --content <text>                  Comment content`,
		examples: `  todoist comment update --id 567 --content "Review complete"`,
		notes:    `  Find comment IDs with comment list; --content replaces the comment text literally.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"comment delete": {
		usage:    `--id <id>`,
		flags:    `  --id <id>                         ID`,
		examples: `  todoist comment delete --id 567 --dry-run`,
		notes:    `  Find comment IDs with comment list. Prompts in a terminal; --force confirms without prompting.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"reminder list": {
		usage:    `(<task> | --task <ref>)`,
		flags:    `  --task <ref>                      Task reference`,
		examples: `  todoist reminder list --task id:123456`,
		notes:    `  Find the task with task list; use an exact ID if its name is ambiguous.`,
		globals: `  --no-input            Disable prompts
  --ids-only            Print one raw ID per result`,
	},
	"reminder add": {
		usage: `(<task> | --task <ref>) (--before <duration> | --at <datetime>)`,
		flags: `  --task <ref>                      Task reference
  --before <duration>               Reminder offset before due (e.g. 30m, 1h)
  --at <datetime>                   Reminder datetime (RFC3339 or YYYY-MM-DD HH:MM)`,
		examples: `  todoist reminder add --task id:123456 --before 30m`,
		notes:    `  Choose --before or --at. Relative reminders use the task's due time; absolute times accept RFC3339 or YYYY-MM-DD HH:MM.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"reminder update": {
		usage: `(<id> | --id <id>) (--before <duration> | --at <datetime>)`,
		flags: `  --id <id>                         Reminder ID
  --before <duration>               Reminder offset before due (e.g. 30m, 1h)
  --at <datetime>                   Reminder datetime (RFC3339 or YYYY-MM-DD HH:MM)`,
		examples: `  todoist reminder update --id 678 --before 1h`,
		notes:    `  Find reminder IDs with reminder list; choose --before or --at.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"reminder delete": {
		usage: `(<id> | --id <id>) [--yes]`,
		flags: `  --id <id>                         Reminder ID
  --yes                             Skip confirmation`,
		examples: `  todoist reminder delete --id 678 --yes --dry-run`,
		notes:    `  Find reminder IDs with reminder list. Confirm using --yes, global --force, or an interactive prompt.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"notification list": {
		usage: `[flags]`,
		flags: `  --type <types>                    Filter by notification type (comma-separated)
  --unread                          Only unread notifications
  --read                            Only read notifications
  --limit <n>                       Max notifications to show
  --offset <n>                      Skip first N notifications`,
		examples: `  todoist notification list --unread --limit 20`,
		notes:    `  --read and --unread are mutually exclusive. Use --offset for additional results.`,
		globals: `  --no-input            Disable prompts
  --ids-only            Print one raw ID per result`,
	},
	"notification view": {
		usage:    `[id] [--id <id>]`,
		flags:    `  --id <id>                         Notification ID`,
		examples: `  todoist notification view --id 789`,
		notes:    `  Find IDs with notification list; an ID is required.`,
		globals:  `  --no-input            Disable prompts`,
	},
	"notification accept": {
		usage:    `[id] [--id <id>]`,
		flags:    `  --id <id>                         ID`,
		examples: `  todoist notification accept --id 789`,
		notes:    `  Requires an ID from notification list. Accepts share_invitation_sent invitations only.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"notification reject": {
		usage:    `[id] [--id <id>]`,
		flags:    `  --id <id>                         ID`,
		examples: `  todoist notification reject --id 789`,
		notes:    `  Requires an ID from notification list. Rejects share_invitation_sent invitations only.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"notification read": {
		usage: `<id>
--id <id>
--all --yes`,
		flags: `  --id <id>                         Notification ID
  --all                             Mark all notifications as read
  --yes                             Confirm --all operation`,
		examples: `  todoist notification read --id 789
  todoist notification read --all --yes --dry-run`,
		notes: `  Find IDs with notification list. Marking all requires --yes or --force, except in --dry-run.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"notification unread": {
		usage:    `[id] [--id <id>]`,
		flags:    `  --id <id>                         Notification ID`,
		examples: `  todoist notification unread --id 789`,
		notes:    `  Requires an ID from notification list.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"stats goals": {
		usage: `[--daily <n>] [--weekly <n>]`,
		flags: `  --daily <n>                       Set daily goal
  --weekly <n>                      Set weekly goal`,
		examples: `  todoist stats goals --daily 5 --weekly 25 --dry-run`,
		notes:    `  Supply at least one non-negative goal. Use bare todoist stats to inspect productivity.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"stats vacation": {
		usage: `(--on | --off)`,
		flags: `  --on                              Enable vacation mode
  --off                             Disable vacation mode`,
		examples: `  todoist stats vacation --on --dry-run`,
		notes:    `  Choose exactly one of --on or --off.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"settings view": {
		usage:    ``,
		examples: `  todoist settings view --json`,
		notes:    `  Reads account settings. Use settings themes to discover supported theme names.`,
		globals:  `  --no-input            Disable prompts`,
	},
	"settings update": {
		usage: `[flags]`,
		flags: `  --timezone <value>                Timezone (for example UTC, Europe/London)
  --time-format <value>             Time format: 12 or 24
  --date-format <value>             Date format: us or intl
  --start-day <value>               Week start day
  --theme <value>                   Theme name
  --auto-reminder <value>           Default reminder minutes
  --next-week <value>               "Next week" day
  --start-page <value>              Start page
  --reminder-push <value>           Push reminders on/off
  --reminder-desktop <value>        Desktop reminders on/off
  --reminder-email <value>          Email reminders on/off
  --completed-sound-desktop <value> Desktop completion sound on/off
  --completed-sound-mobile <value>  Mobile completion sound on/off`,
		examples: `  todoist settings update --timezone Europe/Berlin --dry-run`,
		notes:    `  Supply at least one setting. Use settings view to inspect current values and settings themes for theme names.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"settings themes": {
		usage:    ``,
		examples: `  todoist settings themes`,
		notes:    `  Lists supported theme names for settings update.`,
	},
	"agent plan": {
		usage: `<instruction> [flags]`,
		flags: `  --out <file>                      Output plan file
  --planner <command>               Planner command
  --plan-version <n>                Expected plan version
  --context-project <ref>           Project context (repeatable)
  --context-label <name>            Label context (repeatable)
  --context-completed <Nd>          Include completed tasks from last Nd (e.g. 7d)`,
		examples: `  todoist agent plan "Review overdue tasks" --out plan.json`,
		notes: `  Runs the configured external planner and saves the last plan locally; --out also writes the requested path.
  Configure a planner with todoist agent planner. Review the plan before applying it.`,
		globals: `  --no-input            Disable prompts`,
	},
	"agent apply": {
		usage: `--plan <file> --confirm <token> [flags]
<instruction> --confirm <token> [flags]`,
		flags: `  --plan <file>                     Plan file (or - for stdin)
  --confirm <token>                 Confirmation token
  --planner <command>               Planner command
  --policy <file>                   Policy file path
  --on-error <fail|continue>        On error: fail|continue
  --plan-version <n>                Expected plan version
  --context-project <ref>           Project context (repeatable)
  --context-label <name>            Label context (repeatable)
  --context-completed <Nd>          Include completed tasks from last Nd (e.g. 7d)`,
		examples: `  todoist agent apply --plan plan.json --force --dry-run`,
		notes: `  Use --plan - to read stdin. Inspect the plan's confirm_token; previews also require --confirm or --force. --dry-run sends no Todoist mutations.
  Authorization and replay-record failures stop execution even with --on-error continue. External planners are trusted programs.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"agent status": {
		usage:    ``,
		examples: `  todoist agent status --json`,
		notes:    `  Inspects local planner configuration and last-plan availability. Does not run the planner or apply a plan.`,
		globals:  `  --no-input            Disable prompts`,
	},
	"agent run": {
		usage: `[--plan <file> | --instruction <text>] [flags]`,
		flags: `  --plan <file>                     Plan file (or - for stdin)
  --instruction <text>              Instruction to plan/apply
  --planner <command>               Planner command
  --confirm <token>                 Confirmation token
  --on-error <fail|continue>        On error: fail|continue
  --plan-version <n>                Expected plan version
  --out <file>                      Write plan output to file
  --policy <file>                   Policy file path
  --context-project <ref>           Project context (repeatable)
  --context-label <name>            Label context (repeatable)
  --context-completed <Nd>          Include completed tasks from last Nd (e.g. 7d)`,
		examples: `  todoist agent run --instruction "Review overdue tasks" --force --dry-run`,
		notes: `  Combines planning and application. Application and previews need the plan's confirmation token or --force.
  Dry runs may invoke the external planner and write local plan files; external planners are trusted programs.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"agent examples": {
		usage:    ``,
		examples: `  todoist agent examples`,
		notes:    `  Prints CLI invocation examples without running the planner.`,
	},
	"agent planner": {
		usage: `[--set --cmd <command>]`,
		flags: `  --cmd <command>                   Planner command to set
  --set                             Set planner command`,
		examples: `  todoist agent planner
  todoist agent planner --set --cmd "my-planner"`,
		notes: `  Without --set, reports the configured planner. --set requires --cmd and saves local configuration.
  The top-level planner command is equivalent. Planner programs are trusted and not sandboxed.`,
		globals: `  --no-input            Disable prompts`,
	},
	"agent schedule print": {
		usage: `--weekly <day HH:MM> [flags]`,
		flags: `  --weekly <day HH:MM>              Weekly schedule, e.g. "sat 09:00"
  --planner <command>               Planner command
  --instruction <text>              Instruction to plan/apply
  --policy <file>                   Policy file path
  --plan <file>                     Plan file (or - for stdin)
  --confirm <token>                 Confirmation token
  --on-error <fail|continue>        On error: fail|continue
  --plan-version <n>                Expected plan version
  --context-project <ref>           Project context (repeatable)
  --context-label <name>            Label context (repeatable)
  --context-completed <Nd>          Include completed tasks from last Nd (e.g. 7d)
  --cron                            Print cron entry
  --bin <path>                      Path to todoist binary (defaults to current executable)`,
		examples: `  todoist agent schedule print --weekly "sat 09:00" --instruction "Review overdue tasks" --force --dry-run --cron`,
		notes: `  Prints launchd XML by default or a cron line with --cron; it does not install or run the schedule.
  Confirmation, profile, config, and preview selections are carried into the generated command.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
	"completion install": {
		usage:    `[bash|zsh|fish|powershell|pwsh] [--path <file>]`,
		flags:    `  --path <file>                     Install path override`,
		examples: `  todoist completion install zsh`,
		notes: `  Omitting the shell uses shell detection; pass it explicitly if detection fails.
  Writes a completion file and prints activation instructions; does not edit shell startup files.`,
	},
	"completion uninstall": {
		usage:    `[bash|zsh|fish|powershell|pwsh] [--path <file>]`,
		flags:    `  --path <file>                     Uninstall path override`,
		examples: `  todoist completion uninstall zsh`,
		notes: `  Omitting the shell removes default scripts for supported shells. --path selects an explicit file.
  Unrecognized PowerShell files are preserved; shell startup files are not edited.`,
	},
	"completion bash": {
		usage:    ``,
		examples: `  todoist completion bash > todoist-completion.bash`,
		notes:    `  Prints shell source only. Use completion install bash for installation and activation guidance.`,
	},
	"completion zsh": {
		usage:    ``,
		examples: `  todoist completion zsh > _todoist`,
		notes:    `  Prints shell source only. Use completion install zsh and follow its activation instructions; do not source _todoist directly.`,
	},
	"completion fish": {
		usage:    ``,
		examples: `  todoist completion fish > todoist.fish`,
		notes:    `  Prints shell source only. Use completion install fish for installation and activation guidance.`,
	},
	"completion powershell": {
		usage:    ``,
		examples: `  todoist completion powershell > todoist.ps1`,
		notes:    `  Prints shell source only. pwsh is an alias; use completion install powershell for activation guidance.`,
	},
	"inbox add": {
		usage: `--content <text> [flags]`,
		flags: `  --content <text>                  Task content
  --description <text>              Task description
  --section <ref>                   Section
  --label <name>                    Label (repeatable)
  --priority <1-4>                  Priority
  --due <text>                      Due string
  --due-date <YYYY-MM-DD>           Due date
  --due-datetime <RFC3339>          Due datetime
  --due-lang <code>                 Due language
  --duration <n>                    Duration
  --duration-unit <minute|day>      Duration unit
  --deadline <YYYY-MM-DD>           Deadline date
  --assignee <ref>                  Assignee ID`,
		examples: `  todoist inbox add --content "Review launch notes" --due tomorrow`,
		notes: `  Adds to Inbox using configured label/due defaults where not explicitly supplied.
  Positional text is also accepted; --content - reads stdin. Priority is numeric 1-4 (4 is highest).
  Human capture prints returned task details and a view command, except with --quiet.
  Receipt priority matches Todoist p1-p4; machine output retains API priorities.
  --dry-run shows proposed fields, not a saved task; due expressions still need Todoist interpretation.`,
		globals: `  -n, --dry-run          Preview without Todoist mutations (reads may occur)
  --no-input            Disable prompts`,
	},
}
