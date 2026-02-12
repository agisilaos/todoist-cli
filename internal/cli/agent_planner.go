package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func agentPlanner(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("agent planner", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var cmd string
	var set bool
	var help bool
	fs.StringVar(&cmd, "cmd", "", "Planner command to set")
	fs.BoolVar(&set, "set", false, "Set planner command")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
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
		if ctx.Mode == output.ModeJSON {
			return output.WriteJSON(ctx.Stdout, map[string]any{"planner_cmd": cmd}, output.Meta{})
		}
		fmt.Fprintf(ctx.Stdout, "Planner command set to: %s\n", cmd)
		return nil
	}
	effective, source := resolvePlannerCmd(ctx, "", false)
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{"planner_cmd": effective, "source": source}, output.Meta{})
	}
	fmt.Fprintf(ctx.Stdout, "Planner command: %s (source: %s)\n", effective, source)
	return nil
}

func savePlannerCmd(ctx *Context, cmd string) error {
	cfgPath := ctx.ConfigPath
	if cfgPath == "" {
		var err error
		cfgPath, err = config.DefaultUserConfigPath()
		if err != nil {
			return err
		}
	}
	cfg := ctx.Config
	cfg.PlannerCmd = cmd
	return config.SaveConfig(cfgPath, cfg)
}

func resolvePlannerCmd(ctx *Context, override string, includeEnv bool) (string, string) {
	if override != "" {
		return override, "flag"
	}
	if includeEnv {
		if env := os.Getenv("TODOIST_PLANNER_CMD"); env != "" {
			return env, "env"
		}
	}
	if ctx != nil && ctx.Config.PlannerCmd != "" {
		return ctx.Config.PlannerCmd, "config"
	}
	return "", "none"
}
