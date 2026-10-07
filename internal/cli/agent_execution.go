package cli

import (
	"errors"
	"strings"
	"time"

	appagent "github.com/agisilaos/todoist-cli/internal/app/agent"
)

type agentExecutionOptions struct {
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

// executeAgentPlan owns the shared confirmed-plan workflow. Command adapters
// retain parsing; run's optional export occurs before preview or application.
func executeAgentPlan(ctx *Context, command string, opts agentExecutionOptions) error {
	event := strings.ReplaceAll(command, " ", "_")
	emitProgress(ctx, event+"_start", map[string]any{
		"command": command,
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
		EnforcePolicy: func(plan Plan) error {
			policy, err := loadAgentPolicy(ctx, opts.PolicyPath)
			if err != nil {
				return err
			}
			return enforceAgentPolicy(plan, policy)
		},
	})
	if err != nil {
		var validation *appagent.PlanValidationError
		if errors.As(err, &validation) {
			err = &CodeError{Code: exitUsage, Err: validation.Err}
		}
		if command == "agent apply" {
			if codeErr, ok := err.(*CodeError); ok && codeErr.Code == exitUsage {
				printAgentHelp(ctx.Stderr)
			}
		}
		emitProgress(ctx, event+"_error", map[string]any{"error": err.Error()})
		return err
	}
	source := "planner"
	if strings.TrimSpace(opts.PlanPath) != "" {
		source = "plan_file"
	}
	emitAgentPlanLoaded(ctx, command, len(plan.Actions), source)
	if command == "agent run" && plan.Review != nil && opts.OutPath != "-" {
		if err := validateReviewOutputPath(ctx, opts.OutPath); err != nil {
			return err
		}
	}
	if opts.OutPath != "" && opts.OutPath != "-" {
		var saveErr error
		if plan.Review != nil {
			saveErr = saveReviewPlan(opts.OutPath, plan)
		} else {
			saveErr = writePlanFile(opts.OutPath, plan)
		}
		if saveErr != nil {
			return saveErr
		}
	}
	if opts.DryRun {
		emitAgentApplySummary(ctx, command, nil, true, nil)
		emitProgress(ctx, event+"_complete", map[string]any{"dry_run": true, "action_count": len(plan.Actions)})
		return writePlanPreview(ctx, plan, true)
	}
	if err := ensureClient(ctx); err != nil {
		emitProgress(ctx, event+"_error", map[string]any{"error": err.Error()})
		return err
	}
	if plan.Review != nil {
		return applyReviewAndReport(ctx, plan, opts.PlanPath, opts.OnError, command)
	}
	applyMode := applyErrorMode(opts.OnError)
	results, applyErr := applyActionsWithMode(ctx, plan.ConfirmToken, plan.Actions, applyMode)
	if shouldAbortApply(ctx, applyMode, applyErr) {
		emitAgentApplySummary(ctx, command, results, false, applyErr)
		emitProgress(ctx, event+"_error", map[string]any{"error": applyErr.Error()})
		return applyErr
	}
	_, _, replayed := summarizeApplyResults(results)
	if replayed != len(results) {
		plan.AppliedAt = ctx.Now().UTC().Format(time.RFC3339)
		if err := writePlanFile(lastPlanPath(ctx), plan); err != nil {
			emitAgentApplySummary(ctx, command, results, false, err)
			emitProgress(ctx, event+"_error", map[string]any{"error": err.Error()})
			return err
		}
	}
	emitAgentApplySummary(ctx, command, results, false, applyErr)
	emitProgress(ctx, event+"_complete", map[string]any{"action_count": len(plan.Actions)})
	return writePlanApplyResult(ctx, plan, results, applyErr)
}
