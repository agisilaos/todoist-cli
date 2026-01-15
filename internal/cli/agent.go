package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

type PlannerRequest struct {
	Instruction string         `json:"instruction"`
	Profile     string         `json:"profile"`
	Context     PlannerContext `json:"context"`
	Now         string         `json:"now"`
}

type PlannerContext struct {
	Projects       []any `json:"projects"`
	Sections       []any `json:"sections"`
	Labels         []any `json:"labels"`
	CompletedTasks []any `json:"completed_tasks,omitempty"`
}

type Plan struct {
	Version      int         `json:"version"`
	Instruction  string      `json:"instruction"`
	CreatedAt    string      `json:"created_at"`
	ConfirmToken string      `json:"confirm_token"`
	Summary      PlanSummary `json:"summary"`
	Actions      []Action    `json:"actions"`
	AppliedAt    string      `json:"applied_at,omitempty"`
}

type PlanSummary struct {
	Tasks    int `json:"tasks"`
	Projects int `json:"projects"`
	Sections int `json:"sections"`
	Labels   int `json:"labels"`
	Comments int `json:"comments"`
}

type Action struct {
	Type         string   `json:"type"`
	TaskID       string   `json:"task_id,omitempty"`
	ProjectID    string   `json:"project_id,omitempty"`
	SectionID    string   `json:"section_id,omitempty"`
	LabelID      string   `json:"label_id,omitempty"`
	CommentID    string   `json:"comment_id,omitempty"`
	Idempotent   bool     `json:"idempotent,omitempty"`
	Content      string   `json:"content,omitempty"`
	Description  string   `json:"description,omitempty"`
	Name         string   `json:"name,omitempty"`
	Labels       []string `json:"labels,omitempty"`
	Project      string   `json:"project,omitempty"`
	Section      string   `json:"section,omitempty"`
	Parent       string   `json:"parent,omitempty"`
	Priority     int      `json:"priority,omitempty"`
	Due          string   `json:"due,omitempty"`
	DueDate      string   `json:"due_date,omitempty"`
	DueDatetime  string   `json:"due_datetime,omitempty"`
	DueLang      string   `json:"due_lang,omitempty"`
	Duration     int      `json:"duration,omitempty"`
	DurationUnit string   `json:"duration_unit,omitempty"`
	Deadline     string   `json:"deadline_date,omitempty"`
	Assignee     string   `json:"assignee_id,omitempty"`
	Color        string   `json:"color,omitempty"`
	Order        int      `json:"order,omitempty"`
	Favorite     *bool    `json:"is_favorite,omitempty"`
}

func agentCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printAgentHelp(ctx.Stdout)
		return nil
	}
	switch args[0] {
	case "plan":
		return agentPlan(ctx, args[1:])
	case "apply":
		return agentApply(ctx, args[1:])
	case "status":
		return agentStatus(ctx)
	case "run":
		return agentRun(ctx, args[1:])
	case "schedule":
		return agentSchedule(ctx, args[1:])
	case "examples":
		return agentExamples(ctx)
	case "planner":
		return agentPlanner(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown agent subcommand: %s", args[0])}
	}
}

func agentPlan(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("agent plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var outPath string
	var planner string
	var expectedVersion int
	var contextProjects multiValue
	var contextLabels multiValue
	var contextCompleted string
	var help bool
	fs.StringVar(&outPath, "out", "", "Output plan file")
	fs.StringVar(&planner, "planner", "", "Planner command")
	fs.IntVar(&expectedVersion, "plan-version", 1, "Expected plan version")
	fs.Var(&contextProjects, "context-project", "Project context (repeatable)")
	fs.Var(&contextLabels, "context-label", "Label context (repeatable)")
	fs.StringVar(&contextCompleted, "context-completed", "", "Include completed tasks from last Nd (e.g. 7d)")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printAgentHelp(ctx.Stdout)
		return nil
	}
	instruction := strings.Join(fs.Args(), " ")
	if instruction == "" {
		printAgentHelp(ctx.Stderr)
		return &CodeError{Code: exitUsage, Err: errors.New("instruction is required")}
	}
	ctxOpts, err := parseContextOptions(ctx, contextProjects, contextLabels, contextCompleted)
	if err != nil {
		return err
	}
	plan, err := runPlanner(ctx, planner, instruction, expectedVersion, ctxOpts)
	if err != nil {
		return err
	}
	if outPath != "" && outPath != "-" {
		if err := writePlanFile(outPath, plan); err != nil {
			return err
		}
	}
	if err := writePlanFile(lastPlanPath(ctx), plan); err != nil {
		return err
	}
	return writePlanOutput(ctx, plan)
}

func agentApply(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("agent apply", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var planPath string
	var confirm string
	var planner string
	var onError string
	var expectedVersion int
	var contextProjects multiValue
	var contextLabels multiValue
	var contextCompleted string
	var help bool
	fs.StringVar(&planPath, "plan", "", "Plan file (or - for stdin)")
	fs.StringVar(&confirm, "confirm", "", "Confirmation token")
	fs.StringVar(&planner, "planner", "", "Planner command")
	fs.StringVar(&onError, "on-error", "fail", "On error: fail|continue")
	fs.IntVar(&expectedVersion, "plan-version", 1, "Expected plan version")
	fs.Var(&contextProjects, "context-project", "Project context (repeatable)")
	fs.Var(&contextLabels, "context-label", "Label context (repeatable)")
	fs.StringVar(&contextCompleted, "context-completed", "", "Include completed tasks from last Nd (e.g. 7d)")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printAgentHelp(ctx.Stdout)
		return nil
	}
	var plan Plan
	if planPath != "" {
		var err error
		plan, err = readPlanFile(planPath, ctx.Stdin)
		if err != nil {
			return err
		}
	} else {
		instruction := strings.Join(fs.Args(), " ")
		if instruction == "" {
			printAgentHelp(ctx.Stderr)
			return &CodeError{Code: exitUsage, Err: errors.New("instruction is required when --plan is not provided")}
		}
		ctxOpts, err := parseContextOptions(ctx, contextProjects, contextLabels, contextCompleted)
		if err != nil {
			return err
		}
		p, err := runPlanner(ctx, planner, instruction, expectedVersion, ctxOpts)
		if err != nil {
			return err
		}
		plan = p
	}
	if err := validatePlan(plan, expectedVersion); err != nil {
		return err
	}
	if !ctx.Global.Force {
		if confirm == "" {
			printAgentHelp(ctx.Stderr)
			return &CodeError{Code: exitUsage, Err: errors.New("--confirm is required (or use --force)")}
		}
		if plan.ConfirmToken != "" && confirm != plan.ConfirmToken {
			printAgentHelp(ctx.Stderr)
			return &CodeError{Code: exitUsage, Err: errors.New("confirmation token does not match plan")}
		}
	}
	if ctx.Global.DryRun {
		return writePlanPreview(ctx, plan, true)
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	results, err := applyActionsWithMode(ctx, plan.Actions, onError)
	if err != nil && onError == "fail" {
		return err
	}
	plan.AppliedAt = ctx.Now().UTC().Format(time.RFC3339)
	if err := writePlanFile(lastPlanPath(ctx), plan); err != nil {
		return err
	}
	return writePlanApplyResult(ctx, plan, results, err)
}

func agentStatus(ctx *Context) error {
	plan, err := readPlanFile(lastPlanPath(ctx), nil)
	if err != nil {
		return err
	}
	return writePlanOutput(ctx, plan)
}

func runPlanner(ctx *Context, plannerCmd string, instruction string, expectedVersion int, ctxOpts plannerContextOptions) (Plan, error) {
	if plannerCmd == "" {
		plannerCmd, _ = resolvePlannerCmd(ctx, plannerCmd, true)
	}
	if plannerCmd == "" {
		return Plan{}, &CodeError{Code: exitUsage, Err: errors.New("no planner configured; set TODOIST_PLANNER_CMD or --planner")}
	}
	if err := ensureClient(ctx); err != nil {
		return Plan{}, err
	}
	plannerContext, err := buildPlannerContext(ctx, ctxOpts)
	if err != nil {
		return Plan{}, err
	}
	request := PlannerRequest{
		Instruction: instruction,
		Profile:     ctx.Profile,
		Context:     plannerContext,
		Now:         ctx.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return Plan{}, err
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), time.Duration(ctx.Config.TimeoutSeconds)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "/bin/sh", "-c", plannerCmd)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return Plan{}, fmt.Errorf("planner failed: %s", msg)
	}
	var plan Plan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		return Plan{}, fmt.Errorf("parse planner output: %w", err)
	}
	if err := normalizeAndValidatePlan(&plan, instruction, ctx.Now, expectedVersion); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func applyAction(ctx *Context, action Action) error {
	switch action.Type {
	case "task_add":
		body := map[string]any{"content": action.Content}
		if action.Description != "" {
			body["description"] = action.Description
		}
		if action.Project != "" {
			id, _ := resolveProjectID(ctx, action.Project)
			body["project_id"] = id
		}
		if action.Section != "" {
			id, _ := resolveSectionID(ctx, action.Section, action.Project)
			body["section_id"] = id
		}
		if action.Parent != "" {
			body["parent_id"] = action.Parent
		}
		if len(action.Labels) > 0 {
			body["labels"] = action.Labels
		}
		if action.Priority > 0 {
			body["priority"] = action.Priority
		}
		if action.Due != "" {
			body["due_string"] = action.Due
		}
		if action.DueDate != "" {
			body["due_date"] = action.DueDate
		}
		if action.DueDatetime != "" {
			body["due_datetime"] = action.DueDatetime
		}
		if action.DueLang != "" {
			body["due_lang"] = action.DueLang
		}
		if action.Duration > 0 {
			body["duration"] = action.Duration
		}
		if action.DurationUnit != "" {
			body["duration_unit"] = action.DurationUnit
		}
		if action.Deadline != "" {
			body["deadline_date"] = action.Deadline
		}
		if action.Assignee != "" {
			body["assignee_id"] = action.Assignee
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/tasks", nil, body, nil, true)
		cancel()
		return err
	case "task_update":
		if action.TaskID == "" {
			return errors.New("task_update requires task_id")
		}
		body := map[string]any{}
		if action.Content != "" {
			body["content"] = action.Content
		}
		if action.Description != "" {
			body["description"] = action.Description
		}
		if len(action.Labels) > 0 {
			body["labels"] = action.Labels
		}
		if action.Priority > 0 {
			body["priority"] = action.Priority
		}
		if action.Due != "" {
			body["due_string"] = action.Due
		}
		if action.DueDate != "" {
			body["due_date"] = action.DueDate
		}
		if action.DueDatetime != "" {
			body["due_datetime"] = action.DueDatetime
		}
		if action.DueLang != "" {
			body["due_lang"] = action.DueLang
		}
		if action.Duration > 0 {
			body["duration"] = action.Duration
		}
		if action.DurationUnit != "" {
			body["duration_unit"] = action.DurationUnit
		}
		if action.Deadline != "" {
			body["deadline_date"] = action.Deadline
		}
		if action.Assignee != "" {
			body["assignee_id"] = action.Assignee
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/tasks/"+action.TaskID, nil, body, nil, true)
		cancel()
		return err
	case "task_move":
		if action.TaskID == "" {
			return errors.New("task_move requires task_id")
		}
		body := map[string]any{}
		if action.Project != "" {
			id, _ := resolveProjectID(ctx, action.Project)
			body["project_id"] = id
		}
		if action.Section != "" {
			id, _ := resolveSectionID(ctx, action.Section, action.Project)
			body["section_id"] = id
		}
		if action.Parent != "" {
			body["parent_id"] = action.Parent
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/tasks/"+action.TaskID+"/move", nil, body, nil, true)
		cancel()
		return err
	case "task_complete":
		if action.TaskID == "" {
			return errors.New("task_complete requires task_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/tasks/"+action.TaskID+"/close", nil, nil, nil, true)
		cancel()
		return err
	case "task_reopen":
		if action.TaskID == "" {
			return errors.New("task_reopen requires task_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/tasks/"+action.TaskID+"/reopen", nil, nil, nil, true)
		cancel()
		return err
	case "task_delete":
		if action.TaskID == "" {
			return errors.New("task_delete requires task_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Delete(reqCtx, "/tasks/"+action.TaskID, nil)
		cancel()
		return err
	case "project_add":
		if action.Name == "" {
			return errors.New("project_add requires name")
		}
		body := map[string]any{"name": action.Name}
		if action.Description != "" {
			body["description"] = action.Description
		}
		if action.Parent != "" {
			id, _ := resolveProjectID(ctx, action.Parent)
			body["parent_id"] = id
		}
		if action.Color != "" {
			body["color"] = action.Color
		}
		if action.Favorite != nil {
			body["is_favorite"] = *action.Favorite
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/projects", nil, body, nil, true)
		cancel()
		return err
	case "project_update":
		if action.ProjectID == "" {
			return errors.New("project_update requires project_id")
		}
		body := map[string]any{}
		if action.Name != "" {
			body["name"] = action.Name
		}
		if action.Description != "" {
			body["description"] = action.Description
		}
		if action.Color != "" {
			body["color"] = action.Color
		}
		if action.Favorite != nil {
			body["is_favorite"] = *action.Favorite
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/projects/"+action.ProjectID, nil, body, nil, true)
		cancel()
		return err
	case "project_archive":
		if action.ProjectID == "" {
			return errors.New("project_archive requires project_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/projects/"+action.ProjectID+"/archive", nil, nil, nil, true)
		cancel()
		return err
	case "project_unarchive":
		if action.ProjectID == "" {
			return errors.New("project_unarchive requires project_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/projects/"+action.ProjectID+"/unarchive", nil, nil, nil, true)
		cancel()
		return err
	case "project_delete":
		if action.ProjectID == "" {
			return errors.New("project_delete requires project_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Delete(reqCtx, "/projects/"+action.ProjectID, nil)
		cancel()
		return err
	case "section_add":
		if action.Name == "" || action.Project == "" {
			return errors.New("section_add requires name and project")
		}
		projectID, _ := resolveProjectID(ctx, action.Project)
		body := map[string]any{"name": action.Name, "project_id": projectID}
		if action.Order > 0 {
			body["order"] = action.Order
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/sections", nil, body, nil, true)
		cancel()
		return err
	case "section_update":
		if action.SectionID == "" || action.Name == "" {
			return errors.New("section_update requires section_id and name")
		}
		body := map[string]any{"name": action.Name}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/sections/"+action.SectionID, nil, body, nil, true)
		cancel()
		return err
	case "section_delete":
		if action.SectionID == "" {
			return errors.New("section_delete requires section_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Delete(reqCtx, "/sections/"+action.SectionID, nil)
		cancel()
		return err
	case "label_add":
		if action.Name == "" {
			return errors.New("label_add requires name")
		}
		body := map[string]any{"name": action.Name}
		if action.Color != "" {
			body["color"] = action.Color
		}
		if action.Order > 0 {
			body["order"] = action.Order
		}
		if action.Favorite != nil {
			body["is_favorite"] = *action.Favorite
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/labels", nil, body, nil, true)
		cancel()
		return err
	case "label_update":
		if action.LabelID == "" {
			return errors.New("label_update requires label_id")
		}
		body := map[string]any{}
		if action.Name != "" {
			body["name"] = action.Name
		}
		if action.Color != "" {
			body["color"] = action.Color
		}
		if action.Order > 0 {
			body["order"] = action.Order
		}
		if action.Favorite != nil {
			body["is_favorite"] = *action.Favorite
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/labels/"+action.LabelID, nil, body, nil, true)
		cancel()
		return err
	case "label_delete":
		if action.LabelID == "" {
			return errors.New("label_delete requires label_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Delete(reqCtx, "/labels/"+action.LabelID, nil)
		cancel()
		return err
	case "comment_add":
		if action.Content == "" {
			return errors.New("comment_add requires content")
		}
		body := map[string]any{"content": action.Content}
		if action.TaskID != "" {
			body["task_id"] = action.TaskID
		}
		if action.Project != "" {
			projectID, _ := resolveProjectID(ctx, action.Project)
			body["project_id"] = projectID
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/comments", nil, body, nil, true)
		cancel()
		return err
	case "comment_update":
		if action.CommentID == "" || action.Content == "" {
			return errors.New("comment_update requires comment_id and content")
		}
		body := map[string]any{"content": action.Content}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Post(reqCtx, "/comments/"+action.CommentID, nil, body, nil, true)
		cancel()
		return err
	case "comment_delete":
		if action.CommentID == "" {
			return errors.New("comment_delete requires comment_id")
		}
		reqCtx, cancel := requestContext(ctx)
		_, err := ctx.Client.Delete(reqCtx, "/comments/"+action.CommentID, nil)
		cancel()
		return err
	default:
		return fmt.Errorf("unsupported action type: %s", action.Type)
	}
}

func summarizeActions(actions []Action) PlanSummary {
	var s PlanSummary
	for _, a := range actions {
		switch {
		case strings.HasPrefix(a.Type, "task_"):
			s.Tasks++
		case strings.HasPrefix(a.Type, "project_"):
			s.Projects++
		case strings.HasPrefix(a.Type, "section_"):
			s.Sections++
		case strings.HasPrefix(a.Type, "label_"):
			s.Labels++
		case strings.HasPrefix(a.Type, "comment_"):
			s.Comments++
		}
	}
	return s
}

func writePlanOutput(ctx *Context, plan Plan) error {
	return writePlanPreview(ctx, plan, false)
}

func writePlanFile(path string, plan Plan) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func readPlanFile(path string, stdin io.Reader) (Plan, error) {
	var data []byte
	var err error
	if path == "-" {
		if stdin == nil {
			return Plan{}, errors.New("stdin not available")
		}
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return Plan{}, err
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func writePlanPreview(ctx *Context, plan Plan, dryRun bool) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{
			"plan":         plan,
			"dry_run":      dryRun,
			"action_count": len(plan.Actions),
			"summary":      plan.Summary,
		}, output.Meta{})
	}
	fmt.Fprintf(ctx.Stdout, "Plan: %s\n", plan.Instruction)
	if dryRun {
		fmt.Fprintln(ctx.Stdout, "DRY RUN: no actions applied")
	}
	fmt.Fprintf(ctx.Stdout, "Confirm: %s\n", plan.ConfirmToken)
	fmt.Fprintf(ctx.Stdout, "Actions: %d (tasks=%d projects=%d sections=%d labels=%d comments=%d)\n",
		len(plan.Actions), plan.Summary.Tasks, plan.Summary.Projects, plan.Summary.Sections, plan.Summary.Labels, plan.Summary.Comments)
	for i, action := range plan.Actions {
		fmt.Fprintf(ctx.Stdout, "%d. %s\n", i+1, action.Type)
	}
	return nil
}

func normalizeAndValidatePlan(plan *Plan, instruction string, now func() time.Time, expectedVersion int) error {
	if plan.Version == 0 {
		plan.Version = 1
	}
	if expectedVersion > 0 && plan.Version != expectedVersion {
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported plan version %d (expected %d)", plan.Version, expectedVersion)}
	}
	if plan.Instruction == "" {
		plan.Instruction = instruction
	}
	if plan.CreatedAt == "" {
		plan.CreatedAt = now().UTC().Format(time.RFC3339)
	}
	if plan.Summary == (PlanSummary{}) {
		plan.Summary = summarizeActions(plan.Actions)
	}
	return validatePlan(*plan, expectedVersion)
}

func validatePlan(plan Plan, expectedVersion int) error {
	if plan.ConfirmToken == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("plan missing confirm_token")}
	}
	if len(plan.Actions) == 0 {
		return &CodeError{Code: exitUsage, Err: errors.New("plan has no actions")}
	}
	if expectedVersion > 0 && plan.Version != 0 && plan.Version != expectedVersion {
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported plan version %d (expected %d)", plan.Version, expectedVersion)}
	}
	allowed := map[string]struct{}{
		"task_add":          {},
		"task_update":       {},
		"task_move":         {},
		"task_complete":     {},
		"task_reopen":       {},
		"task_delete":       {},
		"project_add":       {},
		"project_update":    {},
		"project_archive":   {},
		"project_unarchive": {},
		"project_delete":    {},
		"section_add":       {},
		"section_update":    {},
		"section_delete":    {},
		"label_add":         {},
		"label_update":      {},
		"label_delete":      {},
		"comment_add":       {},
		"comment_update":    {},
		"comment_delete":    {},
	}
	for _, a := range plan.Actions {
		if _, ok := allowed[a.Type]; !ok {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("unsupported action type: %s", a.Type)}
		}
		if err := validateActionFields(a); err != nil {
			return err
		}
	}
	return nil
}

func validateActionFields(a Action) error {
	switch a.Type {
	case "task_add":
		if a.Content == "" {
			return &CodeError{Code: exitUsage, Err: errors.New("task_add requires content")}
		}
	case "task_update", "task_move", "task_complete", "task_reopen", "task_delete":
		if a.TaskID == "" {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires task_id", a.Type)}
		}
		if a.Type == "task_move" && a.Project == "" && a.Section == "" && a.Parent == "" {
			return &CodeError{Code: exitUsage, Err: errors.New("task_move requires project, section, or parent")}
		}
	case "project_add":
		if a.Name == "" {
			return &CodeError{Code: exitUsage, Err: errors.New("project_add requires name")}
		}
	case "project_update", "project_archive", "project_unarchive", "project_delete":
		if a.ProjectID == "" {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires project_id", a.Type)}
		}
	case "section_add":
		if a.Name == "" || a.Project == "" {
			return &CodeError{Code: exitUsage, Err: errors.New("section_add requires name and project")}
		}
	case "section_update", "section_delete":
		if a.SectionID == "" {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires section_id", a.Type)}
		}
	case "label_add":
		if a.Name == "" {
			return &CodeError{Code: exitUsage, Err: errors.New("label_add requires name")}
		}
	case "label_update", "label_delete":
		if a.LabelID == "" {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires label_id", a.Type)}
		}
	case "comment_add":
		if a.Content == "" {
			return &CodeError{Code: exitUsage, Err: errors.New("comment_add requires content")}
		}
		if a.TaskID == "" && a.ProjectID == "" {
			return &CodeError{Code: exitUsage, Err: errors.New("comment_add requires task_id or project_id")}
		}
	case "comment_update", "comment_delete":
		if a.CommentID == "" {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("%s requires comment_id", a.Type)}
		}
	}
	return nil
}

func lastPlanPath(ctx *Context) string {
	if ctx.ConfigPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(ctx.ConfigPath), "last_plan.json")
}

func newConfirmToken() string {
	id := api.NewRequestID()
	if len(id) >= 4 {
		return id[:4]
	}
	return "confirm"
}

func toAnySlice[T any](items []T) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}
func applyActionsWithMode(ctx *Context, actions []Action, onError string) ([]applyResult, error) {
	if onError == "" {
		onError = "fail"
	}
	results := make([]applyResult, 0, len(actions))
	for _, action := range actions {
		err := applyAction(ctx, action)
		results = append(results, applyResult{Action: action, Error: err})
		if err != nil && onError == "fail" {
			return results, err
		}
	}
	return results, nil
}

type applyResult struct {
	Action Action `json:"action"`
	Error  error  `json:"-"`
}

func writePlanApplyResult(ctx *Context, plan Plan, results []applyResult, applyErr error) error {
	if ctx.Mode == output.ModeJSON {
		type resultJSON struct {
			Action Action `json:"action"`
			Error  string `json:"error,omitempty"`
		}
		out := struct {
			Plan    Plan         `json:"plan"`
			Results []resultJSON `json:"results"`
		}{
			Plan: plan,
		}
		for _, r := range results {
			entry := resultJSON{Action: r.Action}
			if r.Error != nil {
				entry.Error = r.Error.Error()
			}
			out.Results = append(out.Results, entry)
		}
		return output.WriteJSON(ctx.Stdout, out, output.Meta{RequestID: ctxRequestIDValue(ctx)})
	}
	fmt.Fprintf(ctx.Stdout, "Applied plan: %s\n", plan.Instruction)
	for i, r := range results {
		status := "ok"
		if r.Error != nil {
			status = "error: " + r.Error.Error()
		}
		fmt.Fprintf(ctx.Stdout, "%d. %s [%s]\n", i+1, r.Action.Type, status)
	}
	return applyErr
}
