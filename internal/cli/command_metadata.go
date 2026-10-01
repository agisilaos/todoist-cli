package cli

import (
	"fmt"
	"sort"
	"strings"
)

// commandMetadata owns discovery only. Execution switches and flag parsers remain
// authoritative and are checked independently by the dispatch contract test.
// Slice order is completion order; rootHelpOrder preserves the editorial help order.
type commandMetadata struct {
	path          string
	aliases       string
	group         bool
	summary       string
	section       string
	rootHelpOrder int
}

var commandCatalog = []commandMetadata{
	{path: "review", summary: "Guided daily task review", section: "Everyday", rootHelpOrder: 3},
	{path: "today", summary: "Tasks due today and overdue across projects", section: "Everyday", rootHelpOrder: 2},
	{path: "completed", summary: "Completed task history", section: "Organization", rootHelpOrder: 15},
	{path: "upcoming", summary: "Tasks due today and the next 6 days (default)", section: "Everyday", rootHelpOrder: 4},
	{path: "inbox", group: true, summary: "List Inbox tasks or add to Inbox", section: "Everyday", rootHelpOrder: 5},
	{path: "inbox add"},
	{path: "add", summary: "Capture tasks with natural language parsing", section: "Everyday", rootHelpOrder: 1},
	{path: "auth", group: true, summary: "Authenticate and manage tokens", section: "Setup and reference", rootHelpOrder: 22},
	{path: "auth login"},
	{path: "auth status"},
	{path: "auth logout"},
	{path: "auth migrate"},
	{path: "auth repair"},
	{path: "profile", group: true, summary: "List, inspect, select, or remove credential profiles", section: "Setup and reference", rootHelpOrder: 27},
	{path: "profile list"},
	{path: "profile current"},
	{path: "profile use"},
	{path: "profile remove"},
	{path: "task", group: true, summary: "Manage tasks (list defaults to Inbox)", section: "Organization", rootHelpOrder: 6},
	{path: "task list", aliases: "ls"},
	{path: "task add"},
	{path: "task view", aliases: "show"},
	{path: "task update"},
	{path: "task move"},
	{path: "task complete"},
	{path: "task reopen"},
	{path: "task delete", aliases: "rm del"},
	{path: "filter", group: true, summary: "Manage filters", section: "Organization", rootHelpOrder: 11},
	{path: "filter list", aliases: "ls"},
	{path: "filter show"},
	{path: "filter add"},
	{path: "filter update"},
	{path: "filter delete", aliases: "rm del"},
	{path: "project", group: true, summary: "Manage projects", section: "Organization", rootHelpOrder: 7},
	{path: "project list", aliases: "ls"},
	{path: "project view", aliases: "show"},
	{path: "project browse"},
	{path: "project collaborators"},
	{path: "project add", aliases: "create"},
	{path: "project update"},
	{path: "project move"},
	{path: "project archive"},
	{path: "project unarchive"},
	{path: "project delete", aliases: "rm del"},
	{path: "workspace", group: true, summary: "Manage workspaces", section: "Organization", rootHelpOrder: 8},
	{path: "workspace list", aliases: "ls"},
	{path: "section", group: true, summary: "Manage sections", section: "Organization", rootHelpOrder: 9},
	{path: "section list", aliases: "ls"},
	{path: "section add"},
	{path: "section update"},
	{path: "section delete", aliases: "rm del"},
	{path: "label", group: true, summary: "Manage labels", section: "Organization", rootHelpOrder: 10},
	{path: "label list", aliases: "ls"},
	{path: "label add"},
	{path: "label update"},
	{path: "label delete", aliases: "rm del"},
	{path: "comment", group: true, summary: "Manage comments", section: "Organization", rootHelpOrder: 12},
	{path: "comment list", aliases: "ls"},
	{path: "comment add"},
	{path: "comment update"},
	{path: "comment delete", aliases: "rm del"},
	{path: "reminder", group: true, summary: "Manage task reminders", section: "Organization", rootHelpOrder: 13},
	{path: "reminder list", aliases: "ls"},
	{path: "reminder add"},
	{path: "reminder update"},
	{path: "reminder delete", aliases: "rm del"},
	{path: "notification", group: true, summary: "Manage notifications", section: "Organization", rootHelpOrder: 14},
	{path: "notification list", aliases: "ls"},
	{path: "notification view"},
	{path: "notification accept"},
	{path: "notification reject"},
	{path: "notification read"},
	{path: "notification unread"},
	{path: "activity", summary: "View activity logs", section: "Organization", rootHelpOrder: 16},
	{path: "stats", group: true, summary: "View productivity stats", section: "Organization", rootHelpOrder: 17},
	{path: "stats goals"},
	{path: "stats vacation"},
	{path: "settings", group: true, summary: "Manage user settings", section: "Setup and reference", rootHelpOrder: 23},
	{path: "settings view"},
	{path: "settings update"},
	{path: "settings themes"},
	{path: "view", summary: "Open Todoist web URLs in CLI", section: "Organization", rootHelpOrder: 18},
	{path: "agent", group: true, summary: "Plan and apply agentic actions", section: "Automation", rootHelpOrder: 19},
	{path: "agent plan"},
	{path: "agent apply"},
	{path: "agent run"},
	{path: "agent schedule", group: true},
	{path: "agent schedule print"},
	{path: "agent examples"},
	{path: "agent planner"},
	{path: "agent status"},
	{path: "skill", group: true, summary: "Install and maintain coding-agent skills", section: "Setup and reference", rootHelpOrder: 28},
	{path: "skill install"},
	{path: "skill list"},
	{path: "skill update"},
	{path: "skill uninstall"},
	{path: "completion", group: true, summary: "Shell completion", section: "Setup and reference", rootHelpOrder: 24},
	{path: "completion bash"},
	{path: "completion zsh"},
	{path: "completion fish"},
	{path: "completion powershell", aliases: "pwsh"},
	{path: "completion install"},
	{path: "completion uninstall"},
	{path: "doctor", summary: "Run environment and configuration checks", section: "Setup and reference", rootHelpOrder: 25},
	{path: "schema", summary: "Show output schemas and wire-format contracts", section: "Automation", rootHelpOrder: 21},
	{path: "planner", summary: "Show or set planner command", section: "Automation", rootHelpOrder: 20},
	{path: "help", summary: "Show help for a command", section: "Setup and reference", rootHelpOrder: 26},
}

func commandParentAndName(path string) (string, string) {
	if i := strings.LastIndexByte(path, ' '); i >= 0 {
		return path[:i], path[i+1:]
	}
	return "", path
}

func commandNames(parent string) []string {
	var names []string
	for _, command := range commandCatalog {
		p, name := commandParentAndName(command.path)
		if p == parent {
			names = append(names, name)
			names = append(names, strings.Fields(command.aliases)...)
		}
	}
	return names
}

func buildCommandHelpCatalog() map[string]commandHelp {
	pages := make(map[string]commandHelp, len(commandCatalog))
	for _, command := range commandCatalog {
		page := leafHelpPages[command.path]
		page.aliases, page.group = command.aliases, command.group
		pages[command.path] = page
	}
	return pages
}

var commandHelpCatalog = buildCommandHelpCatalog()

func rootCommandListing() string {
	var roots []commandMetadata
	for _, command := range commandCatalog {
		if parent, _ := commandParentAndName(command.path); parent == "" {
			roots = append(roots, command)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].rootHelpOrder < roots[j].rootHelpOrder })
	var out strings.Builder
	section := ""
	for _, command := range roots {
		if command.section != section {
			if section != "" {
				out.WriteByte('\n')
			}
			section = command.section
			fmt.Fprintf(&out, "%s:\n", section)
		}
		fmt.Fprintf(&out, "  %-14s%s\n", command.path, command.summary)
	}
	return out.String()
}
