package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

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
