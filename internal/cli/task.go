package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
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
	fs := newFlagSet("add")
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
	bindHelpFlag(fs, &help)
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
