package cli

import (
	"fmt"

	"github.com/agisilaos/todoist-cli/internal/output"
)

func dispatch(ctx *Context, args []string) int {
	cmd := args[0]
	rest := args[1:]
	var err error
	switch cmd {
	case "auth":
		err = authCommand(ctx, rest)
	case "task":
		err = taskCommand(ctx, rest)
	case "project":
		err = projectCommand(ctx, rest)
	case "filter":
		err = filterCommand(ctx, rest)
	case "workspace":
		err = workspaceCommand(ctx, rest)
	case "section":
		err = sectionCommand(ctx, rest)
	case "label":
		err = labelCommand(ctx, rest)
	case "comment":
		err = commentCommand(ctx, rest)
	case "reminder":
		err = reminderCommand(ctx, rest)
	case "notification":
		err = notificationCommand(ctx, rest)
	case "activity":
		err = activityCommand(ctx, rest)
	case "stats":
		err = statsCommand(ctx, rest)
	case "agent":
		err = agentCommand(ctx, rest)
	case "completion":
		err = completionCommand(ctx, rest)
	case "doctor":
		err = doctorCommand(ctx, rest)
	case "inbox":
		err = inboxCommand(ctx, rest)
	case "schema":
		err = schemaCommand(ctx, rest)
	case "planner":
		err = agentPlanner(ctx, rest)
	case "add":
		err = quickAddCommand(ctx, rest)
	case "today":
		err = todayCommand(ctx, rest)
	case "completed":
		err = completedCommand(ctx, rest)
	case "upcoming":
		err = upcomingCommand(ctx, rest)
	case "help":
		err = helpCommand(ctx, rest)
	default:
		err = &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown command: %s", cmd)}
		if ctx.Mode == output.ModeJSON {
			writeError(ctx, err)
			return exitUsage
		}
		fmt.Fprintf(ctx.Stderr, "unknown command: %s\n", cmd)
		printRootHelp(ctx.Stderr)
		return exitUsage
	}
	if err != nil {
		writeError(ctx, err)
	}
	return toExitCode(err)
}
