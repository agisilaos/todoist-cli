package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func inboxCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printInboxHelp(ctx.Stdout)
		if len(args) == 0 {
			return nil
		}
	}
	switch args[0] {
	case "add":
		return inboxAdd(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown inbox subcommand: %s", args[0])}
	}
}

func inboxAdd(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("inbox add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var content string
	var description string
	var section string
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
	fs.StringVar(&content, "content", "", "Task content")
	fs.StringVar(&description, "description", "", "Task description")
	fs.StringVar(&section, "section", "", "Section")
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
		printInboxHelp(ctx.Stdout)
		return nil
	}
	if content == "-" {
		val, err := readAllTrim(ctx.Stdin)
		if err != nil {
			return err
		}
		content = val
	}
	if content == "" {
		printInboxHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("--content is required")}
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	inboxID, err := inboxProjectID(ctx)
	if err != nil || inboxID == "" {
		return &CodeError{Code: exitError, Err: errors.New("failed to resolve Inbox project")}
	}

	// Apply defaults from config when not explicitly set.
	if len(labels) == 0 && len(ctx.Config.DefaultInboxLabels) > 0 {
		labels = append(labels, ctx.Config.DefaultInboxLabels...)
	}
	if dueString == "" && dueDate == "" && dueDatetime == "" && ctx.Config.DefaultInboxDue != "" {
		dueString = ctx.Config.DefaultInboxDue
	}

	body := map[string]any{
		"content":    content,
		"project_id": inboxID,
	}
	if description != "" {
		body["description"] = description
	}
	if section != "" {
		id, err := resolveSectionID(ctx, section, inboxID)
		if err != nil {
			return err
		}
		body["section_id"] = id
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
		return writeDryRun(ctx, "inbox add", body)
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
