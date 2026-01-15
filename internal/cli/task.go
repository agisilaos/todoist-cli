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

func taskCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	switch args[0] {
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
	fs.IntVar(&priority, "priority", 0, "Priority")
	fs.StringVar(&dueString, "due", "", "Due string")
	fs.BoolVar(&strict, "strict", false, "Disable quick-add parsing")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
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
	if !strict {
		parsed := parseQuickAdd(content)
		if parsed.Content != "" {
			content = parsed.Content
		}
		if project == "" && parsed.Project != "" {
			project = parsed.Project
		}
		if priority == 0 && parsed.Priority > 0 {
			priority = parsed.Priority
		}
		if dueString == "" && parsed.Due != "" {
			dueString = parsed.Due
		}
		if len(labels) == 0 && len(parsed.Labels) > 0 {
			labels = append(labels, parsed.Labels...)
		}
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
		taskArgs = append(taskArgs, "--priority", strconv.Itoa(priority))
	}
	if dueString != "" {
		taskArgs = append(taskArgs, "--due", dueString)
	}
	return taskAdd(ctx, taskArgs)
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
	if err := fs.Parse(args); err != nil {
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
	var allTasks []api.Task
	var next string
	for {
		var page api.Paginated[api.Task]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/tasks", query, &page)
		cancel()
		if err != nil {
			return err
		}
		setRequestID(ctx, reqID)
		allTasks = append(allTasks, page.Results...)
		next = page.NextCursor
		if !all || next == "" {
			break
		}
		query.Set("cursor", next)
	}
	sortTasks(allTasks, sortBy)
	return writeTaskList(ctx, allTasks, next, wide)
}

func taskListFiltered(ctx *Context, filter, cursor string, limit int, all bool, wide bool) error {
	query := url.Values{}
	query.Set("query", filter)
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	var allTasks []api.Task
	var next string
	for {
		var page api.Paginated[api.Task]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/tasks/filter", query, &page)
		cancel()
		if err != nil {
			return err
		}
		setRequestID(ctx, reqID)
		allTasks = append(allTasks, page.Results...)
		next = page.NextCursor
		if !all || next == "" {
			break
		}
		query.Set("cursor", next)
	}
	// Keep original ordering from API for filter; no client sort to preserve meaning.
	return writeTaskList(ctx, allTasks, next, wide)
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
	var allTasks []api.Task
	var next string
	for {
		var page api.Paginated[api.Task]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, path, query, &page)
		cancel()
		if err != nil {
			return err
		}
		setRequestID(ctx, reqID)
		allTasks = append(allTasks, page.Results...)
		next = page.NextCursor
		if !all || next == "" {
			break
		}
		query.Set("cursor", next)
	}
	return writeTaskList(ctx, allTasks, next, wide)
}

func taskAdd(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("task add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var content string
	var description string
	var project string
	var section string
	var parent string
	var labels multiValue
	var priority int
	var dueString string
	var dueDate string
	var dueDatetime string
	var dueLang string
	var duration int
	var durationUnit string
	var deadline string
	var assignee string
	var quick bool
	var help bool
	fs.StringVar(&content, "content", "", "Task content")
	fs.StringVar(&description, "description", "", "Task description")
	fs.StringVar(&project, "project", "", "Project")
	fs.StringVar(&section, "section", "", "Section")
	fs.StringVar(&parent, "parent", "", "Parent task")
	fs.Var(&labels, "label", "Label")
	fs.IntVar(&priority, "priority", 0, "Priority")
	fs.StringVar(&dueString, "due", "", "Due string")
	fs.StringVar(&dueDate, "due-date", "", "Due date")
	fs.StringVar(&dueDatetime, "due-datetime", "", "Due datetime")
	fs.StringVar(&dueLang, "due-lang", "", "Due language")
	fs.IntVar(&duration, "duration", 0, "Duration")
	fs.StringVar(&durationUnit, "duration-unit", "", "Duration unit")
	fs.StringVar(&deadline, "deadline", "", "Deadline date")
	fs.StringVar(&assignee, "assignee", "", "Assignee ID")
	fs.BoolVar(&quick, "quick", false, "Quick add using inbox defaults")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
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
		printTaskHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--content is required (or pass as positional text)")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if quick {
		if project == "" && section == "" {
			if inboxID, err := inboxProjectID(ctx); err == nil && inboxID != "" {
				project = inboxID
			}
		}
		if len(labels) == 0 && len(ctx.Config.DefaultInboxLabels) > 0 {
			labels = append(labels, ctx.Config.DefaultInboxLabels...)
		}
		if dueString == "" && dueDate == "" && dueDatetime == "" && ctx.Config.DefaultInboxDue != "" {
			dueString = ctx.Config.DefaultInboxDue
		}
	}
	body := map[string]any{"content": content}
	if description != "" {
		body["description"] = description
	}
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err != nil {
			return err
		}
		body["project_id"] = id
	}
	if section != "" {
		id, err := resolveSectionID(ctx, section, project)
		if err != nil {
			return err
		}
		body["section_id"] = id
	}
	if parent != "" {
		body["parent_id"] = parent
	}
	if len(labels) > 0 {
		body["labels"] = []string(labels)
	}
	if priority > 0 {
		body["priority"] = priority
	}
	if dueString != "" {
		body["due_string"] = dueString
	}
	if dueDate != "" {
		body["due_date"] = dueDate
	}
	if dueDatetime != "" {
		body["due_datetime"] = dueDatetime
	}
	if dueLang != "" {
		body["due_lang"] = dueLang
	}
	if duration > 0 {
		body["duration"] = duration
	}
	if durationUnit != "" {
		body["duration_unit"] = durationUnit
	}
	if deadline != "" {
		body["deadline_date"] = deadline
	}
	if assignee != "" {
		body["assignee_id"] = assignee
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task add", body)
	}
	var task api.Task
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/tasks", nil, body, &task, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeTaskList(ctx, []api.Task{task}, "", false)
}

func taskUpdate(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("task update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	var content string
	var description string
	var labels multiValue
	var priority int
	var dueString string
	var dueDate string
	var dueDatetime string
	var dueLang string
	var duration int
	var durationUnit string
	var deadline string
	var assignee string
	var help bool
	fs.StringVar(&id, "id", "", "Task ID")
	fs.StringVar(&content, "content", "", "Task content")
	fs.StringVar(&description, "description", "", "Task description")
	fs.Var(&labels, "label", "Label")
	fs.IntVar(&priority, "priority", 0, "Priority")
	fs.StringVar(&dueString, "due", "", "Due string")
	fs.StringVar(&dueDate, "due-date", "", "Due date")
	fs.StringVar(&dueDatetime, "due-datetime", "", "Due datetime")
	fs.StringVar(&dueLang, "due-lang", "", "Due language")
	fs.IntVar(&duration, "duration", 0, "Duration")
	fs.StringVar(&durationUnit, "duration-unit", "", "Duration unit")
	fs.StringVar(&deadline, "deadline", "", "Deadline date")
	fs.StringVar(&assignee, "assignee", "", "Assignee ID")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		ref := strings.Join(fs.Args(), " ")
		if err := ensureClient(ctx); err != nil {
			return err
		}
		task, err := resolveTaskRef(ctx, ref)
		if err != nil {
			return err
		}
		id = task.ID
	}
	if id == "" {
		printTaskHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--id is required (or pass a text reference)")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	body := map[string]any{}
	if content != "" {
		body["content"] = content
	}
	if description != "" {
		body["description"] = description
	}
	if len(labels) > 0 {
		body["labels"] = []string(labels)
	}
	if priority > 0 {
		body["priority"] = priority
	}
	if dueString != "" {
		body["due_string"] = dueString
	}
	if dueDate != "" {
		body["due_date"] = dueDate
	}
	if dueDatetime != "" {
		body["due_datetime"] = dueDatetime
	}
	if dueLang != "" {
		body["due_lang"] = dueLang
	}
	if duration > 0 {
		body["duration"] = duration
	}
	if durationUnit != "" {
		body["duration_unit"] = durationUnit
	}
	if deadline != "" {
		body["deadline_date"] = deadline
	}
	if assignee != "" {
		body["assignee_id"] = assignee
	}
	if len(body) == 0 {
		return &CodeError{Code: exitUsage, Err: errors.New("no fields to update")}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task update", body)
	}
	var task api.Task
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/tasks/"+id, nil, body, &task, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeTaskList(ctx, []api.Task{task}, "", false)
}

func taskMove(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("task move", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	var project string
	var section string
	var parent string
	var help bool
	fs.StringVar(&id, "id", "", "Task ID")
	fs.StringVar(&project, "project", "", "Project")
	fs.StringVar(&section, "section", "", "Section")
	fs.StringVar(&parent, "parent", "", "Parent")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	if id == "" && len(fs.Args()) > 0 {
		ref := strings.Join(fs.Args(), " ")
		if err := ensureClient(ctx); err != nil {
			return err
		}
		task, err := resolveTaskRef(ctx, ref)
		if err != nil {
			return err
		}
		id = task.ID
	}
	if id == "" {
		printTaskHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--id is required (or pass a text reference)")}
	}
	if project == "" && section == "" && parent == "" {
		printTaskHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("at least one of --project, --section, or --parent is required")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	body := map[string]any{}
	if project != "" {
		id, err := resolveProjectID(ctx, project)
		if err != nil {
			return err
		}
		body["project_id"] = id
	}
	if section != "" {
		id, err := resolveSectionID(ctx, section, project)
		if err != nil {
			return err
		}
		body["section_id"] = id
	}
	if parent != "" {
		body["parent_id"] = parent
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task move", body)
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/tasks/"+id+"/move", nil, body, nil, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "moved", id)
}

func taskComplete(ctx *Context, args []string) error {
	id, err := requireTaskID(ctx, "task complete", args)
	if err != nil {
		printTaskHelp(ctx.Stderr)
		return err
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task complete", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/tasks/"+id+"/close", nil, nil, nil, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "completed", id)
}

func taskReopen(ctx *Context, args []string) error {
	id, err := requireTaskID(ctx, "task reopen", args)
	if err != nil {
		printTaskHelp(ctx.Stderr)
		return err
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task reopen", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Post(reqCtx, "/tasks/"+id+"/reopen", nil, nil, nil, true)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "reopened", id)
}

func taskDelete(ctx *Context, args []string) error {
	id, err := requireTaskID(ctx, "task delete", args)
	if err != nil {
		printTaskHelp(ctx.Stderr)
		return err
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	if !ctx.Global.Force && !ctx.Global.DryRun {
		ok, err := confirm(ctx, fmt.Sprintf("Delete task %s?", id))
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "task delete", map[string]any{"id": id})
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.Delete(reqCtx, "/tasks/"+id, nil)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "deleted", id)
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
	if err := fs.Parse(args); err != nil {
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
			return api.Task{}, err
		}
		setRequestID(ctx, reqID)
		return task, nil
	}
	tasks, err := listAllActiveTasks(ctx)
	if err != nil {
		return api.Task{}, err
	}
	matches := matchTasksByContent(tasks, ref)
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		var suggestions []string
		for _, task := range matches {
			suggestions = append(suggestions, fmt.Sprintf("%s (id:%s)", task.Content, task.ID))
		}
		return api.Task{}, &CodeError{Code: exitUsage, Err: fmt.Errorf("ambiguous task reference %q; matches: %s", ref, strings.Join(suggestions, ", "))}
	}
	return api.Task{}, &CodeError{Code: exitNotFound, Err: fmt.Errorf("task %q not found", ref)}
}

func listAllActiveTasks(ctx *Context) ([]api.Task, error) {
	query := url.Values{}
	query.Set("limit", "200")
	var all []api.Task
	for {
		var page api.Paginated[api.Task]
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, "/tasks", query, &page)
		cancel()
		if err != nil {
			return nil, err
		}
		setRequestID(ctx, reqID)
		all = append(all, page.Results...)
		if page.NextCursor == "" {
			break
		}
		query.Set("cursor", page.NextCursor)
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
