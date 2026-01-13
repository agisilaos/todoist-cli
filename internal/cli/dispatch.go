package cli

import "fmt"

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
	case "section":
		err = sectionCommand(ctx, rest)
	case "label":
		err = labelCommand(ctx, rest)
	case "comment":
		err = commentCommand(ctx, rest)
	case "agent":
		err = agentCommand(ctx, rest)
	case "completion":
		err = completionCommand(ctx, rest)
	case "inbox":
		err = inboxCommand(ctx, rest)
	case "schema":
		err = schemaCommand(ctx, rest)
	case "add":
		err = taskAdd(ctx, rest)
	case "help":
		err = helpCommand(ctx, rest)
	default:
		fmt.Fprintf(ctx.Stderr, "unknown command: %s\n", cmd)
		printRootHelp(ctx.Stderr)
		return exitUsage
	}
	if err != nil {
		writeError(ctx, err)
	}
	return toExitCode(err)
}
