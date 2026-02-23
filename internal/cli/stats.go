package cli

import (
	"fmt"
	"strconv"

	appstats "github.com/agisilaos/todoist-cli/internal/app/stats"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func statsCommand(ctx *Context, args []string) error {
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

Notes:
  - Shows productivity stats, goals, and completion progress.

Examples:
  todoist stats
  todoist stats --json
`)
}
