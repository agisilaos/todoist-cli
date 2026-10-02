package cli

import (
	"fmt"
)

func todayCommand(ctx *Context, args []string) error {
	fs := newFlagSet("today")
	var help bool
	var sorting taskSortOptions
	sorting.bind(fs)
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTodayHelp(ctx.Stdout)
		return nil
	}
	if len(fs.Args()) != 0 {
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("today accepts no positional arguments")}
	}
	if err := sorting.validate(); err != nil {
		return err
	}
	filter := "overdue | today"
	if err := ensureClient(ctx); err != nil {
		return err
	}
	return taskListFiltered(ctx, filter, "", 200, true, false, taskOverview{
		Scope: "Today · Across projects",
		Empty: "No overdue or due-today tasks returned.",
	}, sorting)
}

func printTodayHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, "Usage:\n  todoist today [--ids-only] [--sort <key>] [--sort-order asc|desc]\n\nNotes:\n  - Shows tasks due today and overdue (across all projects).\n  - Accepts task sorting and global flags; use task list for custom filtering or limits.\n  - Human output shows scope, coverage, and titles wrapping to three lines.\n  - Overview P1 is highest; relative due labels use the printed UTC calendar date.\n  - Todoist still selects tasks; near midnight its Today selection may differ from UTC labels.\n  - Use task view id:<id> for full text, or task list --filter \"overdue | today\" --all --wide for the detailed table.\n")
}
