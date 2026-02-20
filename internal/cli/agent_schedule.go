package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type scheduleSpec struct {
	Weekday int
	Hour    int
	Minute  int
}

func agentSchedule(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printAgentScheduleHelp(ctx.Stdout)
		if len(args) == 0 {
			return &CodeError{Code: exitUsage, Err: errors.New("subcommand is required")}
		}
		return nil
	}
	switch args[0] {
	case "print":
		return agentSchedulePrint(ctx, args[1:])
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown schedule subcommand: %s", args[0])}
	}
}

func agentSchedulePrint(ctx *Context, args []string) error {
	fs := newFlagSet("agent schedule print")
	var weekly string
	var planner string
	var instruction string
	var planPath string
	var confirm string
	var force bool
	var dryRun bool
	var onError string
	var expectedVersion int
	var contextProjects multiValue
	var contextLabels multiValue
	var contextCompleted string
	var cron bool
	var binPath string
	var help bool
	fs.StringVar(&weekly, "weekly", "", `Weekly schedule, e.g. "sat 09:00"`)
	fs.StringVar(&planner, "planner", "", "Planner command")
	fs.StringVar(&instruction, "instruction", "", "Instruction to plan/apply")
	fs.StringVar(&planPath, "plan", "", "Plan file (or - for stdin)")
	fs.StringVar(&confirm, "confirm", "", "Confirmation token")
	fs.BoolVar(&force, "force", false, "Skip confirmation prompts")
	fs.BoolVar(&dryRun, "dry-run", false, "Preview only")
	fs.StringVar(&onError, "on-error", "fail", "On error: fail|continue")
	fs.IntVar(&expectedVersion, "plan-version", 1, "Expected plan version")
	fs.Var(&contextProjects, "context-project", "Project context (repeatable)")
	fs.Var(&contextLabels, "context-label", "Label context (repeatable)")
	fs.StringVar(&contextCompleted, "context-completed", "", "Include completed tasks from last Nd (e.g. 7d)")
	fs.BoolVar(&cron, "cron", false, "Print cron entry")
	fs.StringVar(&binPath, "bin", "", "Path to todoist binary (defaults to current executable)")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printAgentScheduleHelp(ctx.Stdout)
		return nil
	}
	if weekly == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("--weekly is required")}
	}
	spec, err := parseWeeklySpec(weekly)
	if err != nil {
		return err
	}
	if binPath == "" {
		exe, err := os.Executable()
		if err == nil && exe != "" {
			binPath = exe
		}
	}
	if binPath == "" {
		binPath = "todoist"
	}
	if onError != "fail" && onError != "continue" {
		return &CodeError{Code: exitUsage, Err: errors.New("invalid --on-error; must be fail or continue")}
	}
	runArgs := buildAgentRunArgs(agentRunOptions{
		PlanPath:         planPath,
		Instruction:      instruction,
		Planner:          planner,
		Confirm:          confirm,
		OnError:          onError,
		ExpectedVersion:  expectedVersion,
		Force:            force,
		DryRun:           dryRun,
		ContextProjects:  contextProjects,
		ContextLabels:    contextLabels,
		ContextCompleted: contextCompleted,
	})
	if cron {
		line := cronLine(spec, binPath, runArgs)
		fmt.Fprintln(ctx.Stdout, line)
		return nil
	}
	fmt.Fprint(ctx.Stdout, launchdPlist(spec, binPath, runArgs))
	return nil
}

func buildAgentRunArgs(opts agentRunOptions) []string {
	args := []string{"agent", "run"}
	if opts.PlanPath != "" {
		args = append(args, "--plan", opts.PlanPath)
	}
	if opts.Instruction != "" {
		args = append(args, "--instruction", opts.Instruction)
	}
	if opts.Planner != "" {
		args = append(args, "--planner", opts.Planner)
	}
	if opts.Confirm != "" {
		args = append(args, "--confirm", opts.Confirm)
	}
	for _, proj := range opts.ContextProjects {
		args = append(args, "--context-project", proj)
	}
	for _, label := range opts.ContextLabels {
		args = append(args, "--context-label", label)
	}
	if opts.ContextCompleted != "" {
		args = append(args, "--context-completed", opts.ContextCompleted)
	}
	if opts.Force {
		args = append(args, "--force")
	}
	if opts.DryRun {
		args = append(args, "--dry-run")
	}
	if opts.OnError != "" && opts.OnError != "fail" {
		args = append(args, "--on-error", opts.OnError)
	}
	if opts.ExpectedVersion > 0 {
		args = append(args, "--plan-version", strconv.Itoa(opts.ExpectedVersion))
	}
	return args
}

func parseWeeklySpec(input string) (scheduleSpec, error) {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(input)))
	if len(parts) != 2 {
		return scheduleSpec{}, &CodeError{Code: exitUsage, Err: errors.New("weekly spec must be like \"sat 09:00\"")}
	}
	weekday, ok := mapWeekday(parts[0])
	if !ok {
		return scheduleSpec{}, &CodeError{Code: exitUsage, Err: fmt.Errorf("invalid weekday: %s", parts[0])}
	}
	hour, minute, err := parseTime(parts[1])
	if err != nil {
		return scheduleSpec{}, err
	}
	return scheduleSpec{Weekday: weekday, Hour: hour, Minute: minute}, nil
}

func mapWeekday(day string) (int, bool) {
	switch day {
	case "sun", "sunday":
		return 1, true
	case "mon", "monday":
		return 2, true
	case "tue", "tues", "tuesday":
		return 3, true
	case "wed", "weds", "wednesday":
		return 4, true
	case "thu", "thur", "thurs", "thursday":
		return 5, true
	case "fri", "friday":
		return 6, true
	case "sat", "saturday":
		return 7, true
	default:
		return 0, false
	}
}

func parseTime(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, &CodeError{Code: exitUsage, Err: errors.New("time must be HH:MM")}
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, &CodeError{Code: exitUsage, Err: errors.New("invalid hour in time")}
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, &CodeError{Code: exitUsage, Err: errors.New("invalid minute in time")}
	}
	return h, m, nil
}

func cronLine(spec scheduleSpec, binPath string, args []string) string {
	weekday := cronWeekday(spec.Weekday)
	cmd := strings.Join(append([]string{shellEscape(binPath)}, escapeArgs(args)...), " ")
	return fmt.Sprintf("%d %d * * %d %s", spec.Minute, spec.Hour, weekday, cmd)
}

func cronWeekday(launchdWeekday int) int {
	switch launchdWeekday {
	case 1:
		return 0
	case 2:
		return 1
	case 3:
		return 2
	case 4:
		return 3
	case 5:
		return 4
	case 6:
		return 5
	case 7:
		return 6
	default:
		return 0
	}
}

func launchdPlist(spec scheduleSpec, binPath string, args []string) string {
	label := "com.todoist.agent.weekly"
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString(`  <dict>` + "\n")
	b.WriteString(fmt.Sprintf("    <key>Label</key>\n    <string>%s</string>\n", label))
	b.WriteString("    <key>ProgramArguments</key>\n    <array>\n")
	b.WriteString(fmt.Sprintf("      <string>%s</string>\n", binPath))
	for _, arg := range args {
		b.WriteString(fmt.Sprintf("      <string>%s</string>\n", arg))
	}
	b.WriteString("    </array>\n")
	b.WriteString("    <key>StartCalendarInterval</key>\n    <dict>\n")
	b.WriteString(fmt.Sprintf("      <key>Weekday</key>\n      <integer>%d</integer>\n", spec.Weekday))
	b.WriteString(fmt.Sprintf("      <key>Hour</key>\n      <integer>%d</integer>\n", spec.Hour))
	b.WriteString(fmt.Sprintf("      <key>Minute</key>\n      <integer>%d</integer>\n", spec.Minute))
	b.WriteString("    </dict>\n")
	b.WriteString(fmt.Sprintf("    <key>StandardOutPath</key>\n    <string>%s</string>\n", filepath.Join(os.TempDir(), "todoist-agent.log")))
	b.WriteString(fmt.Sprintf("    <key>StandardErrorPath</key>\n    <string>%s</string>\n", filepath.Join(os.TempDir(), "todoist-agent.err")))
	b.WriteString("  </dict>\n</plist>\n")
	return b.String()
}

func shellEscape(value string) string {
	if value == "" {
		return "''"
	}
	if strings.ContainsAny(value, " \t\"'\\") {
		return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
	}
	return value
}

func escapeArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, shellEscape(arg))
	}
	return out
}
