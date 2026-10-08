package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	apptasks "github.com/agisilaos/todoist-cli/internal/app/tasks"
)

func taskList(ctx *Context, args []string) error {
	fs := newFlagSet("task list")
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
	var sorting taskSortOptions
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
	sorting.bind(fs)
	fs.IntVar(&truncateWidth, "truncate-width", 0, "Override table width (human output)")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	if err := sorting.validate(); err != nil {
		return err
	}
	now := time.Now
	if ctx != nil && ctx.Now != nil {
		now = ctx.Now
	}
	plan, err := apptasks.PlanList(now(), apptasks.ListInput{
		Filter:      filter,
		Preset:      preset,
		Completed:   completed,
		CompletedBy: completedBy,
		Since:       since,
		Until:       until,
	})
	if err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if plan.Mode == "filter" {
		for _, selector := range []struct{ name, value string }{
			{"project", project}, {"section", section}, {"parent", parent}, {"label", label}, {"id", ids},
		} {
			if selector.value != "" {
				return &CodeError{Code: exitUsage, Err: fmt.Errorf("--%s cannot be combined with an active --filter or --preset; express the selection in the filter query", selector.name)}
			}
		}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if truncateWidth > 0 {
		ctx.Config.TableWidth = truncateWidth
	}
	if plan.Mode == "completed" {
		return taskListCompleted(ctx, plan.CompletedBy, plan.Filter, project, section, parent, plan.Since, plan.Until, cursor, limit, all, wide, sorting)
	}
	if plan.Mode == "filter" {
		return taskListFiltered(ctx, plan.Filter, cursor, limit, all, wide, taskOverview{
			Scope: "Filter: " + strconv.Quote(plan.Filter) + " · Active tasks",
			Empty: "No active tasks returned for this filter.",
		}, sorting)
	}
	return taskListActive(ctx, project, section, parent, label, ids, cursor, limit, all, allProjects, wide, sorting)
}

func taskListActive(ctx *Context, project, section, parent, label, ids, cursor string, limit int, all bool, allProjects bool, wide bool, sorting taskSortOptions) error {
	query := url.Values{}
	scope := activeTaskScope(project, section, parent, label, ids)
	empty := "No active tasks returned for this selection."
	if project == "" && section == "" && parent == "" && label == "" && ids == "" {
		empty = "No active tasks returned across projects."
	}
	if project == "" && section == "" && parent == "" && label == "" && ids == "" && !allProjects {
		id, err := inboxProjectID(ctx)
		if err != nil {
			return fmt.Errorf("cannot resolve Inbox project: %w", err)
		}
		if id == "" {
			return &CodeError{Code: exitNotFound, Err: errors.New("Inbox project not found; run 'todoist project list' to check available projects")}
		}
		project = id
		scope = "Inbox · Active tasks"
		empty = "No active tasks in Inbox."
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
	if err := sorting.apply(allTasks); err != nil {
		return err
	}
	return writeTaskOverview(ctx, allTasks, next, wide, taskOverview{Scope: scope, Empty: empty, Continued: cursor != ""})
}

func taskListFiltered(ctx *Context, filter, cursor string, limit int, all bool, wide bool, view taskOverview, sorting taskSortOptions) error {
	allTasks, next, err := listTasksByFilter(ctx, filter, cursor, limit, all)
	if err != nil {
		return err
	}
	// Omitted sorting preserves the filter response order.
	view.Continued = cursor != ""
	if err := sorting.apply(allTasks); err != nil {
		return err
	}
	return writeTaskOverview(ctx, allTasks, next, wide, view)
}

func listTasksByFilter(ctx *Context, filter, cursor string, limit int, all bool, strict ...bool) ([]api.Task, string, error) {
	query := url.Values{}
	query.Set("query", filter)
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	tasks, next, err := fetchPaginated[api.Task](ctx, "/tasks/filter", query, all, strict...)
	if err == nil {
		return tasks, next, nil
	}
	if !isInvalidSearchQueryError(err) || !isLikelyLiteralFilter(filter) {
		return nil, "", err
	}
	query.Set("query", apptasks.ToSearchFilter(filter))
	return fetchPaginated[api.Task](ctx, "/tasks/filter", query, all, strict...)
}

func taskListCompleted(ctx *Context, completedBy, filter, project, section, parent, since, until, cursor string, limit int, all bool, wide bool, sorting taskSortOptions) error {
	path := "/tasks/completed/by_completion_date"
	if completedBy == "due" {
		path = "/tasks/completed/by_due_date"
	}
	since, until, err := normalizeCompletedDateRange(ctx, since, until)
	if err != nil {
		return err
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
	if err := sorting.apply(allTasks); err != nil {
		return err
	}
	return writeTaskList(ctx, allTasks, next, wide)
}

func isInvalidSearchQueryError(err error) bool {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Status == 400 && strings.Contains(apiErr.Message, "INVALID_SEARCH_QUERY")
}

func isLikelyLiteralFilter(filter string) bool {
	return apptasks.IsLikelyLiteralFilter(filter)
}

func normalizeCompletedDateRange(ctx *Context, since, until string) (string, string, error) {
	now := time.Now
	if ctx != nil && ctx.Now != nil {
		now = ctx.Now
	}
	s, u, err := apptasks.NormalizeCompletedDateRange(now(), since, until)
	if err != nil {
		return "", "", &CodeError{Code: exitUsage, Err: err}
	}
	return s, u, nil
}
