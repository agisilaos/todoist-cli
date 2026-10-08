package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func upcomingCommand(ctx *Context, args []string) error {
	fs := newFlagSet("upcoming")
	var days int
	var project string
	var label string
	var wide bool
	var sorting taskSortOptions
	var truncateWidth int
	var help bool
	fs.IntVar(&days, "days", 0, "Days to include (default 7)")
	fs.StringVar(&project, "project", "", "Project")
	fs.StringVar(&label, "label", "", "Label")
	fs.BoolVar(&wide, "wide", false, "Wider table output")
	sorting.bind(fs)
	fs.IntVar(&truncateWidth, "truncate-width", 0, "Override table width (human output)")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printUpcomingHelp(ctx.Stdout)
		return nil
	}
	if days == 0 {
		days = 7
	}
	if len(fs.Args()) > 1 {
		if ctx.Mode != output.ModeIDsOnly {
			printUpcomingHelp(ctx.Stderr)
		}
		return &CodeError{Code: exitUsage, Err: errors.New("upcoming accepts at most one positional [days] argument")}
	}
	if len(fs.Args()) == 1 {
		parsed, err := strconv.Atoi(strings.TrimSpace(fs.Arg(0)))
		if err != nil {
			return &CodeError{Code: exitUsage, Err: errors.New("upcoming days must be a positive integer")}
		}
		days = parsed
	}
	if days <= 0 {
		return &CodeError{Code: exitUsage, Err: errors.New("upcoming days must be a positive integer")}
	}
	if err := sorting.validate(); err != nil {
		return err
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if truncateWidth > 0 {
		ctx.Config.TableWidth = truncateWidth
	}
	tasks, now, err := listUpcomingTasks(ctx, days, project, label)
	if err != nil {
		return err
	}
	if sorting.key == "" {
		sortTasks(tasks, "due")
	} else if err := sorting.apply(tasks); err != nil {
		return err
	}
	start := now.Format("2006-01-02")
	end := now.AddDate(0, 0, days-1).Format("2006-01-02")
	return writeTaskOverview(ctx, tasks, "", wide, taskOverview{
		Scope:         "Upcoming · " + start + " to " + end + " (UTC) · " + activeTaskScope(project, "", "", label, ""),
		Empty:         "No tasks due in this UTC window for this selection.",
		ReferenceDate: start,
	})
}

func listUpcomingTasks(ctx *Context, days int, project, label string) ([]api.Task, time.Time, error) {
	query := url.Values{}
	query.Set("limit", "200")
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err != nil {
			return nil, time.Time{}, err
		}
		query.Set("project_id", id)
	}
	if label != "" {
		name, err := resolveLabelName(ctx, label)
		if err != nil {
			return nil, time.Time{}, err
		}
		query.Set("label", name)
	}
	allTasks, _, err := fetchPaginated[api.Task](ctx, "/tasks", query, true)
	if err != nil {
		return nil, time.Time{}, err
	}
	now := time.Now().UTC()
	if ctx != nil && ctx.Now != nil {
		now = ctx.Now().UTC()
	}
	return filterUpcomingTasks(allTasks, now, days), now, nil
}

func filterUpcomingTasks(tasks []api.Task, now time.Time, days int) []api.Task {
	start := now.UTC().Format("2006-01-02")
	end := now.UTC().AddDate(0, 0, days-1).Format("2006-01-02")
	out := make([]api.Task, 0, len(tasks))
	for _, task := range tasks {
		date := taskDueDate(task)
		if date == "" {
			continue
		}
		if date >= start && date <= end {
			out = append(out, task)
		}
	}
	return out
}

func printUpcomingHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, "Usage:\n  todoist upcoming [days] [--project <id|name>] [--label <name>] [--sort due|deadline|priority|added|updated|completed|content|order|none] [--sort-order asc|desc] [--wide] [--ids-only]\n\nNotes:\n  - Shows N days including today (default 7: today and the next 6 days, UTC).\n  - Excludes overdue tasks and tasks without a due date.\n  - Default ordering is by due date and time, including timed due dates.\n  - Human output shows the UTC window, scope, coverage, and three-line titles.\n  - Overview P1 is highest; --wide retains the detailed table (API priorities: 4 highest).\n  - Use task view id:<id> for full text and secondary details.\n\nExamples:\n  todoist upcoming\n  todoist upcoming 14 --project Learning\n  todoist upcoming --label reading --sort priority\n")
}
