package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func agentPlanner(ctx *Context, args []string) error {
	fs := newFlagSet("agent planner")
	var cmd string
	var set bool
	var help bool
	fs.StringVar(&cmd, "cmd", "", "Planner command to set")
	fs.BoolVar(&set, "set", false, "Set planner command")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help || (!set && cmd != "") {
		printAgentPlannerHelp(ctx.Stdout)
		if help {
			return nil
		}
	}
	if set {
		if cmd == "" {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("--cmd is required with --set")}
		}
		if err := savePlannerCmd(ctx, cmd); err != nil {
			return err
		}
		if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
			return writeStructuredValue(ctx, map[string]any{"planner_cmd": cmd})
		}
		fmt.Fprintf(ctx.Stdout, "Planner command set to: %s\n", cmd)
		return nil
	}
	effective, source := resolvePlannerCmd(ctx, "", false)
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		return writeStructuredValue(ctx, map[string]any{"planner_cmd": effective, "source": source})
	}
	fmt.Fprintf(ctx.Stdout, "Planner command: %s (source: %s)\n", effective, source)
	return nil
}

func savePlannerCmd(ctx *Context, cmd string) error {
	cfgPath := ctx.ConfigPath
	if cfgPath == "" {
		var err error
		cfgPath, err = config.DefaultUserConfigPathWithEnv(ctx.getenv)
		if err != nil {
			return err
		}
	}
	bounded, cancel := context.WithTimeout(operationContext(ctx), 2*time.Second)
	defer cancel()
	unlock, err := (credentials.Disk{}).Lock(bounded, filepath.Join(filepath.Dir(cfgPath), ".todoist-config.lock"))
	if err != nil {
		return fmt.Errorf("lock configuration before setting planner: %w", err)
	}
	defer unlock()
	return config.SetPlannerCommand(cfgPath, cmd)
}

func resolvePlannerCmd(ctx *Context, override string, includeEnv bool) (string, string) {
	if override != "" {
		return override, "flag"
	}
	if includeEnv {
		if env := ctx.getenv("TODOIST_PLANNER_CMD"); env != "" {
			return env, "env"
		}
	}
	if ctx != nil && ctx.Config.PlannerCmd != "" {
		return ctx.Config.PlannerCmd, "config"
	}
	return "", "none"
}
