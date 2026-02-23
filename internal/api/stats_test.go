package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestFetchProductivityStats(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/tasks/completed/stats" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		payload := `{
			"karma": 1234,
			"karma_trend": "up",
			"karma_last_update": 1700000000,
			"completed_count": 456,
			"days_items": [{"date":"2026-02-23","total_completed":3}],
			"week_items": [{"from":"2026-02-17","to":"2026-02-23","total_completed":19}],
			"goals": {
				"daily_goal": 5,
				"weekly_goal": 25,
				"current_daily_streak": {"count": 4, "start": "2026-02-20", "end": "2026-02-23"},
				"current_weekly_streak": {"count": 2, "start": "2026-02-10", "end": "2026-02-23"},
				"max_daily_streak": {"count": 10, "start": "2025-12-01", "end": "2025-12-10"},
				"max_weekly_streak": {"count": 8, "start": "2025-10-01", "end": "2025-11-26"},
				"vacation_mode": 0,
				"karma_disabled": 1,
				"ignore_days": [0,6]
			}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(payload)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	out, _, err := client.FetchProductivityStats(context.Background())
	if err != nil {
		t.Fatalf("FetchProductivityStats: %v", err)
	}
	if out.Karma != 1234 || out.KarmaTrend != "up" || out.CompletedCount != 456 {
		t.Fatalf("unexpected top-level stats: %#v", out)
	}
	if out.Goals.DailyGoal != 5 || out.Goals.WeeklyGoal != 25 || !out.Goals.KarmaDisabled {
		t.Fatalf("unexpected goals: %#v", out.Goals)
	}
	if len(out.DaysItems) != 1 || out.DaysItems[0].TotalCompleted != 3 {
		t.Fatalf("unexpected days_items: %#v", out.DaysItems)
	}
	if len(out.WeekItems) != 1 || out.WeekItems[0].TotalCompleted != 19 {
		t.Fatalf("unexpected week_items: %#v", out.WeekItems)
	}
}
