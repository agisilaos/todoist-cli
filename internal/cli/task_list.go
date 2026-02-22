package cli

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
)

var relativeAgoPattern = regexp.MustCompile(`^([0-9]+)\s+(day|days|week|weeks)\s+ago$`)

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
	bindHelpFlag(fs, &help)
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
	tasks, next, err := fetchPaginated[api.Task](ctx, "/tasks/filter", query, all)
	if err == nil {
		return tasks, next, nil
	}
	if !isInvalidSearchQueryError(err) || !isLikelyLiteralFilter(filter) {
		return nil, "", err
	}
	query.Set("query", toSearchFilter(filter))
	return fetchPaginated[api.Task](ctx, "/tasks/filter", query, all)
}

func taskListCompleted(ctx *Context, completedBy, filter, project, section, parent, since, until, cursor string, limit int, all bool, wide bool) error {
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
	value := strings.TrimSpace(filter)
	if value == "" {
		return false
	}
	if strings.ContainsAny(value, "@#|&!:()[]{}") {
		return false
	}
	return true
}

func toSearchFilter(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	escaped := replacer.Replace(strings.TrimSpace(value))
	return fmt.Sprintf(`search: "%s"`, escaped)
}

func normalizeCompletedDateRange(ctx *Context, since, until string) (string, string, error) {
	now := time.Now
	if ctx != nil && ctx.Now != nil {
		now = ctx.Now
	}
	var err error
	since = strings.TrimSpace(since)
	until = strings.TrimSpace(until)
	if since != "" {
		since, err = normalizeCompletedDateValue(since, now())
		if err != nil {
			return "", "", err
		}
		if until == "" {
			until = now().UTC().Format("2006-01-02")
		}
	}
	if until != "" {
		until, err = normalizeCompletedDateValue(until, now())
		if err != nil {
			return "", "", err
		}
	}
	if since != "" && until != "" && since > until {
		return "", "", &CodeError{Code: exitUsage, Err: errors.New("--since must be on or before --until")}
	}
	return since, until, nil
}

func normalizeCompletedDateValue(value string, now time.Time) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t.Format("2006-01-02"), nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC().Format("2006-01-02"), nil
	}
	lower := strings.ToLower(value)
	switch lower {
	case "today":
		return now.UTC().Format("2006-01-02"), nil
	case "yesterday":
		return now.UTC().AddDate(0, 0, -1).Format("2006-01-02"), nil
	case "tomorrow":
		return now.UTC().AddDate(0, 0, 1).Format("2006-01-02"), nil
	}
	if weekday, ok := parseWeekday(lower); ok {
		return mostRecentWeekday(now.UTC(), weekday).Format("2006-01-02"), nil
	}
	if m := relativeAgoPattern.FindStringSubmatch(lower); len(m) == 3 {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "day", "days":
			return now.UTC().AddDate(0, 0, -n).Format("2006-01-02"), nil
		case "week", "weeks":
			return now.UTC().AddDate(0, 0, -(n * 7)).Format("2006-01-02"), nil
		}
	}
	return "", &CodeError{
		Code: exitUsage,
		Err:  fmt.Errorf("invalid date %q; use YYYY-MM-DD, RFC3339, today/yesterday, weekday name, or '<N> days ago'", value),
	}
}

func parseWeekday(value string) (time.Weekday, bool) {
	switch value {
	case "sunday":
		return time.Sunday, true
	case "monday":
		return time.Monday, true
	case "tuesday":
		return time.Tuesday, true
	case "wednesday":
		return time.Wednesday, true
	case "thursday":
		return time.Thursday, true
	case "friday":
		return time.Friday, true
	case "saturday":
		return time.Saturday, true
	default:
		return time.Sunday, false
	}
}

func mostRecentWeekday(now time.Time, weekday time.Weekday) time.Time {
	diff := (int(now.Weekday()) - int(weekday) + 7) % 7
	return now.AddDate(0, 0, -diff)
}
