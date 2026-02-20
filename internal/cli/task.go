package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

type priorityFlag int

func (p *priorityFlag) String() string {
	return strconv.Itoa(int(*p))
}

func (p *priorityFlag) Set(value string) error {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "":
		*p = 0
		return nil
	case "1":
		*p = 1
		return nil
	case "2":
		*p = 2
		return nil
	case "3":
		*p = 3
		return nil
	case "4":
		*p = 4
		return nil
	case "p1":
		*p = 4
		return nil
	case "p2":
		*p = 3
		return nil
	case "p3":
		*p = 2
		return nil
	case "p4":
		*p = 1
		return nil
	}
	return fmt.Errorf("invalid priority: %s (expected 1-4 or p1-p4)", value)
}

func quickAddPriorityToken(priority int) (string, error) {
	switch priority {
	case 4:
		return "p1", nil
	case 3:
		return "p2", nil
	case 2:
		return "p3", nil
	case 1:
		return "p4", nil
	}
	return "", fmt.Errorf("invalid priority: %d", priority)
}

func buildQuickAddText(content, project string, labels []string, priority int, due string) (string, error) {
	text := strings.TrimSpace(content)
	if text == "" {
		return "", errors.New("content is required")
	}
	extras := make([]string, 0, 4)
	if project != "" {
		project = strings.TrimSpace(project)
		if strings.HasPrefix(strings.ToLower(project), "id:") {
			return "", errors.New("--project by id is not supported with quick add; use --strict")
		}
		projectName := stripIDPrefix(project)
		if isNumeric(projectName) {
			return "", errors.New("--project numeric ID is not supported with quick add; use --strict")
		}
		if strings.HasPrefix(projectName, "#") {
			extras = append(extras, projectName)
		} else {
			extras = append(extras, "#"+projectName)
		}
	}
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if strings.HasPrefix(label, "@") {
			extras = append(extras, label)
		} else {
			extras = append(extras, "@"+label)
		}
	}
	if priority > 0 {
		token, err := quickAddPriorityToken(priority)
		if err != nil {
			return "", err
		}
		extras = append(extras, token)
	}
	if due != "" {
		extras = append(extras, due)
	}
	if len(extras) == 0 {
		return text, nil
	}
	return text + " " + strings.Join(extras, " "), nil
}

func validateStrictAddInputs(project string, labels []string, dueString string) error {
	project = strings.TrimSpace(project)
	if strings.HasPrefix(project, "#") {
		return errors.New("--project cannot start with '#' when --strict is set; pass project name or id")
	}
	for _, label := range labels {
		if strings.HasPrefix(strings.TrimSpace(label), "@") {
			return errors.New("--label cannot start with '@' when --strict is set; pass label name")
		}
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(dueString)), "due:") {
		return errors.New("--due should not include 'due:' when --strict is set")
	}
	return nil
}

func taskCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	sub := canonicalSubcommand(args[0], map[string]string{
		"ls":   "list",
		"rm":   "delete",
		"del":  "delete",
		"show": "view",
	})
	switch sub {
	case "list":
		return taskList(ctx, args[1:])
	case "add":
		return taskAdd(ctx, args[1:])
	case "update":
		return taskUpdate(ctx, args[1:])
	case "move":
		return taskMove(ctx, args[1:])
	case "view":
		return taskView(ctx, args[1:])
	case "complete":
		return taskComplete(ctx, args[1:])
	case "reopen":
		return taskReopen(ctx, args[1:])
	case "delete":
		return taskDelete(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown task subcommand: %s", args[0])}
	}
}

func quickAddCommand(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var content string
	var project string
	var section string
	var labels multiValue
	var priority int
	var dueString string
	var strict bool
	var help bool
	fs.StringVar(&content, "content", "", "Task content")
	fs.StringVar(&project, "project", "", "Project")
	fs.StringVar(&section, "section", "", "Section")
	fs.Var(&labels, "label", "Label")
	fs.Var((*priorityFlag)(&priority), "priority", "Priority (accepts p1..p4)")
	fs.StringVar(&dueString, "due", "", "Due string")
	fs.BoolVar(&strict, "strict", false, "Disable quick-add parsing")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printAddHelp(ctx.Stdout)
		return nil
	}
	if content == "" && len(fs.Args()) > 0 {
		content = strings.Join(fs.Args(), " ")
	}
	if content == "-" {
		val, err := readAllTrim(ctx.Stdin)
		if err != nil {
			return err
		}
		content = val
	}
	if content == "" {
		printAddHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("content is required")}
	}
	if strict {
		if err := validateStrictAddInputs(project, labels, dueString); err != nil {
			return &CodeError{Code: exitUsage, Err: err}
		}
		taskArgs := []string{"--content", content}
		if project != "" {
			taskArgs = append(taskArgs, "--project", project)
		}
		if section != "" {
			taskArgs = append(taskArgs, "--section", section)
		}
		for _, label := range labels {
			taskArgs = append(taskArgs, "--label", label)
		}
		if priority > 0 {
			token, err := quickAddPriorityToken(priority)
			if err != nil {
				return &CodeError{Code: exitUsage, Err: err}
			}
			taskArgs = append(taskArgs, "--priority", token)
		}
		if dueString != "" {
			taskArgs = append(taskArgs, "--due", dueString)
		}
		return taskAdd(ctx, taskArgs)
	}
	if section != "" {
		return &CodeError{Code: exitUsage, Err: errors.New("--section is only supported with --strict")}
	}
	text, err := buildQuickAddText(content, project, labels, priority, dueString)
	if err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task add", map[string]any{"text": text, "sync_quick_add": true})
	}
	reqCtx, cancel := requestContext(ctx)
	task, reqID, err := ctx.Client.QuickAdd(reqCtx, text)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeTaskList(ctx, []api.Task{task}, "", false)
}

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

func taskView(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("task view", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	var full bool
	var help bool
	fs.StringVar(&id, "id", "", "Task ID")
	fs.BoolVar(&full, "full", false, "Show full task fields")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	ref := id
	if ref == "" && len(fs.Args()) > 0 {
		ref = strings.Join(fs.Args(), " ")
	}
	if ref == "" {
		printTaskHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("task view requires id or text reference")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	task, err := resolveTaskRef(ctx, ref)
	if err != nil {
		return err
	}
	return writeTaskView(ctx, task, full)
}

func resolveTaskRef(ctx *Context, ref string) (api.Task, error) {
	ref = strings.TrimSpace(ref)
	ref = stripIDPrefix(ref)
	if isNumeric(ref) {
		var task api.Task
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/tasks/"+ref, nil, &task)
		cancel()
		if err != nil {
			if isLegacyV1IDError(err) {
				return api.Task{}, &CodeError{Code: exitUsage, Err: errors.New("legacy numeric task IDs are not supported by Todoist API v1; use a current task ID from `todoist task list --json` or use text reference")}
			}
			return api.Task{}, err
		}
		setRequestID(ctx, reqID)
		return task, nil
	}
	tasks, err := listAllActiveTasks(ctx)
	if err != nil {
		return api.Task{}, err
	}
	candidates := fuzzyCandidates(ref, tasks, func(t api.Task) string { return t.Content }, func(t api.Task) string { return t.ID })
	if len(candidates) == 1 {
		for _, task := range tasks {
			if task.ID == candidates[0].ID {
				return task, nil
			}
		}
	}
	if len(candidates) > 1 {
		if chosen, ok, err := promptAmbiguousChoice(ctx, "task", ref, candidates); err != nil {
			return api.Task{}, err
		} else if ok {
			for _, task := range tasks {
				if task.ID == chosen {
					return task, nil
				}
			}
		}
		return api.Task{}, ambiguousMatchCodeError("task", ref, candidates)
	}
	return api.Task{}, &CodeError{Code: exitNotFound, Err: fmt.Errorf("task %q not found", ref)}
}

func isLegacyV1IDError(err error) bool {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return strings.Contains(apiErr.Message, "V1_ID_CANNOT_BE_USED")
}

func listAllActiveTasks(ctx *Context) ([]api.Task, error) {
	if cache := ctx.cache(); cache != nil && cache.activeTasksLoaded {
		return cloneSlice(cache.activeTasks), nil
	}
	query := url.Values{}
	query.Set("limit", "200")
	all, _, err := fetchPaginated[api.Task](ctx, "/tasks", query, true)
	if err != nil {
		return nil, err
	}
	if cache := ctx.cache(); cache != nil {
		cache.activeTasks = cloneSlice(all)
		cache.activeTasksLoaded = true
	}
	return all, nil
}

func matchTasksByContent(tasks []api.Task, ref string) []api.Task {
	refLower := strings.ToLower(ref)
	var matches []api.Task
	for _, task := range tasks {
		if strings.Contains(strings.ToLower(task.Content), refLower) {
			matches = append(matches, task)
		}
	}
	return matches
}

func writeTaskView(ctx *Context, task api.Task, full bool) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, task, output.Meta{RequestID: ctxRequestIDValue(ctx)})
	}
	fmt.Fprintf(ctx.Stdout, "ID: %s\n", task.ID)
	fmt.Fprintf(ctx.Stdout, "Content: %s\n", task.Content)
	if task.Description != "" {
		fmt.Fprintf(ctx.Stdout, "Description: %s\n", task.Description)
	}
	if task.ProjectID != "" {
		fmt.Fprintf(ctx.Stdout, "Project: %s\n", task.ProjectID)
	}
	if task.SectionID != "" {
		fmt.Fprintf(ctx.Stdout, "Section: %s\n", task.SectionID)
	}
	if len(task.Labels) > 0 {
		fmt.Fprintf(ctx.Stdout, "Labels: %s\n", strings.Join(task.Labels, ", "))
	}
	if task.Due != nil {
		fmt.Fprintf(ctx.Stdout, "Due: %s\n", formatDue(task.Due))
	}
	if full {
		fmt.Fprintf(ctx.Stdout, "Priority: %d\n", task.Priority)
		fmt.Fprintf(ctx.Stdout, "Completed: %v\n", task.Checked)
		fmt.Fprintf(ctx.Stdout, "Added: %s\n", task.AddedAt)
		fmt.Fprintf(ctx.Stdout, "Updated: %s\n", task.UpdatedAt)
		fmt.Fprintf(ctx.Stdout, "CompletedAt: %s\n", task.CompletedAt)
		fmt.Fprintf(ctx.Stdout, "NoteCount: %d\n", task.NoteCount)
	}
	return nil
}

type taskTableConfig struct {
	ID       int
	Content  int
	Project  int
	Section  int
	Labels   int
	Due      int
	Priority int
	Status   int
}

func taskTableConfigFor(ctx *Context, wide bool) taskTableConfig {
	cfg := taskTableConfig{
		ID:       8,
		Content:  50,
		Project:  16,
		Section:  12,
		Labels:   12,
		Due:      12,
		Priority: 8,
		Status:   9,
	}
	if wide {
		cfg.ID = 12
		cfg.Content = 80
		cfg.Project = 24
		cfg.Section = 18
		cfg.Labels = 18
		cfg.Due = 16
	}
	columns := tableWidth(ctx)
	fixed := cfg.ID + cfg.Project + cfg.Section + cfg.Labels + cfg.Due + cfg.Priority + cfg.Status + 7
	available := columns - fixed
	if available < 20 {
		available = 20
	}
	maxContent := 80
	if wide {
		maxContent = 120
	}
	if available < cfg.Content {
		cfg.Content = available
	} else if available < maxContent {
		cfg.Content = available
	} else {
		cfg.Content = maxContent
	}
	return cfg
}

func writeTaskList(ctx *Context, tasks []api.Task, cursor string, wide bool) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, tasks, output.Meta{RequestID: ctxRequestIDValue(ctx), Count: len(tasks), Cursor: cursor})
	}
	if ctx.Mode == output.ModeNDJSON {
		return writeTaskNDJSON(ctx, tasks)
	}
	cfg := taskTableConfigFor(ctx, wide)
	projectNames := map[string]string(nil)
	sectionNames := map[string]string(nil)
	if ctx.Mode == output.ModeHuman {
		projectNames = projectNameMap(ctx)
		sectionNames = sectionNameMap(ctx)
	}
	rows := make([][]string, 0, len(tasks))
	for _, task := range tasks {
		project := task.ProjectID
		if name, ok := projectNames[task.ProjectID]; ok {
			project = name
		}
		section := task.SectionID
		if name, ok := sectionNames[task.SectionID]; ok {
			section = name
		}
		labels := cleanCell(strings.Join(task.Labels, ","))
		content := cleanCell(task.Content)
		id := cleanCell(task.ID)
		due := formatDue(task.Due)
		if ctx.Mode == output.ModeHuman {
			content = truncateString(content, cfg.Content)
			project = truncateString(cleanCell(project), cfg.Project)
			section = truncateString(cleanCell(section), cfg.Section)
			labels = truncateString(labels, cfg.Labels)
			id = shortID(id, cfg.ID, wide)
			due = truncateString(due, cfg.Due)
		}
		rows = append(rows, []string{
			id,
			content,
			project,
			section,
			labels,
			due,
			strconv.Itoa(task.Priority),
			formatCompleted(task.Checked, task.CompletedAt),
		})
	}
	if ctx.Mode == output.ModePlain {
		return output.WritePlain(ctx.Stdout, rows)
	}
	return output.WriteTable(ctx.Stdout, []string{"ID", "Content", "Project", "Section", "Labels", "Due", "Priority", "Completed"}, rows)
}

func writeTaskNDJSON(ctx *Context, tasks []api.Task) error {
	items := make([]any, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, task)
	}
	return output.WriteNDJSON(ctx.Stdout, items)
}

func formatDue(due map[string]interface{}) string {
	if due == nil {
		return ""
	}
	if val, ok := due["datetime"].(string); ok && val != "" {
		return val
	}
	if val, ok := due["date"].(string); ok && val != "" {
		return val
	}
	if val, ok := due["string"].(string); ok && val != "" {
		return val
	}
	return ""
}

func formatCompleted(checked bool, completedAt string) string {
	if checked || completedAt != "" {
		return "yes"
	}
	return "no"
}

func sortTasks(tasks []api.Task, sortBy string) {
	switch sortBy {
	case "":
		return
	case "priority":
		sort.SliceStable(tasks, func(i, j int) bool {
			return tasks[i].Priority > tasks[j].Priority
		})
	case "due":
		sort.SliceStable(tasks, func(i, j int) bool {
			return parseDue(tasks[i].Due).Before(parseDue(tasks[j].Due))
		})
	default:
		// ignore unknown sort
	}
}

func parseDue(due map[string]interface{}) time.Time {
	if due == nil {
		return time.Time{}
	}
	if val, ok := due["datetime"].(string); ok && val != "" {
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			return t
		}
	}
	if val, ok := due["date"].(string); ok && val != "" {
		if t, err := time.Parse("2006-01-02", val); err == nil {
			return t
		}
	}
	return time.Time{}
}
