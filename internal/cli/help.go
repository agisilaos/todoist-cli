package cli

import "fmt"

func printRootHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `todoist - A terminal companion for Todoist

Usage:
  todoist [global flags] <command> [args]

`+rootCommandListing()+`
Global flags:
  -h, --help            Show help
  --version             Show version
  -q, --quiet           Suppress non-essential output
  --quiet-json          Compact single-line JSON errors
  -v, --verbose         Enable verbose output
  --accessible          Add text markers for screen-reader-friendly task output
  --json                JSON output
  --plain               Plain text output (tab-separated)
  --ndjson              NDJSON output
  --ids-only            One raw ID per line (supported lists only)
  --no-color            Disable color
  --no-input            Disable prompts
  --timeout <seconds>   Request timeout (default 10)
  --config <path>       Config file path
  --profile <name>      Profile name (default "default")
  -n, --dry-run         Preview changes without applying
  -f, --force           Skip confirmation prompts
  --fuzzy               Enable fuzzy name resolution
  --no-fuzzy            Disable fuzzy name resolution
  --progress-jsonl      Emit progress events as JSONL to stderr or file
  --base-url <url>      Override API base URL

Examples:
  todoist auth login
  todoist add "Review PR 42 today"
  todoist inbox
  todoist today
  todoist upcoming
  todoist task list --all-projects --all
  todoist help task

List scope:
  task list defaults to Inbox; --all-projects selects across projects.
  --all fetches every page. inbox, today, and upcoming fetch every page.

Machine clients:
  Use task add for explicit fields; add uses natural language parsing.
  Use --no-input with explicit inputs and --json, --ndjson, --plain,
  or --ids-only for stable output. --quiet-json makes JSON errors compact.
  Example: todoist task list --all-projects --all --no-input --json
  Output contracts: todoist schema --name task_list
  ID output and supported commands: todoist schema --name ids_only
`)
}

func helpCommand(ctx *Context, args []string) error {
	if len(args) == 0 {
		printRootHelp(ctx.Stdout)
		return nil
	}
	// Retain the historical help-only examples topic.
	if args[0] == "examples" {
		return agentExamples(ctx)
	}
	path, err := resolveHelpPath(args)
	if err != nil {
		return err
	}
	if page, ok := commandHelpCatalog[path]; ok && page.examples != "" {
		printLeafHelp(ctx.Stdout, path, page)
		return nil
	}
	if path == "agent schedule" {
		printAgentScheduleHelp(ctx.Stdout)
		return nil
	}
	if path == "" || path == "help" {
		printRootHelp(ctx.Stdout)
		return nil
	}
	switch args[0] {
	case "inbox":
		printInboxHelp(ctx.Stdout)
	case "auth":
		printAuthHelp(ctx.Stdout)
	case "add":
		printAddHelp(ctx.Stdout)
	case "review":
		printReviewHelp(ctx.Stdout)
	case "today":
		printTodayHelp(ctx.Stdout)
	case "completed":
		printCompletedHelp(ctx.Stdout)
	case "upcoming":
		printUpcomingHelp(ctx.Stdout)
	case "task":
		printTaskHelp(ctx.Stdout)
	case "filter":
		printFilterHelp(ctx.Stdout)
	case "project":
		printProjectHelp(ctx.Stdout)
	case "workspace":
		printWorkspaceHelp(ctx.Stdout)
	case "section":
		printSectionHelp(ctx.Stdout)
	case "label":
		printLabelHelp(ctx.Stdout)
	case "comment":
		printCommentHelp(ctx.Stdout)
	case "reminder":
		printReminderHelp(ctx.Stdout)
	case "notification":
		printNotificationHelp(ctx.Stdout)
	case "activity":
		printActivityHelp(ctx.Stdout)
	case "stats":
		printStatsHelp(ctx.Stdout)
	case "settings":
		printSettingsHelp(ctx.Stdout)
	case "view":
		printViewHelp(ctx.Stdout)
	case "agent":
		printAgentHelp(ctx.Stdout)
	case "completion":
		printCompletionHelp(ctx.Stdout)
	case "doctor":
		printDoctorHelp(ctx.Stdout)
	case "schema":
		printSchemaHelp(ctx.Stdout)
	case "planner":
		printAgentPlannerHelp(ctx.Stdout)
	default:
		printRootHelp(ctx.Stdout)
	}
	return nil
}

func printAuthHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist auth login [--token-stdin] [--print-env] [--credential-store=native|file]
  todoist auth login --oauth [--read-only] [--client-id <id>] [--no-browser] [--print-env]
  todoist auth login --oauth-device [--read-only] [--client-id <id>] [--print-env]
  todoist auth status
  todoist auth logout
  todoist auth migrate --credential-store=native|file
  todoist auth repair

Storage:
  New profiles require native storage by default; file storage is an explicit fallback.
  Existing profiles keep their backend. Status reads metadata without retrieving secrets.
  Keychain never prompts; unlock or adjust it externally, then retry.
  Manual login verifies the token with Todoist before saving; network access is required.

Examples:
  todoist auth login
  todoist auth login --token-stdin < token.txt
  todoist auth login --oauth --client-id "$TODOIST_OAUTH_CLIENT_ID"
  todoist --profile reader auth login --oauth --read-only
  todoist auth login --oauth-device --client-id "$TODOIST_OAUTH_CLIENT_ID"
  todoist auth login --oauth --no-browser
  todoist auth login --print-env
`)
}

func printAuthLoginHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist auth login [--token-stdin] [--print-env] [--credential-store=native|file]
  todoist auth login --oauth [--read-only] [--client-id <id>] [--no-browser] [--print-env]
  todoist auth login --oauth-device [--read-only] [--client-id <id>] [--print-env]
                    [--oauth-authorize-url <url>] [--oauth-token-url <url>]
                    [--oauth-device-url <url>]
                    [--oauth-listen <host:port>] [--oauth-redirect-uri <uri>]

Flags:
  --credential-store          New-profile backend: native or file (existing profiles require migration)
  --token-stdin                Read token from stdin
  --print-env                  Print token export instead of saving profile credentials
  --oauth                      Authenticate using OAuth PKCE flow
  --oauth-device               Authenticate using OAuth device flow (headless-friendly)
  --read-only                  Request data:read; requires --oauth or --oauth-device
  --no-browser                 Do not auto-open browser for OAuth flow
  --client-id <id>             OAuth client ID (or TODOIST_OAUTH_CLIENT_ID)
  --oauth-authorize-url <url>  OAuth authorize URL override
  --oauth-token-url <url>      OAuth token URL override
  --oauth-device-url <url>     OAuth device code URL override
  --oauth-listen <host:port>   OAuth callback listen address (default 127.0.0.1:8765)
  --oauth-redirect-uri <uri>   OAuth redirect URI (default http://<listen>/callback)

Examples:
  todoist auth login
  todoist auth login --token-stdin < token.txt
  todoist auth login --oauth --client-id "$TODOIST_OAUTH_CLIENT_ID"
  todoist --profile reader auth login --oauth --read-only
  todoist auth login --oauth-device --client-id "$TODOIST_OAUTH_CLIENT_ID"
  todoist auth login --oauth --no-browser --print-env

Notes:
  Paste only the API token from Todoist settings, without spaces, quotes, or Bearer.
  Manual tokens are checked with Todoist before saving or --print-env; network access is required.
  Failed verification leaves existing credentials unchanged. Rerun login to retry.
  OAuth defaults to read-write; --read-only blocks Todoist mutations, including agent apply/run.
  Planning, local inspection, and dry runs remain available with read-only credentials.
  Manual, environment, and legacy tokens have unknown scopes and permit write attempts.
  --print-env exports only the token; later environment use has unknown authorization.
  Device flow endpoint support by Todoist is unverified. Token refresh is not implemented.
`)
}

func printTaskHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist task list [--filter <query>] [--project <id|name>] [--section <id|name>] [--label <name>] [--completed] [--completed-by completion|due] [--since <date>] [--until <date>] [--wide] [--all-projects]
  todoist task add --content <text> [flags]
  todoist task view <ref> [--full]
  todoist task update <ref> [flags]
  todoist task move <ref> [--project <id|name>] [--section <id|name>] [--parent <id>]
  todoist task move --filter <query> [--project <id|name>] [--section <id|name>] [--parent <id>] --yes
  todoist task complete <ref>
  todoist task complete --filter <query> --yes
  todoist task reopen <ref>
  todoist task delete <ref> --yes

Task flags:
  --content <text>           Task content ("-" reads stdin)
  --quick                    Quick add using inbox defaults
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

Notes:
  By default, task list shows Inbox tasks. Human active lists show scope and page coverage.
  Titles wrap to three lines; context shows due, project, recurrence, and full ID.
  Overview priorities use P1 highest; due labels use the printed UTC date.
  --quiet suppresses summaries; tasks and cursor notices remain. Machine output has no scope label.
  Use --all-projects or --filter to list across projects.
  --all fetches every page; --all-projects alone does not.
  inbox lists every page of Inbox tasks.
  Inbox lookup failures stop the list; they never select all projects.
  --strict belongs to top-level "todoist add", not "todoist task add".
  Aliases: ls=list, show=view, rm/del=delete.
  Completed listing supports YYYY-MM-DD, RFC3339, today/yesterday, weekday names, and "<N> days ago".
  If --completed uses --since without --until, --until defaults to today.
  For bulk actions, plain --filter text is treated as search text when not a Todoist query.
  Detailed table/--plain columns: ID, Content, Project, Section, Labels, Due, Priority, Completed.
  --wide retains the detailed table (API priorities: 4 highest); task view id:<id> shows full text.
  Human task view uses P1 highest, named context, absolute dates, recurrence, and current state.
  task view --full adds exact destination IDs and inspection metadata.
  Task view --json emits a task object; --ndjson emits the same object on one line.
  JSON/NDJSON have no enrichment; --full does not change their payloads.
  Plain/redirected task view retains labeled text.
  --ids-only on task list emits raw IDs, one per line; empty results emit nothing.
  Human output resolves project/section names; --plain uses IDs.
  Task add prints a capture receipt in a terminal (except --quiet), including a view command.
  Receipt priority matches p1..p4; numeric --priority uses API values (1 normal, 4 urgent).
  Add --dry-run shows proposed fields; Todoist must still interpret natural-language due expressions.
  Task updates/completions/deletes accept IDs or text references.
  Use --content - to read task content from stdin.
  Use id:<id> to explicitly reference a task ID.
  Task/project/label/filter references can also use Todoist app URLs.
  Task references can include due hints like "call mom today", "call mom tomorrow", or "call mom overdue".

Examples:
  todoist task list --filter "today"
  todoist task list --all-projects --all
  todoist task add --content "Pay rent" --project Home --due "1st of month"
  todoist add "Pay rent #Home p2 due:tomorrow"
  todoist task list --preset today --sort priority
  echo "From stdin" | todoist task add --content -
  todoist task view id:123456 --full
`)
}

func printProjectHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist project list [--archived] [--ids-only]
  todoist project view <id|name>
  todoist project browse <id|name>
  todoist project collaborators <id|name> [--ids-only]
  todoist project add --name <name> [flags]
  todoist project create --name <name> [flags]
  todoist project update --id <project_id> [flags]
  todoist project move <id|name> (--to-workspace <id|name> | --to-personal) [--visibility <level>] [--yes]
  todoist project archive --id <project_id>
  todoist project unarchive --id <project_id>
  todoist project delete --id <project_id>
`)
}

func printFilterHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist filter list [--ids-only]
  todoist filter show <id|name> [--ids-only]
  todoist filter add --name <name> --query <query> [--color <color>] [--favorite]
  todoist filter update <id|name> [--name <name>] [--query <query>] [--color <color>] [--favorite|--unfavorite]
  todoist filter delete <id|name> --yes
`)
}

func printWorkspaceHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist workspace list [--ids-only]
`)
}

func printSectionHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist section list [--project <id|name>] [--ids-only]
  todoist section add --name <name> --project <id|name>
  todoist section update --id <section_id> --name <name>
  todoist section delete --id <section_id>
`)
}

func printLabelHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist label list [--ids-only]
  todoist label add --name <name> [--color <color>] [--favorite]
  todoist label update --id <label_id> [flags]
  todoist label delete --id <label_id>
`)
}

func printCommentHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist comment list (--task <id> | --project <id>) [--ids-only]
  todoist comment add --content <text> (--task <id> | --project <id>)
  todoist comment update --id <comment_id> --content <text>
  todoist comment delete --id <comment_id>
`)
}

func printReminderHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist reminder list (<task> | --task <ref>) [--ids-only]
  todoist reminder add (<task> | --task <ref>) (--before <duration> | --at <datetime>)
  todoist reminder update (<id> | --id <id>) (--before <duration> | --at <datetime>)
  todoist reminder delete (<id> | --id <id>) [--yes]

Notes:
  - List and add require a task; update and delete require a reminder ID.
  - Task refs support id:<id>, text references, and Todoist task URLs.
  - --before accepts values like 30m, 1h, 2h15m.
  - --at accepts RFC3339, YYYY-MM-DD HH:MM, or YYYY-MM-DD.

Examples:
  todoist reminder list "Call mom"
  todoist reminder add "Call mom" --before 30m
  todoist reminder update id:r1 --at "2026-02-24 10:00"
  todoist reminder delete id:r1 --yes
`)
}

func printNotificationHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist notification list [--type <types>] [--unread|--read] [--limit <n>] [--offset <n>] [--ids-only]
  todoist notification view [id] [--id <id>]
  todoist notification accept [id] [--id <id>]
  todoist notification reject [id] [--id <id>]
  todoist notification read [id] [--id <id>] [--all --yes]
  todoist notification unread [id] [--id <id>]

Notes:
  - --type accepts a comma-separated list (for example: item_assigned,note_added).
  - accept/reject require share_invitation_sent notifications.
  - "read --all" requires --yes unless --force is set.

Examples:
  todoist notification list --unread
  todoist notification view id:n1
  todoist notification accept id:n1
  todoist notification reject id:n1
  todoist notification list --type item_assigned,note_added --limit 20
  todoist notification read id:n1
  todoist notification read --all --yes
  todoist notification unread id:n1
`)
}

func printAgentHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist agent plan <instruction> [--out <file>] [--planner <cmd>]
  todoist agent apply <instruction> --confirm <token> [--planner <cmd>] [--policy <file>]
  todoist agent apply --plan <file> --confirm <token>
  todoist agent apply --plan <file> --confirm <token> --dry-run [--policy <file>]
  todoist agent run --instruction <text> [--planner <cmd>] [--confirm <token>|--force] [--policy <file>]
  todoist agent schedule print --weekly "sat 09:00" [--instruction <text>] [--planner <cmd>] [--confirm <token>|--force]
  todoist agent examples
  todoist agent planner
  todoist agent status

Examples:
  todoist agent plan "Move overdue tasks to Catch Up" --out plan.json
  todoist agent apply --plan plan.json --confirm 6f2b
  todoist agent run --instruction "Triage inbox"
  todoist agent schedule print --weekly "sat 09:00" --instruction "Move 3 articles from Learning to Today"

Context flags:
  --context-project <name>   Limit planner context to project(s) (repeatable)
  --context-label <name>     Limit planner context to label(s) (repeatable)
  --context-completed <Nd>   Include completed tasks for last N days (e.g. 7d)
  --policy <file>            Enforce policy rules for planned actions

Notes:
  agent status is safe on first run and reports planner config + whether a last plan exists.
  agent apply/agent run allow no-action plans in --dry-run mode for pipeline validation.
  Planner context includes active tasks plus project/section/label/completed slices.
  Plan actions may include optional "reason" text; human previews print it when present.
`)
}

func printCompletionHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist completion bash|zsh|fish|powershell|pwsh
  todoist completion install [bash|zsh|fish|powershell|pwsh] [--path <file>]
  todoist completion uninstall [bash|zsh|fish|powershell|pwsh] [--path <file>]

Notes:
  - "install" writes the script to a user-writable path (override with --path).
  - "uninstall" removes scripts from default paths (or --path).
  - "powershell" is canonical; "pwsh" is an identical alias for PowerShell 7 on macOS and Linux.
  - PowerShell installs under $XDG_DATA_HOME, or ~/.local/share when XDG_DATA_HOME is unset.
  - PowerShell installation never edits $PROFILE; follow the printed activation instructions.
  - Without a shell argument, "install" detects PowerShell environment markers, then SHELL.
  - For zsh, run the printed activation command; add it to .zshrc for future shells.
  - Do not source the zsh completion file directly; it runs inside a completion function.
`)
}

func printAgentPlannerHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist agent planner                 # show planner command
  todoist agent planner --set --cmd "<command>"  # set planner command

Notes:
  Sources (priority): --planner flag > TODOIST_PLANNER_CMD env > config.planner_cmd > none.
`)
}

func printAgentScheduleHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist agent schedule print --weekly "sat 09:00" [--instruction <text>] [--planner <cmd>] [--confirm <token>|--force] [--cron]

Notes:
  - Default output is a macOS launchd plist. Use --cron for cron syntax.
  - --bin can override the todoist binary path for scheduling.
  - --policy <file> preserves an explicit agent policy in the generated command.
  - Profile and config selections are preserved; authorization is checked when the schedule runs.
  - Schedules contain no token. TODOIST_TOKEN still overrides the profile at execution.
`)
}
func printInboxHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist inbox [--ids-only]
  todoist inbox add --content <text> [flags]

Flags:
  --content <text>        Task content ("-" reads stdin)
  --description <text>    Task description
  --section <id|name>     Section within Inbox
  --label <name>          Label (repeatable)
  --priority <1-4>        Priority
  --due <string>          Natural language due
  --due-date <YYYY-MM-DD> Due date
  --due-datetime <RFC3339> Due date/time
  --due-lang <code>       Due language
  --duration <minutes>    Duration
  --duration-unit <unit>  Duration unit (minute/day)
  --deadline <YYYY-MM-DD> Deadline date
  --assignee <id>         Assignee ID

Notes:
  - Lists all pages of active Inbox tasks, regardless of due date.
  - Human output shows scope, coverage, and titles wrapping to three lines.
  - Overview P1 is highest; due labels use the printed UTC date without changing selection.
  - Use task view id:<id> for full text and secondary details.
  - Stops with an error if Inbox cannot be resolved.
  - Uses Inbox project automatically.
  - A terminal capture receipt shows returned task details and a view command (except --quiet).
  - Receipt priority uses Todoist 1..4; numeric --priority uses API values (1 normal, 4 urgent).
  - --dry-run shows proposed fields, not a saved task; due expressions still need Todoist interpretation.
  - Applies default labels/due from config (default_inbox_labels, default_inbox_due) when not set.
  - Use --content - to read task content from stdin.
  - Positional text is accepted when --content is omitted.
`)
}

func printAddHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist add <text> [flags]

Notes:
  - Default lets Todoist interpret project, labels, priority, and natural-language dates.
  - Use --strict for literal content and explicit creation fields.
  - Quick add does not support --section or project IDs; use --strict for those.
  - In --strict mode, pass --project as a name/id (no "#"), --label as names (no "@"), and --due without "due:".
  - If --content is omitted, remaining args are treated as task content.
  - A terminal receipt shows returned content, destination, due, recurrence, priority, labels, and a view command.
  - Receipt priority matches p1..p4 (p2 shows Priority: 2); numeric --priority uses API values (1 normal, 4 urgent).
  - Missing response fields show Not returned; an explicit empty due shows No due date.
  - --dry-run shows submitted text/fields, not confirmed interpretation; no task is created.
  - --strict leaves content literal; --due expressions still need Todoist interpretation.
  - JSON/NDJSON/plain and --quiet output are unchanged; --ids-only rejects creation.
  - Correct fields with task update; change destination with task move. Use either command's --help.

Examples:
  todoist add "Pay rent"
  todoist add "Pay rent #Home p2 due:tomorrow"
  todoist add "Buy milk tomorrow p1 #Home @errands"
  todoist add --content - --strict
  echo "From stdin" | todoist add --content -
`)
}

func printAuthStorageHelp(w interface{ Write([]byte) (int, error) }, operation string) {
	if operation == "migrate" {
		fmt.Fprintln(w, `Usage: todoist auth migrate --credential-store=native|file

Move the selected profile after verifying destination storage.
Native migration removes its plaintext token. No automatic fallback occurs.
Older CLI versions cannot use native profiles; explicitly migrate to file first.

Options:
  --credential-store native|file  Required destination backend
  -h, --help                      Show help`)
	} else {
		fmt.Fprintln(w, `Usage: todoist auth repair

Recover the selected profile's interrupted credential transaction or retry cleanup.
Does not reconstruct corrupt files or access another directory's native entries.

Options:
  -h, --help  Show help`)
	}
}
