package cli

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func taskList(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("task list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var filter string
	var project string
	var section string
	var parent string
	var label string
	var ids string
	var cursor string
	var limit int
	var all bool
	var allProjects bool
	var completed bool
	var completedBy string
	var since string
	var until string
	var wide bool
	var preset string
	var sortBy string
	var truncateWidth int
	var help bool
	fs.StringVar(&filter, "filter", "", "Filter query")
	fs.StringVar(&project, "project", "", "Project")
	fs.StringVar(&section, "section", "", "Section")
	fs.StringVar(&parent, "parent", "", "Parent task")
	fs.StringVar(&label, "label", "", "Label")
	fs.StringVar(&ids, "id", "", "Comma-separated task IDs")
	fs.StringVar(&cursor, "cursor", "", "Cursor")
	fs.IntVar(&limit, "limit", 50, "Limit")
	fs.BoolVar(&all, "all", false, "Fetch all pages")
	fs.BoolVar(&allProjects, "all-projects", false, "List tasks from all projects")
	fs.BoolVar(&completed, "completed", false, "List completed tasks")
	fs.StringVar(&completedBy, "completed-by", "completion", "completed or due")
	fs.StringVar(&since, "since", "", "Start date (RFC3339 or YYYY-MM-DD)")
	fs.StringVar(&until, "until", "", "End date (RFC3339 or YYYY-MM-DD)")
	fs.BoolVar(&wide, "wide", false, "Wider table output")
	fs.StringVar(&preset, "preset", "", "Shortcut filter: today, overdue, next7")
	fs.StringVar(&sortBy, "sort", "", "Sort by: due, priority")
	fs.IntVar(&truncateWidth, "truncate-width", 0, "Override table width (human output)")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if completed {
		return taskListCompleted(ctx, completedBy, filter, project, section, parent, since, until, cursor, limit, all, wide)
	}
	if filter == "" && preset != "" {
		switch preset {
		case "today":
			filter = "today"
		case "overdue":
			filter = "overdue"
		case "next7":
			filter = "next 7 days"
		default:
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("invalid preset: %s", preset)}
		}
	}
	if truncateWidth > 0 {
		ctx.Config.TableWidth = truncateWidth
	}
	if filter != "" {
		return taskListFiltered(ctx, filter, cursor, limit, all, wide)
	}
	return taskListActive(ctx, project, section, parent, label, ids, cursor, limit, all, allProjects, wide, sortBy)
}

func taskListActive(ctx *Context, project, section, parent, label, ids, cursor string, limit int, all bool, allProjects bool, wide bool, sortBy string) error {
	query := url.Values{}
	if project == "" && section == "" && parent == "" && label == "" && ids == "" && !allProjects {
		id, err := inboxProjectID(ctx)
		if err == nil && id != "" {
			project = id
		}
	}
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err != nil {
			return err
		}
		query.Set("project_id", id)
	}
	if section != "" {
		id, err := resolveSectionID(ctx, section, project)
		if err != nil {
			return err
		}
		query.Set("section_id", id)
	}
	if parent != "" {
		query.Set("parent_id", parent)
	}
	if label != "" {
		name, err := resolveLabelName(ctx, label)
		if err != nil {
			return err
		}
		query.Set("label", name)
	}
	if ids != "" {
		query.Set("ids", ids)
	}
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	allTasks, next, err := fetchPaginated[api.Task](ctx, "/tasks", query, all)
	if err != nil {
		return err
	}
	sortTasks(allTasks, sortBy)
	return writeTaskList(ctx, allTasks, next, wide)
}

func taskListFiltered(ctx *Context, filter, cursor string, limit int, all bool, wide bool) error {
	allTasks, next, err := listTasksByFilter(ctx, filter, cursor, limit, all)
	if err != nil {
		return err
	}
	// Keep original ordering from API for filter; no client sort to preserve meaning.
	return writeTaskList(ctx, allTasks, next, wide)
}

func listTasksByFilter(ctx *Context, filter, cursor string, limit int, all bool) ([]api.Task, string, error) {
	query := url.Values{}
	query.Set("query", filter)
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	return fetchPaginated[api.Task](ctx, "/tasks/filter", query, all)
}

func taskListCompleted(ctx *Context, completedBy, filter, project, section, parent, since, until, cursor string, limit int, all bool, wide bool) error {
	path := "/tasks/completed/by_completion_date"
	if completedBy == "due" {
		path = "/tasks/completed/by_due_date"
	}
	query := url.Values{}
	if since != "" {
		query.Set("since", since)
	}
	if until != "" {
		query.Set("until", until)
	}
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err != nil {
			return err
		}
		query.Set("project_id", id)
	}
	if section != "" {
		id, err := resolveSectionID(ctx, section, project)
		if err != nil {
			return err
		}
		query.Set("section_id", id)
	}
	if parent != "" {
		query.Set("parent_id", parent)
	}
	if filter != "" {
		query.Set("filter_query", filter)
	}
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	allTasks, next, err := fetchPaginated[api.Task](ctx, path, query, all)
	if err != nil {
		return err
	}
	return writeTaskList(ctx, allTasks, next, wide)
}
