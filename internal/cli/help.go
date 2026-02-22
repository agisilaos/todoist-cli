package cli

import "fmt"

func printRootHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `todoist - Agentic Todoist CLI

Usage:
  todoist [global flags] <command> [args]

Commands:
  inbox       Quick-add to Inbox
  add         Quick add (alias of task add)
  today       Tasks due today and overdue
  auth        Authenticate and manage tokens
  task        Manage tasks
  filter      Manage filters
  project     Manage projects
  workspace   Manage workspaces
  section     Manage sections
  label       Manage labels
  comment     Manage comments
  agent       Plan and apply agentic actions
  completion  Shell completion
  doctor      Run environment and configuration checks
  schema      Show JSON schemas for outputs
  planner     Show or set planner command
  help        Show help for a command

Global flags:
  -h, --help            Show help
  --version             Show version
  -q, --quiet           Suppress non-essential output
  --quiet-json          Compact single-line JSON errors
  -v, --verbose         Enable verbose output
  --json                JSON output
  --plain               Plain text output (tab-separated)
  --ndjson              NDJSON output
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
  todoist task list
  todoist task list --all-projects
  todoist task add --content "Pay rent" --project Home --due "1st of month"
  todoist help task
  todoist inbox add --content "Capture idea"
`)
}

func helpCommand(ctx *Context, args []string) error {
	if len(args) == 0 {
		printRootHelp(ctx.Stdout)
		return nil
	}
	switch args[0] {
	case "auth":
		printAuthHelp(ctx.Stdout)
	case "add":
		printAddHelp(ctx.Stdout)
	case "today":
		printTodayHelp(ctx.Stdout)
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
	case "examples":
		_ = agentExamples(ctx)
	default:
		printRootHelp(ctx.Stdout)
	}
	return nil
}

func printAuthHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist auth login [--token-stdin] [--print-env]
  todoist auth login --oauth [--client-id <id>] [--no-browser] [--print-env]
  todoist auth login --oauth-device [--client-id <id>] [--print-env]
  todoist auth status
  todoist auth logout

Examples:
  todoist auth login
  todoist auth login --token-stdin < token.txt
  todoist auth login --oauth --client-id "$TODOIST_OAUTH_CLIENT_ID"
  todoist auth login --oauth-device --client-id "$TODOIST_OAUTH_CLIENT_ID"
  todoist auth login --oauth --no-browser
  todoist auth login --print-env
`)
}

func printAuthLoginHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist auth login [--token-stdin] [--print-env]
  todoist auth login --oauth [--client-id <id>] [--no-browser] [--print-env]
  todoist auth login --oauth-device [--client-id <id>] [--print-env]
                    [--oauth-authorize-url <url>] [--oauth-token-url <url>]
                    [--oauth-device-url <url>]
                    [--oauth-listen <host:port>] [--oauth-redirect-uri <uri>]

Flags:
  --token-stdin                Read token from stdin
  --print-env                  Print token export instead of saving profile credentials
  --oauth                      Authenticate using OAuth PKCE flow
  --oauth-device               Authenticate using OAuth device flow (headless-friendly)
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
  todoist auth login --oauth-device --client-id "$TODOIST_OAUTH_CLIENT_ID"
  todoist auth login --oauth --no-browser --print-env
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
  todoist task delete <ref> [--yes]

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
  --yes                      Skip delete confirmation

Notes:
  By default, todoist task list shows Inbox tasks. Use --all-projects or --filter to list across projects.
  --strict belongs to top-level "todoist add", not "todoist task add".
  Aliases: ls=list, show=view, rm/delete=delete.
  Completed listing supports YYYY-MM-DD, RFC3339, today/yesterday, weekday names, and "<N> days ago".
  If --completed uses --since without --until, --until defaults to today.
  For bulk actions, plain --filter text is treated as search text when not a Todoist query.
  Output columns (human/--plain): ID, Content, Project, Section, Labels, Due, Priority, Completed.
  Human output resolves project/section names; --plain uses IDs.
  Task updates/completions/deletes require task IDs; projects/sections/labels resolve names.
  Use --content - to read task content from stdin.
  Use id:<id> to explicitly reference a task ID.
  Task references can include due hints like "call mom today", "call mom tomorrow", or "call mom overdue".

Examples:
  todoist task list --filter "today"
  todoist task list --all-projects
  todoist task add --content "Pay rent" --project Home --due "1st of month"
  todoist add "Pay rent #Home p2 due:tomorrow"
  todoist task list --preset today --sort priority
  echo "From stdin" | todoist task add --content -
  todoist task view id:123456 --full
`)
}

func printProjectHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist project list [--archived]
  todoist project collaborators <id|name>
  todoist project add --name <name> [flags]
  todoist project update --id <project_id> [flags]
  todoist project archive --id <project_id>
  todoist project unarchive --id <project_id>
  todoist project delete --id <project_id>
`)
}

func printFilterHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist filter list
  todoist filter show <id|name>
  todoist filter add --name <name> --query <query> [--color <color>] [--favorite]
  todoist filter update <id|name> [--name <name>] [--query <query>] [--color <color>] [--favorite|--unfavorite]
  todoist filter delete <id|name> --yes
`)
}

func printWorkspaceHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist workspace list
`)
}

func printSectionHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist section list [--project <id|name>]
  todoist section add --name <name> --project <id|name>
  todoist section update --id <section_id> --name <name>
  todoist section delete --id <section_id>
`)
}

func printLabelHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist label list
  todoist label add --name <name> [--color <color>] [--favorite]
  todoist label update --id <label_id> [flags]
  todoist label delete --id <label_id>
`)
}

func printCommentHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist comment list --task <id> | --project <id>
  todoist comment add --content <text> (--task <id> | --project <id>)
  todoist comment update --id <comment_id> --content <text>
  todoist comment delete --id <comment_id>
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
  todoist completion bash|zsh|fish
  todoist completion install [bash|zsh|fish] [--path <file>]

Notes:
  - "install" writes the script to a user-writable path (override with --path).
  - Without a shell argument, "install" tries to detect SHELL.
  - For zsh, ensure the install path directory is in $fpath.
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
`)
}
func printInboxHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
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
  - Uses Inbox project automatically.
  - Applies default labels/due from config (default_inbox_labels, default_inbox_due) when not set.
  - Use --content - to read task content from stdin.
  - Positional text is accepted when --content is omitted.
`)
}

func printAddHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist add <text> [flags]

Notes:
  - Default uses Sync API quick add (full natural language parsing).
  - Use --strict to disable parsing and use the REST add endpoint.
  - Quick add does not support --section or project IDs; use --strict for those.
  - In --strict mode, pass --project as a name/id (no "#"), --label as names (no "@"), and --due without "due:".
  - If --content is omitted, remaining args are treated as task content.

Examples:
  todoist add "Pay rent"
  todoist add "Pay rent #Home p2 due:tomorrow"
  todoist add "Buy milk tomorrow p1 #Home @errands"
  todoist add --content - --strict
  echo "From stdin" | todoist add --content -
`)
}
