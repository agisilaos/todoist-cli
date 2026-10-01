package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	apprefs "github.com/agisilaos/todoist-cli/internal/app/refs"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func parseTaskOutputVersion(value string) (int, error) {
	switch value {
	case "1":
		return 1, nil
	case "2":
		return 2, nil
	default:
		return 0, fmt.Errorf("--task-output-version requires 1 (legacy) or 2 (faithful), got %q", value)
	}
}

func validateTaskOutputSelection(ctx *Context, args []string) error {
	if !ctx.Global.TaskOutputVersionSet {
		return nil
	}
	if ctx.Mode != output.ModeJSON && ctx.Mode != output.ModeNDJSON {
		return fmt.Errorf("--task-output-version requires --json or --ndjson")
	}
	if ctx.Global.DryRun {
		return fmt.Errorf("--task-output-version selects returned tasks and cannot be used with --dry-run")
	}
	if !taskResourceCommand(args) {
		return fmt.Errorf("--task-output-version is only supported by task-resource commands; see 'todoist help task view'")
	}
	return nil
}

// Classify commands without performing lookups, reading stdin, or creating files.
func taskResourceCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "today", "upcoming", "completed", "add":
		return true
	case "inbox":
		return len(args) == 1 || args[1] == "add"
	case "task":
		if len(args) > 1 {
			switch args[1] {
			case "list", "ls", "view", "show", "add", "update":
				return true
			}
		}
	case "filter":
		return len(args) > 1 && args[1] == "show"
	case "view":
		raw, _, err := parseViewArgs(args[1:])
		return err == nil && taskResourceURL(raw)
	}
	return false
}

func taskResourceURL(raw string) bool {
	if entity, ok := apprefs.ParseTodoistEntityURL(raw); ok {
		return entity.Entity == "task" || entity.Entity == "filter" || entity.Entity == "label"
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.") != "app.todoist.com" {
		return false
	}
	switch strings.Trim(parsed.Path, "/") {
	case "app/inbox", "app/today", "app/upcoming", "app/completed":
		return true
	}
	return false
}

func taskResourceValue(ctx *Context, task api.Task) any {
	if ctx.Global.TaskOutputVersion == 2 {
		return task.FaithfulResource()
	}
	return task
}

func writeTaskResponseIssues(ctx *Context, tasks []api.Task) error {
	if ctx.Global.TaskOutputVersion != 2 {
		return nil
	}
	for index, task := range tasks {
		for _, issue := range task.ResponseIssues() {
			if _, err := fmt.Fprintf(ctx.Stderr, "Task result %d: %s unavailable (expected %s, returned %s); malformed fact omitted.\n", index+1, issue.Path, issue.Expected, issue.Actual); err != nil {
				return err
			}
		}
	}
	return nil
}
