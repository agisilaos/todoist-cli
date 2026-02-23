package cli

import "fmt"

func completedCommand(ctx *Context, args []string) error {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		printCompletedHelp(ctx.Stdout)
		return nil
	}
	for _, arg := range args {
		if arg == "--completed" {
			return taskList(ctx, args)
		}
	}
	listArgs := make([]string, 0, len(args)+1)
	listArgs = append(listArgs, "--completed")
	listArgs = append(listArgs, args...)
	return taskList(ctx, listArgs)
}

func printCompletedHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, "Usage:\n  todoist completed [--completed-by completion|due] [--since <date>] [--until <date>] [--project <id|name>] [--section <id|name>] [--filter <query>] [--json|--plain|--ndjson]\n\nNotes:\n  - Shortcut for: todoist task list --completed ...\n  - If --since is set without --until, --until defaults to today.\n\nExamples:\n  todoist completed\n  todoist completed --since \"2 weeks ago\" --completed-by due\n  todoist completed --project Home --json\n")
}
