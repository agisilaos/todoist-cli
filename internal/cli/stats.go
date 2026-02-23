package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	appstats "github.com/agisilaos/todoist-cli/internal/app/stats"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func statsCommand(ctx *Context, args []string) error {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch canonicalSubcommand(args[0], nil) {
		case "goals":
			return statsGoalsCommand(ctx, args[1:])
		case "vacation":
			return statsVacationCommand(ctx, args[1:])
		case "help":
			printStatsHelp(ctx.Stdout)
			return nil
		default:
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown stats subcommand: %s", args[0])}
		}
	}
	fs := newFlagSet("stats")
	var help bool
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printStatsHelp(ctx.Stdout)
		return nil
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	reqCtx, cancel := requestContext(ctx)
	stats, reqID, err := ctx.Client.FetchProductivityStats(reqCtx)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeStatsSummary(ctx, appstats.BuildSummary(stats, ctx.Now()))
}

func statsGoalsCommand(ctx *Context, args []string) error {
	fs := newFlagSet("stats goals")
	var dailyRaw string
	var weeklyRaw string
	var help bool
	fs.StringVar(&dailyRaw, "daily", "", "Set daily goal")
	fs.StringVar(&weeklyRaw, "weekly", "", "Set weekly goal")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printStatsHelp(ctx.Stdout)
		return nil
	}
	daily, err := appstats.ParseGoalValue(dailyRaw)
	if err != nil {
		return &CodeError{Code: exitUsage, Err: errors.New("daily goal must be a non-negative integer")}
	}
	weekly, err := appstats.ParseGoalValue(weeklyRaw)
	if err != nil {
		return &CodeError{Code: exitUsage, Err: errors.New("weekly goal must be a non-negative integer")}
	}
	if err := appstats.ValidateGoalsUpdate(daily, weekly); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	input := api.UpdateGoalsInput{DailyGoal: daily, WeeklyGoal: weekly}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "stats goals", map[string]any{"daily": daily, "weekly": weekly})
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.UpdateGoals(reqCtx, input)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	return writeSimpleResult(ctx, "updated", "goals")
}

func statsVacationCommand(ctx *Context, args []string) error {
	fs := newFlagSet("stats vacation")
	var on bool
	var off bool
	var help bool
	fs.BoolVar(&on, "on", false, "Enable vacation mode")
	fs.BoolVar(&off, "off", false, "Disable vacation mode")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printStatsHelp(ctx.Stdout)
		return nil
	}
	mode, err := appstats.ResolveVacationMode(on, off)
	if err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	input := api.UpdateGoalsInput{VacationMode: mode}
	if ctx.Global.DryRun {
		return writeDryRun(ctx, "stats vacation", map[string]any{"vacation_mode": *mode})
	}
	if err := ensureClient(ctx); err != nil {
		return err
	}
	reqCtx, cancel := requestContext(ctx)
	reqID, err := ctx.Client.UpdateGoals(reqCtx, input)
	cancel()
	if err != nil {
		return err
	}
	setRequestID(ctx, reqID)
	if *mode {
		return writeSimpleResult(ctx, "enabled", "vacation_mode")
	}
	return writeSimpleResult(ctx, "disabled", "vacation_mode")
}

func writeStatsSummary(ctx *Context, summary appstats.Summary) error {
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, summary, output.Meta{RequestID: ctx.RequestID})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSONSlice(ctx.Stdout, []appstats.Summary{summary})
	}
	if ctx.Mode == output.ModePlain {
		rows := [][]string{
			{"karma", strconv.Itoa(summary.Karma)},
			{"karma_trend", summary.KarmaTrend},
			{"completed_count", strconv.Itoa(summary.CompletedCount)},
			{"today_completed", strconv.Itoa(summary.TodayCompleted)},
			{"week_completed", strconv.Itoa(summary.WeekCompleted)},
			{"daily_goal", strconv.Itoa(summary.DailyGoal)},
			{"weekly_goal", strconv.Itoa(summary.WeeklyGoal)},
			{"daily_streak", strconv.Itoa(summary.DailyStreak)},
			{"weekly_streak", strconv.Itoa(summary.WeeklyStreak)},
			{"vacation_mode", strconv.FormatBool(summary.VacationMode)},
		}
		return output.WritePlain(ctx.Stdout, rows)
	}
	if summary.VacationMode {
		fmt.Fprintln(ctx.Stdout, "Vacation mode is on")
		fmt.Fprintln(ctx.Stdout)
	}
	trend := appstats.TrendArrow(summary.KarmaTrend)
	if trend != "" {
		fmt.Fprintf(ctx.Stdout, "Karma: %d (%s)\n", summary.Karma, trend)
	} else {
		fmt.Fprintf(ctx.Stdout, "Karma: %d\n", summary.Karma)
	}
	fmt.Fprintf(ctx.Stdout, "Daily: %d/%d completed today, streak %d\n", summary.TodayCompleted, summary.DailyGoal, summary.DailyStreak)
	fmt.Fprintf(ctx.Stdout, "Weekly: %d/%d completed this week, streak %d\n", summary.WeekCompleted, summary.WeeklyGoal, summary.WeeklyStreak)
	fmt.Fprintf(ctx.Stdout, "Completed total: %d\n", summary.CompletedCount)
	return nil
}

func printStatsHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist stats
  todoist stats goals [--daily <n>] [--weekly <n>]
  todoist stats vacation (--on | --off)

Notes:
  - Shows productivity stats, goals, and completion progress.
  - "goals" updates daily/weekly targets.
  - "vacation" toggles vacation mode.

Examples:
  todoist stats
  todoist stats --json
  todoist stats goals --daily 5 --weekly 25
  todoist stats vacation --on
`)
}
