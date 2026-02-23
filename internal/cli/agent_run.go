package cli

import (
	"errors"
	"strings"
	"time"

	appagent "github.com/agisilaos/todoist-cli/internal/app/agent"
)

type agentRunOptions struct {
	PlanPath         string
	Instruction      string
	Planner          string
	Confirm          string
	OnError          string
	ExpectedVersion  int
	Force            bool
	DryRun           bool
	OutPath          string
	ContextProjects  []string
	ContextLabels    []string
	ContextCompleted string
	PolicyPath       string
}

func agentRun(ctx *Context, args []string) error {
	fs := newFlagSet("agent run")
	var opts agentRunOptions
	fs.StringVar(&opts.PlanPath, "plan", "", "Plan file (or - for stdin)")
	fs.StringVar(&opts.Instruction, "instruction", "", "Instruction to plan/apply")
	fs.StringVar(&opts.Planner, "planner", "", "Planner command")
	fs.StringVar(&opts.Confirm, "confirm", "", "Confirmation token")
	fs.StringVar(&opts.OnError, "on-error", "fail", "On error: fail|continue")
	fs.IntVar(&opts.ExpectedVersion, "plan-version", 1, "Expected plan version")
	fs.StringVar(&opts.OutPath, "out", "", "Write plan output to file")
	fs.StringVar(&opts.PolicyPath, "policy", "", "Policy file path")
	var contextProjects multiValue
	var contextLabels multiValue
	fs.Var(&contextProjects, "context-project", "Project context (repeatable)")
	fs.Var(&contextLabels, "context-label", "Label context (repeatable)")
	fs.StringVar(&opts.ContextCompleted, "context-completed", "", "Include completed tasks from last Nd (e.g. 7d)")
	var help bool
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printAgentHelp(ctx.Stdout)
		return nil
	}
	if opts.OnError != "fail" && opts.OnError != "continue" {
		return &CodeError{Code: exitUsage, Err: errors.New("invalid --on-error; must be fail or continue")}
	}
	if opts.Instruction == "" && len(fs.Args()) > 0 {
		opts.Instruction = strings.Join(fs.Args(), " ")
	}
	opts.Force = ctx.Global.Force
	opts.DryRun = ctx.Global.DryRun
	opts.ContextProjects = contextProjects
	opts.ContextLabels = contextLabels
	emitProgress(ctx, "agent_run_start", map[string]any{
		"command": "agent run",
	})

	plan, err := appagent.PreparePlan(appagent.PrepareInput{
		PlanPath:        opts.PlanPath,
		Instruction:     opts.Instruction,
		Confirm:         opts.Confirm,
		ExpectedVersion: opts.ExpectedVersion,
		Force:           opts.Force,
		DryRun:          opts.DryRun,
	}, appagent.PrepareDeps{
		LoadPlan: func(path string) (Plan, error) {
			return readPlanFile(path, ctx.Stdin)
		},
		Plan: func(instruction string) (Plan, error) {
			ctxOpts, err := parseContextOptions(ctx, opts.ContextProjects, opts.ContextLabels, opts.ContextCompleted)
			if err != nil {
				return Plan{}, err
			}
			return runPlanner(ctx, opts.Planner, instruction, opts.ExpectedVersion, ctxOpts)
		},
		ValidatePlan: func(plan Plan, expectedVersion int, allowEmptyActions bool) error {
			return validatePlan(plan, expectedVersion, allowEmptyActions)
		},
		EnforcePolicy: func(plan Plan) error {
			policy, err := loadAgentPolicy(ctx, opts.PolicyPath)
			if err != nil {
				return err
			}
			return enforceAgentPolicy(plan, policy)
		},
	})
	if err != nil {
		emitProgress(ctx, "agent_run_error", map[string]any{"error": err.Error()})
		return err
	}
	source := "planner"
	if strings.TrimSpace(opts.PlanPath) != "" {
		source = "plan_file"
	}
	emitAgentPlanLoaded(ctx, "agent run", len(plan.Actions), source)
	if opts.OutPath != "" && opts.OutPath != "-" {
		if err := writePlanFile(opts.OutPath, plan); err != nil {
			return err
		}
	}
	if opts.DryRun {
		emitAgentApplySummary(ctx, "agent run", nil, true, nil)
		emitProgress(ctx, "agent_run_complete", map[string]any{"dry_run": true, "action_count": len(plan.Actions)})
		return writePlanPreview(ctx, plan, true)
	}
	if err := ensureClient(ctx); err != nil {
		emitProgress(ctx, "agent_run_error", map[string]any{"error": err.Error()})
		return err
	}
	results, applyErr := applyActionsWithMode(ctx, plan.ConfirmToken, plan.Actions, opts.OnError)
	if applyErr != nil && opts.OnError == "fail" {
		emitAgentApplySummary(ctx, "agent run", results, false, applyErr)
		emitProgress(ctx, "agent_run_error", map[string]any{"error": applyErr.Error()})
		return applyErr
	}
	plan.AppliedAt = ctx.Now().UTC().Format(time.RFC3339)
	if err := writePlanFile(lastPlanPath(ctx), plan); err != nil {
		emitAgentApplySummary(ctx, "agent run", results, false, err)
		emitProgress(ctx, "agent_run_error", map[string]any{"error": err.Error()})
		return err
	}
	emitAgentApplySummary(ctx, "agent run", results, false, applyErr)
	emitProgress(ctx, "agent_run_complete", map[string]any{"action_count": len(plan.Actions)})
	return writePlanApplyResult(ctx, plan, results, applyErr)
}
