package cli

import (
	"errors"
	"flag"
	"io"
	"strings"
	"time"
)

type agentRunOptions struct {
	PlanPath        string
	Instruction     string
	Planner         string
	Confirm         string
	OnError         string
	ExpectedVersion int
	Force           bool
	DryRun          bool
	OutPath         string
}

func agentRun(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("agent run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts agentRunOptions
	fs.StringVar(&opts.PlanPath, "plan", "", "Plan file (or - for stdin)")
	fs.StringVar(&opts.Instruction, "instruction", "", "Instruction to plan/apply")
	fs.StringVar(&opts.Planner, "planner", "", "Planner command")
	fs.StringVar(&opts.Confirm, "confirm", "", "Confirmation token")
	fs.StringVar(&opts.OnError, "on-error", "fail", "On error: fail|continue")
	fs.IntVar(&opts.ExpectedVersion, "plan-version", 1, "Expected plan version")
	fs.StringVar(&opts.OutPath, "out", "", "Write plan output to file")
	var help bool
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
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

	plan, err := loadOrPlan(ctx, opts)
	if err != nil {
		return err
	}
	if opts.OutPath != "" && opts.OutPath != "-" {
		if err := writePlanFile(opts.OutPath, plan); err != nil {
			return err
		}
	}
	if err := validatePlan(plan, opts.ExpectedVersion); err != nil {
		return err
	}
	if !opts.Force {
		if opts.Confirm == "" {
			return &CodeError{Code: exitUsage, Err: errors.New("--confirm is required (or use --force)")}
		}
		if plan.ConfirmToken != "" && opts.Confirm != plan.ConfirmToken {
			return &CodeError{Code: exitUsage, Err: errors.New("confirmation token does not match plan")}
		}
	}
	if opts.DryRun {
		return writePlanPreview(ctx, plan, true)
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	results, applyErr := applyActionsWithMode(ctx, plan.Actions, opts.OnError)
	if applyErr != nil && opts.OnError == "fail" {
		return applyErr
	}
	plan.AppliedAt = ctx.Now().UTC().Format(time.RFC3339)
	if err := writePlanFile(lastPlanPath(ctx), plan); err != nil {
		return err
	}
	return writePlanApplyResult(ctx, plan, results, applyErr)
}

func loadOrPlan(ctx *Context, opts agentRunOptions) (Plan, error) {
	if opts.PlanPath != "" {
		return readPlanFile(opts.PlanPath, ctx.Stdin)
	}
	if strings.TrimSpace(opts.Instruction) == "" {
		return Plan{}, &CodeError{Code: exitUsage, Err: errors.New("instruction is required when --plan is not provided")}
	}
	return runPlanner(ctx, opts.Planner, opts.Instruction, opts.ExpectedVersion)
}
