package cli

import (
	"errors"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func taskAdd(ctx *Context, args []string) error {
	fs := newFlagSet("task add")
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
	var natural bool
	var help bool
	fs.StringVar(&content, "content", "", "Task content")
	fs.StringVar(&description, "description", "", "Task description")
	fs.StringVar(&project, "project", "", "Project")
	fs.StringVar(&section, "section", "", "Section")
	fs.StringVar(&parent, "parent", "", "Parent task")
	fs.Var(&labels, "label", "Label")
	fs.Var((*priorityFlag)(&priority), "priority", "Priority (accepts p1..p4)")
	fs.StringVar(&dueString, "due", "", "Due string")
	fs.StringVar(&dueDate, "due-date", "", "Due date")
	fs.StringVar(&dueDatetime, "due-datetime", "", "Due datetime")
	fs.StringVar(&dueLang, "due-lang", "", "Due language")
	fs.IntVar(&duration, "duration", 0, "Duration")
	fs.StringVar(&durationUnit, "duration-unit", "", "Duration unit")
	fs.StringVar(&deadline, "deadline", "", "Deadline date")
	fs.StringVar(&assignee, "assignee", "", "Assignee reference (id, me, name, email)")
	fs.BoolVar(&quick, "quick", false, "Quick add using inbox defaults")
	fs.BoolVar(&natural, "natural", false, "Parse quick-add style tokens in content (#project @label p1..p4 due:...)")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
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
	if natural {
		parsed := parseQuickAdd(content)
		if parsed.Content != "" {
			content = parsed.Content
		}
		if project == "" && parsed.Project != "" {
			project = parsed.Project
		}
		if len(labels) == 0 && len(parsed.Labels) > 0 {
			labels = append(labels, parsed.Labels...)
		}
		if priority == 0 && parsed.Priority > 0 {
			priority = parsed.Priority
		}
		if dueString == "" && parsed.Due != "" {
			dueString = parsed.Due
		}
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
	body, err := buildTaskCreatePayload(ctx, taskMutationInput{
		Content:      content,
		Description:  description,
		ProjectRef:   project,
		SectionRef:   section,
		ParentID:     parent,
		Labels:       []string(labels),
		Priority:     priority,
		DueString:    dueString,
		DueDate:      dueDate,
		DueDatetime:  dueDatetime,
		DueLang:      dueLang,
		Duration:     duration,
		DurationUnit: durationUnit,
		Deadline:     deadline,
		AssigneeRef:  assignee,
		AssigneeHint: project,
	})
	if err != nil {
		return err
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
	fs := newFlagSet("task update")
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
	var project string
	var natural bool
	var help bool
	fs.StringVar(&id, "id", "", "Task ID")
	fs.StringVar(&content, "content", "", "Task content")
	fs.StringVar(&description, "description", "", "Task description")
	fs.Var(&labels, "label", "Label")
	fs.Var((*priorityFlag)(&priority), "priority", "Priority (accepts p1..p4)")
	fs.StringVar(&dueString, "due", "", "Due string")
	fs.StringVar(&dueDate, "due-date", "", "Due date")
	fs.StringVar(&dueDatetime, "due-datetime", "", "Due datetime")
	fs.StringVar(&dueLang, "due-lang", "", "Due language")
	fs.IntVar(&duration, "duration", 0, "Duration")
	fs.StringVar(&durationUnit, "duration-unit", "", "Duration unit")
	fs.StringVar(&deadline, "deadline", "", "Deadline date")
	fs.StringVar(&assignee, "assignee", "", "Assignee reference (id, me, name, email)")
	fs.StringVar(&project, "project", "", "Project (used for assignee name/email resolution)")
	fs.BoolVar(&natural, "natural", false, "Parse quick-add style tokens in content (#project @label p1..p4 due:...)")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
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
	if natural && strings.TrimSpace(content) != "" {
		parsed := parseQuickAdd(content)
		if parsed.Content != "" {
			content = parsed.Content
		}
		if project == "" && parsed.Project != "" {
			project = parsed.Project
		}
		if len(labels) == 0 && len(parsed.Labels) > 0 {
			labels = append(labels, parsed.Labels...)
		}
		if priority == 0 && parsed.Priority > 0 {
			priority = parsed.Priority
		}
		if dueString == "" && parsed.Due != "" {
			dueString = parsed.Due
		}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	body, err := buildTaskUpdatePayload(ctx, taskMutationInput{
		Content:      content,
		Description:  description,
		Labels:       []string(labels),
		Priority:     priority,
		DueString:    dueString,
		DueDate:      dueDate,
		DueDatetime:  dueDatetime,
		DueLang:      dueLang,
		Duration:     duration,
		DurationUnit: durationUnit,
		Deadline:     deadline,
		AssigneeRef:  assignee,
		AssigneeHint: project,
		TaskID:       id,
	})
	if err != nil {
		return err
	}
	if content != "" {
		body["content"] = content
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
