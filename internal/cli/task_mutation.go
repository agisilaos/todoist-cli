package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/agisilaos/todoist-cli/internal/api"
	apprefs "github.com/agisilaos/todoist-cli/internal/app/refs"
)

type taskEditOptions struct {
	input                                                                                 taskMutationInput
	id, content, description, project, section, parent                                    string
	labels                                                                                multiValue
	reference                                                                             bool
	order                                                                                 int
	natural, quick, clearDue, clearDeadline, clearLabels, clearAssignee, clearDescription bool
	present                                                                               map[string]bool
}

func (o *taskEditOptions) bind(fs *flag.FlagSet, update bool) {
	if update {
		fs.StringVar(&o.id, "id", "", "Task ID")
	}
	fs.StringVar(&o.content, "content", "", "Task content (\"-\" reads stdin)")
	fs.StringVar(&o.description, "description", "", "Task description (\"-\" reads exact stdin text)")
	fs.StringVar(&o.project, "project", "", "Project reference or assignee resolution scope")
	if !update {
		fs.StringVar(&o.section, "section", "", "Section")
		fs.StringVar(&o.parent, "parent", "", "Parent task ID")
		fs.BoolVar(&o.quick, "quick", false, "Apply Inbox defaults")
	}
	fs.Var(&o.labels, "label", "Label")
	fs.Var((*priorityFlag)(&o.input.Priority), "priority", "Priority (p1..p4 or API 1..4)")
	fs.StringVar(&o.input.DueString, "due", "", "Due expression")
	fs.StringVar(&o.input.DueDate, "due-date", "", "Due date")
	fs.StringVar(&o.input.DueDatetime, "due-datetime", "", "Due RFC3339 instant")
	fs.StringVar(&o.input.DueLang, "due-lang", "", "Due language")
	fs.IntVar(&o.input.Duration, "duration", 0, "Duration")
	fs.StringVar(&o.input.DurationUnit, "duration-unit", "", "Duration unit")
	fs.StringVar(&o.input.Deadline, "deadline", "", "Deadline date")
	fs.StringVar(&o.input.AssigneeRef, "assignee", "", "Assignee reference")
	fs.BoolVar(&o.reference, "reference", false, "Reference item title control")
	fs.IntVar(&o.order, "order", 0, "Sibling position (signed int32)")
	fs.BoolVar(&o.natural, "natural", false, "Parse quick-add tokens")
	if update {
		fs.BoolVar(&o.clearDue, "clear-due", false, "Clear due date and recurrence")
		fs.BoolVar(&o.clearDeadline, "clear-deadline", false, "Clear deadline")
		fs.BoolVar(&o.clearLabels, "clear-labels", false, "Clear labels")
		fs.BoolVar(&o.clearAssignee, "clear-assignee", false, "Clear assignee")
		fs.BoolVar(&o.clearDescription, "clear-description", false, "Clear description")
	}
}
func (o *taskEditOptions) prepare(ctx *Context, fs *flag.FlagSet, update bool) error {
	o.present = map[string]bool{}
	fs.Visit(func(f *flag.Flag) { o.present[f.Name] = true })
	fail := func(message string) error { return &CodeError{Code: exitUsage, Err: errors.New(message)} }
	if update && o.id != "" && len(fs.Args()) > 0 {
		return fail("--id cannot be combined with a positional task reference")
	}
	if !update && len(fs.Args()) > 0 {
		if o.present["content"] {
			return fail("content cannot be both positional and --content")
		}
		o.content = strings.Join(fs.Args(), " ")
		o.present["content"] = true
	}
	for _, pair := range []struct {
		clear  bool
		setter string
	}{{o.clearDescription, "description"}, {o.clearLabels, "label"}, {o.clearDeadline, "deadline"}, {o.clearAssignee, "assignee"}} {
		if pair.clear && o.present[pair.setter] {
			return fail("--clear-" + map[string]string{"label": "labels", "description": "description", "deadline": "deadline", "assignee": "assignee"}[pair.setter] + " conflicts with --" + pair.setter)
		}
	}
	dueCount := 0
	for _, name := range []string{"due", "due-date", "due-datetime"} {
		if o.present[name] {
			dueCount++
		}
	}
	if dueCount > 1 || o.clearDue && (dueCount > 0 || o.present["due-lang"]) {
		return fail("due setters, --due-lang and --clear-due conflict")
	}
	if o.content == "-" && o.description == "-" {
		return fail("content and description cannot both read stdin")
	}
	for name, value := range map[string]string{"content": o.content, "project": o.project, "section": o.section, "parent": o.parent, "due": o.input.DueString, "due-date": o.input.DueDate, "due-datetime": o.input.DueDatetime, "due-lang": o.input.DueLang, "deadline": o.input.Deadline, "assignee": o.input.AssigneeRef, "id": o.id, "duration-unit": o.input.DurationUnit} {
		if o.present[name] && strings.TrimSpace(value) == "" {
			return fail("--" + name + " cannot be empty")
		}
	}
	for _, label := range o.labels {
		if strings.TrimSpace(label) == "" {
			return fail("--label cannot be empty; use --clear-labels")
		}
	}
	if o.present["parent"] {
		id, err := normalizeTaskInputID(o.parent)
		if err != nil {
			return fail("invalid --parent: " + err.Error())
		}
		o.parent = id
	}
	if o.present["order"] {
		if int64(o.order) < -2147483648 || int64(o.order) > 2147483647 {
			return fail("--order must be signed int32")
		}
		o.input.Order = &o.order
	}
	if o.present["priority"] && (o.input.Priority < 1 || o.input.Priority > 4) {
		return fail("priority must be p1..p4 or API 1..4")
	}
	if o.content == "-" {
		value, err := readAllTrim(ctx.Stdin)
		if err != nil {
			return err
		}
		o.content = value
	}
	if o.description == "-" {
		value, err := io.ReadAll(ctx.Stdin)
		if err != nil {
			return err
		}
		if !utf8.Valid(value) {
			return fail("description stdin must be UTF-8 text")
		}
		o.description = string(value)
	}
	if o.natural {
		seen := map[string]bool{}
		for _, token := range strings.Fields(o.content) {
			name := quickAddTokenKind(token)
			if name == "" {
				continue
			}
			conflict := o.present[name]
			if name == "due" {
				conflict = conflict || dueCount > 0 || o.clearDue
			}
			if name == "label" {
				conflict = conflict || o.clearLabels
			}
			if conflict || name != "label" && seen[name] {
				return fail("natural input conflicts for " + name)
			}
			seen[name] = true
		}
		parsed := parseQuickAdd(o.content)
		o.content = parsed.Content
		if parsed.Project != "" {
			o.project = parsed.Project
		}
		o.labels = append(o.labels, parsed.Labels...)
		if parsed.Priority > 0 {
			o.input.Priority = parsed.Priority
		}
		if parsed.Due != "" {
			o.input.DueString = parsed.Due
		}
	}
	if (!update || o.present["content"]) && strings.TrimSpace(o.content) == "" {
		return fail("task content cannot be empty")
	}
	for _, field := range []struct{ name, value, layout string }{{"due-date", o.input.DueDate, "2006-01-02"}, {"due-datetime", o.input.DueDatetime, time.RFC3339Nano}, {"deadline", o.input.Deadline, "2006-01-02"}} {
		if field.value != "" {
			var err error
			if field.layout == time.RFC3339Nano {
				_, err = parseTaskInstant(field.value)
			} else {
				_, err = time.Parse(field.layout, field.value)
			}
			if err != nil {
				return fail("invalid --" + field.name)
			}
		}
	}
	if o.present["duration"] != o.present["duration-unit"] {
		return fail("--duration and --duration-unit must be supplied together")
	}
	if o.present["duration"] && o.input.Duration <= 0 {
		return fail("duration must be positive")
	}
	if o.input.DurationUnit != "" && o.input.DurationUnit != "minute" && o.input.DurationUnit != "day" {
		return fail("duration-unit must be minute or day")
	}
	if o.present["reference"] && o.present["content"] {
		value, err := referenceTitle(o.content, o.reference)
		if err != nil {
			return err
		}
		o.content = value
	}
	o.input.Content = o.content
	o.input.Description = o.description
	o.input.DescriptionSet = o.present["description"] || o.clearDescription
	o.input.ProjectRef = o.project
	o.input.SectionRef = o.section
	o.input.ParentID = o.parent
	o.input.Labels = []string(o.labels)
	o.input.ClearLabels = o.clearLabels
	o.input.ClearDeadline = o.clearDeadline
	o.input.ClearAssignee = o.clearAssignee
	o.input.AssigneeHint = o.project
	return nil
}
func referenceTitle(content string, reference bool) (string, error) {
	plain := strings.TrimPrefix(content, "* ")
	if strings.HasPrefix(plain, "* ") || strings.TrimSpace(plain) == "" {
		return "", &CodeError{Code: exitUsage, Err: errors.New("reference title must have one prefix and nonblank content")}
	}
	if reference {
		return "* " + plain, nil
	}
	return plain, nil
}
func taskAdd(ctx *Context, args []string) error    { return taskEdit(ctx, args, false) }
func taskUpdate(ctx *Context, args []string) error { return taskEdit(ctx, args, true) }
func taskEdit(ctx *Context, args []string, update bool) error {
	command := "task add"
	if update {
		command = "task update"
	}
	fs := newFlagSet(command)
	var o taskEditOptions
	var help bool
	o.bind(fs, update)
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printTaskHelp(ctx.Stdout)
		return nil
	}
	if err := o.prepare(ctx, fs, update); err != nil {
		return err
	}
	if update && o.id == "" && len(fs.Args()) == 0 {
		return &CodeError{Code: exitUsage, Err: errors.New("task update requires --id or a reference")}
	}
	if !update && o.present["reference"] {
		content, err := referenceTitle(o.content, o.reference)
		if err != nil {
			return err
		}
		o.input.Content = content
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	var before *api.Task
	if update {
		var err error
		o.id, before, err = resolveEditingTarget(ctx, o.id, strings.Join(fs.Args(), " "), o.present["reference"] && !o.present["content"])
		if err != nil {
			return err
		}
		o.input.TaskID = o.id
		o.input.ProjectRef = ""
		if o.present["reference"] {
			content := o.content
			if !o.present["content"] {
				value, ok := before.ResponseFact("content").Text()
				if !ok {
					return errors.New("task content unavailable; cannot edit reference status")
				}
				content = value
			}
			transformed, err := referenceTitle(content, o.reference)
			if err != nil {
				return err
			}
			o.input.Content = transformed
		}
	}
	if o.quick {
		if o.project == "" && o.section == "" {
			id, err := inboxProjectID(ctx)
			if err != nil {
				return err
			}
			o.input.ProjectRef = id
		}
		if len(o.input.Labels) == 0 {
			o.input.Labels = ctx.Config.DefaultInboxLabels
		}
		if o.input.DueString == "" && o.input.DueDate == "" && o.input.DueDatetime == "" {
			o.input.DueString = ctx.Config.DefaultInboxDue
		}
	}
	var body map[string]any
	var err error
	if update {
		body, err = buildTaskUpdatePayload(ctx, o.input)
		if err == nil && (o.present["content"] || o.present["reference"]) {
			body["content"] = o.input.Content
		}
	} else {
		body, err = buildTaskCreatePayload(ctx, o.input)
	}
	if err != nil {
		return err
	}
	if update && len(body) == 0 && !o.clearDue {
		return &CodeError{Code: exitUsage, Err: errors.New("no fields to update")}
	}
	if ctx.Global.DryRun {
		if o.clearDue {
			return writeDryRun(ctx, command, map[string]any{"id": o.id, "clear_due": true, "payload": body})
		}
		if !update {
			return writeCapturePreview(ctx, command, body)
		}
		return writeDryRun(ctx, command, body)
	}
	if update && before != nil && o.present["reference"] && !o.present["content"] && len(body) == 1 && !o.clearDue {
		content, _ := before.ResponseFact("content").Text()
		if content == o.input.Content {
			return writeTaskAcknowledgement(ctx, o.id, "task_update", "unchanged")
		}
	}
	// Every selector, title transformation and authorization check precedes step one.
	if err := currentAuthorization(ctx).CheckMutation(); err != nil {
		return err
	}
	path := "/tasks"
	operation := "task_add"
	if update {
		path = taskPath(o.id)
		operation = "task_update"
	}
	if o.clearDue {
		raw, reqID, err := nativeTaskCommand(ctx, "item_update", map[string]any{"id": o.id, "due": nil})
		if err != nil {
			return err
		}
		if len(body) == 0 {
			return writeReturnedTask(ctx, raw, o.id, operation, false)
		}
		steps := []taskWriteStep{{Operation: "clear_due", Outcome: "accepted", Dispatched: true, RequestID: reqID}}
		raw, reqID, err = postTaskResource(ctx, path, body)
		if err != nil {
			outcome := api.TaskWriteOutcome(err)
			steps = append(steps, taskWriteStep{Operation: "update_fields", Outcome: outcome, Dispatched: outcome != "not_dispatched", RequestID: reqID})
			return writePartialTaskEdit(ctx, o.id, steps, err)
		}
		return writeReturnedTask(ctx, raw, o.id, operation, false)
	}
	raw, _, err := postTaskResource(ctx, path, body)
	if err != nil {
		return err
	}
	return writeReturnedTask(ctx, raw, o.id, operation, !update)
}
func resolveEditingTarget(ctx *Context, id, ref string, needFacts bool) (string, *api.Task, error) {
	if id != "" {
		normalized, err := normalizeTaskInputID(id)
		if err != nil {
			return "", nil, &CodeError{Code: exitUsage, Err: err}
		}
		id = normalized
		if !needFacts {
			return id, nil, nil
		}
		ref = "id:" + id
	}
	task, err := resolveTaskRef(ctx, ref)
	if err != nil {
		return "", nil, err
	}
	if !usableTaskID(task.ID) {
		return "", nil, errors.New("resolved task identity unavailable")
	}
	return task.ID, &task, nil
}
func normalizeTaskInputID(value string) (string, error) {
	id := strings.TrimSpace(stripIDPrefix(value))
	if strings.Contains(id, "://") {
		taskRef, _, err := apprefs.NormalizeEntityRef(value, "task")
		if err != nil {
			return "", err
		}
		id = taskRef
	}
	if !usableTaskID(id) {
		return "", fmt.Errorf("task ID must be nonempty without whitespace/control characters")
	}
	return id, nil
}
