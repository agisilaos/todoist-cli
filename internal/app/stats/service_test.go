package stats

import (
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func TestBuildSummary(t *testing.T) {
	summary := BuildSummary(api.ProductivityStats{
		Karma:          1200,
		KarmaTrend:     "UP",
		CompletedCount: 400,
		DaysItems:      []api.StatsDayItem{{Date: "2026-02-23", TotalCompleted: 4}},
		WeekItems:      []api.StatsWeekItem{{From: "2026-02-17", To: "2026-02-23", TotalCompleted: 18}},
		Goals: api.StatsGoals{
			DailyGoal:           5,
			WeeklyGoal:          25,
			CurrentDailyStreak:  api.StatsStreak{Count: 3},
			CurrentWeeklyStreak: api.StatsStreak{Count: 2},
			VacationMode:        true,
			IgnoreDays:          []int{0, 6},
		},
	}, time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC))

	if summary.TodayCompleted != 4 || summary.WeekCompleted != 18 {
		t.Fatalf("unexpected completion summary: %#v", summary)
	}
	if summary.KarmaTrend != "up" || !summary.VacationMode {
		t.Fatalf("unexpected summary fields: %#v", summary)
	}
}

func TestTrendArrow(t *testing.T) {
	if got := TrendArrow("up"); got == "" {
		t.Fatalf("expected arrow for up")
	}
	if got := TrendArrow("down"); got == "" {
		t.Fatalf("expected arrow for down")
	}
	if got := TrendArrow("flat"); got != "" {
		t.Fatalf("expected empty for flat, got %q", got)
	}
}
