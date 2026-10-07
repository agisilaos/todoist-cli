package cli

import (
	"errors"
	"strings"
)

func agentRun(ctx *Context, args []string) error {
	fs := newFlagSet("agent run")
	var opts agentExecutionOptions
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
	return executeAgentPlan(ctx, "agent run", opts)
}
