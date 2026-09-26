package cli

import (
	"fmt"
)

func todayCommand(ctx *Context, args []string) error {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		printTodayHelp(ctx.Stdout)
		return nil
	}
	filter := "overdue | today"
	if err := ensureClient(ctx); err != nil {
		return err
	}
	return taskListFiltered(ctx, filter, "", 50, true, false)
}

func printTodayHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, "Usage:\n  todoist today [--ids-only]\n\nNotes:\n  - Shows tasks due today and overdue (across all projects).\n")
}
