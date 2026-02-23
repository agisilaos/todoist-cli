package stats

import (
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
)

type Summary struct {
	Karma          int                   `json:"karma"`
	KarmaTrend     string                `json:"karma_trend"`
	CompletedCount int                   `json:"completed_count"`
	TodayCompleted int                   `json:"today_completed"`
	WeekCompleted  int                   `json:"week_completed"`
	DailyGoal      int                   `json:"daily_goal"`
	WeeklyGoal     int                   `json:"weekly_goal"`
	DailyStreak    int                   `json:"daily_streak"`
	WeeklyStreak   int                   `json:"weekly_streak"`
	VacationMode   bool                  `json:"vacation_mode"`
	IgnoreDays     []int                 `json:"ignore_days"`
	Raw            api.ProductivityStats `json:"raw,omitempty"`
}

func BuildSummary(stats api.ProductivityStats, now time.Time) Summary {
	out := Summary{
		Karma:          stats.Karma,
		KarmaTrend:     strings.ToLower(strings.TrimSpace(stats.KarmaTrend)),
		CompletedCount: stats.CompletedCount,
		DailyGoal:      stats.Goals.DailyGoal,
		WeeklyGoal:     stats.Goals.WeeklyGoal,
		DailyStreak:    stats.Goals.CurrentDailyStreak.Count,
		WeeklyStreak:   stats.Goals.CurrentWeeklyStreak.Count,
		VacationMode:   stats.Goals.VacationMode,
		IgnoreDays:     stats.Goals.IgnoreDays,
		Raw:            stats,
	}

	today := now.UTC().Format("2006-01-02")
	for _, item := range stats.DaysItems {
		if item.Date == today {
			out.TodayCompleted = item.TotalCompleted
			break
		}
	}
	if len(stats.WeekItems) > 0 {
		out.WeekCompleted = stats.WeekItems[0].TotalCompleted
	}
	return out
}

func TrendArrow(trend string) string {
	switch strings.ToLower(strings.TrimSpace(trend)) {
	case "up":
		return "up"
	case "down":
		return "down"
	default:
		return ""
	}
}
