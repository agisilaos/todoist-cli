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
	if err := parseFlagSetInterspersed(fs, args); err != nil {
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
	var policyPath string
	var onError string
	var expectedVersion int
	var contextProjects multiValue
	var contextLabels multiValue
	var contextCompleted string
	var help bool
	fs.StringVar(&planPath, "plan", "", "Plan file (or - for stdin)")
	fs.StringVar(&confirm, "confirm", "", "Confirmation token")
	fs.StringVar(&planner, "planner", "", "Planner command")
	fs.StringVar(&policyPath, "policy", "", "Policy file path")
	fs.StringVar(&onError, "on-error", "fail", "On error: fail|continue")
	fs.IntVar(&expectedVersion, "plan-version", 1, "Expected plan version")
	fs.Var(&contextProjects, "context-project", "Project context (repeatable)")
	fs.Var(&contextLabels, "context-label", "Label context (repeatable)")
	fs.StringVar(&contextCompleted, "context-completed", "", "Include completed tasks from last Nd (e.g. 7d)")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printAgentHelp(ctx.Stdout)
		return nil
	}
	emitProgress(ctx, "agent_apply_start", map[string]any{
		"command": "agent apply",
	})
	var plan Plan
	if planPath != "" {
		var err error
		plan, err = readPlanFile(planPath, ctx.Stdin)
		if err != nil {
			emitProgress(ctx, "agent_apply_error", map[string]any{"error": err.Error()})
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
			emitProgress(ctx, "agent_apply_error", map[string]any{"error": err.Error()})
			return err
		}
		p, err := runPlanner(ctx, planner, instruction, expectedVersion, ctxOpts)
		if err != nil {
			return err
		}
		plan = p
	}
	if err := validatePlan(plan, expectedVersion); err != nil {
		emitProgress(ctx, "agent_apply_error", map[string]any{"error": err.Error()})
		return err
	}
	policy, err := loadAgentPolicy(ctx, policyPath)
	if err != nil {
		emitProgress(ctx, "agent_apply_error", map[string]any{"error": err.Error()})
		return err
	}
	if err := enforceAgentPolicy(plan, policy); err != nil {
		emitProgress(ctx, "agent_apply_error", map[string]any{"error": err.Error()})
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
		emitProgress(ctx, "agent_apply_complete", map[string]any{"dry_run": true, "action_count": len(plan.Actions)})
		return writePlanPreview(ctx, plan, true)
	}
	if err := ensureClient(ctx); err != nil {
		emitProgress(ctx, "agent_apply_error", map[string]any{"error": err.Error()})
		return err
	}
	results, err := applyActionsWithMode(ctx, plan.ConfirmToken, plan.Actions, onError)
	if err != nil && onError == "fail" {
		emitProgress(ctx, "agent_apply_error", map[string]any{"error": err.Error()})
		return err
	}
	plan.AppliedAt = ctx.Now().UTC().Format(time.RFC3339)
	if err := writePlanFile(lastPlanPath(ctx), plan); err != nil {
		emitProgress(ctx, "agent_apply_error", map[string]any{"error": err.Error()})
		return err
	}
	emitProgress(ctx, "agent_apply_complete", map[string]any{"action_count": len(plan.Actions)})
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
	emitProgress(ctx, "agent_planner_start", map[string]any{"instruction": instruction})
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
	emitProgress(ctx, "agent_planner_complete", map[string]any{"action_count": len(plan.Actions)})
	return plan, nil
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
		payload := map[string]any{
			"plan":         plan,
			"dry_run":      dryRun,
			"action_count": len(plan.Actions),
			"summary":      plan.Summary,
		}
		return output.WriteJSON(ctx.Stdout, payload, output.Meta{})
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
func applyActionsWithMode(ctx *Context, confirmToken string, actions []Action, onError string) ([]applyResult, error) {
	if onError == "" {
		onError = "fail"
	}
	journal, journalPath, err := loadReplayJournal(ctx)
	if err != nil {
		return nil, err
	}
	results := make([]applyResult, 0, len(actions))
	for idx, action := range actions {
		emitProgress(ctx, "agent_action_start", map[string]any{"index": idx, "action_type": action.Type})
		replayKey := makeReplayKey(confirmToken, idx, action)
		if _, ok := journal.Applied[replayKey]; ok {
			results = append(results, applyResult{Action: action, SkippedReplay: true})
			emitProgress(ctx, "agent_action_skipped_replay", map[string]any{"index": idx, "action_type": action.Type})
			continue
		}
		err := applyAction(ctx, action)
		results = append(results, applyResult{Action: action, Error: err})
		if err != nil {
			emitProgress(ctx, "agent_action_error", map[string]any{"index": idx, "action_type": action.Type, "error": err.Error()})
		} else {
			nowFn := time.Now
			if ctx != nil && ctx.Now != nil {
				nowFn = ctx.Now
			}
			markReplayApplied(&journal, replayKey, nowFn())
			emitProgress(ctx, "agent_action_complete", map[string]any{"index": idx, "action_type": action.Type})
		}
		if err != nil && onError == "fail" {
			_ = saveReplayJournal(journalPath, journal)
			return results, err
		}
	}
	if err := saveReplayJournal(journalPath, journal); err != nil {
		return results, err
	}
	return results, nil
}

type applyResult struct {
	Action        Action `json:"action"`
	Error         error  `json:"-"`
	SkippedReplay bool   `json:"skipped_replay,omitempty"`
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
			if r.SkippedReplay {
				entry.Error = "skipped_replay"
				out.Results = append(out.Results, entry)
				continue
			}
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
		if r.SkippedReplay {
			status = "skipped (replay)"
		}
		if r.Error != nil {
			status = "error: " + r.Error.Error()
		}
		fmt.Fprintf(ctx.Stdout, "%d. %s [%s]\n", i+1, r.Action.Type, status)
	}
	return applyErr
}
