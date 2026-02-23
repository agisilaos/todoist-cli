package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type StatsStreak struct {
	Count int    `json:"count"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type StatsGoals struct {
	DailyGoal           int         `json:"daily_goal"`
	WeeklyGoal          int         `json:"weekly_goal"`
	CurrentDailyStreak  StatsStreak `json:"current_daily_streak"`
	CurrentWeeklyStreak StatsStreak `json:"current_weekly_streak"`
	MaxDailyStreak      StatsStreak `json:"max_daily_streak"`
	MaxWeeklyStreak     StatsStreak `json:"max_weekly_streak"`
	VacationMode        bool        `json:"vacation_mode"`
	KarmaDisabled       bool        `json:"karma_disabled"`
	IgnoreDays          []int       `json:"ignore_days"`
}

type StatsDayItem struct {
	Date           string `json:"date"`
	TotalCompleted int    `json:"total_completed"`
}

type StatsWeekItem struct {
	From           string `json:"from"`
	To             string `json:"to"`
	TotalCompleted int    `json:"total_completed"`
}

type ProductivityStats struct {
	Karma           int             `json:"karma"`
	KarmaTrend      string          `json:"karma_trend"`
	KarmaLastUpdate int64           `json:"karma_last_update"`
	CompletedCount  int             `json:"completed_count"`
	DaysItems       []StatsDayItem  `json:"days_items"`
	WeekItems       []StatsWeekItem `json:"week_items"`
	Goals           StatsGoals      `json:"goals"`
}

type UpdateGoalsInput struct {
	DailyGoal    *int
	WeeklyGoal   *int
	VacationMode *bool
}

func (c *Client) FetchProductivityStats(ctx context.Context) (ProductivityStats, string, error) {
	var raw map[string]any
	reqID, err := c.Get(ctx, "/tasks/completed/stats", nil, &raw)
	if err != nil {
		return ProductivityStats{}, reqID, err
	}
	return parseProductivityStats(raw), reqID, nil
}

func (c *Client) UpdateGoals(ctx context.Context, in UpdateGoalsInput) (string, error) {
	args := map[string]any{}
	if in.DailyGoal != nil {
		args["daily_goal"] = *in.DailyGoal
	}
	if in.WeeklyGoal != nil {
		args["weekly_goal"] = *in.WeeklyGoal
	}
	if in.VacationMode != nil {
		if *in.VacationMode {
			args["vacation_mode"] = 1
		} else {
			args["vacation_mode"] = 0
		}
	}
	if len(args) == 0 {
		return "", fmt.Errorf("no goals to update")
	}
	payload, err := json.Marshal([]map[string]any{{
		"type": "update_goals",
		"uuid": NewRequestID(),
		"args": args,
	}})
	if err != nil {
		return "", err
	}
	_, requestID, err := c.syncRequest(ctx, map[string]string{"commands": string(payload)})
	return requestID, err
}

func parseProductivityStats(raw map[string]any) ProductivityStats {
	if raw == nil {
		return ProductivityStats{}
	}
	goals := mapAny(raw["goals"])
	out := ProductivityStats{
		Karma:           intAny(raw["karma"]),
		KarmaTrend:      strings.TrimSpace(fmt.Sprintf("%v", raw["karma_trend"])),
		KarmaLastUpdate: int64Any(raw["karma_last_update"]),
		CompletedCount:  intAny(raw["completed_count"]),
		Goals: StatsGoals{
			DailyGoal:           intAny(goals["daily_goal"]),
			WeeklyGoal:          intAny(goals["weekly_goal"]),
			CurrentDailyStreak:  parseStatsStreak(goals["current_daily_streak"]),
			CurrentWeeklyStreak: parseStatsStreak(goals["current_weekly_streak"]),
			MaxDailyStreak:      parseStatsStreak(goals["max_daily_streak"]),
			MaxWeeklyStreak:     parseStatsStreak(goals["max_weekly_streak"]),
			VacationMode:        boolLike(goals["vacation_mode"]),
			KarmaDisabled:       boolLike(goals["karma_disabled"]),
			IgnoreDays:          intSlice(goals["ignore_days"]),
		},
	}
	for _, item := range sliceAny(raw["days_items"]) {
		m := mapAny(item)
		if len(m) == 0 {
			continue
		}
		out.DaysItems = append(out.DaysItems, StatsDayItem{
			Date:           strings.TrimSpace(fmt.Sprintf("%v", m["date"])),
			TotalCompleted: intAny(m["total_completed"]),
		})
	}
	for _, item := range sliceAny(raw["week_items"]) {
		m := mapAny(item)
		if len(m) == 0 {
			continue
		}
		out.WeekItems = append(out.WeekItems, StatsWeekItem{
			From:           strings.TrimSpace(fmt.Sprintf("%v", m["from"])),
			To:             strings.TrimSpace(fmt.Sprintf("%v", m["to"])),
			TotalCompleted: intAny(m["total_completed"]),
		})
	}
	return out
}

func parseStatsStreak(v any) StatsStreak {
	m := mapAny(v)
	if len(m) == 0 {
		return StatsStreak{}
	}
	return StatsStreak{
		Count: intAny(m["count"]),
		Start: strings.TrimSpace(fmt.Sprintf("%v", m["start"])),
		End:   strings.TrimSpace(fmt.Sprintf("%v", m["end"])),
	}
}

func mapAny(v any) map[string]any {
	out, _ := v.(map[string]any)
	return out
}

func sliceAny(v any) []any {
	out, _ := v.([]any)
	return out
}

func intSlice(v any) []int {
	items := sliceAny(v)
	if len(items) == 0 {
		return []int{}
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		out = append(out, intAny(item))
	}
	return out
}

func intAny(v any) int {
	switch vv := v.(type) {
	case int:
		return vv
	case int32:
		return int(vv)
	case int64:
		return int(vv)
	case float64:
		return int(vv)
	case float32:
		return int(vv)
	default:
		return 0
	}
}

func int64Any(v any) int64 {
	switch vv := v.(type) {
	case int:
		return int64(vv)
	case int32:
		return int64(vv)
	case int64:
		return vv
	case float64:
		return int64(vv)
	case float32:
		return int64(vv)
	default:
		return 0
	}
}

func boolLike(v any) bool {
	switch vv := v.(type) {
	case bool:
		return vv
	case int:
		return vv != 0
	case int32:
		return vv != 0
	case int64:
		return vv != 0
	case float64:
		return vv != 0
	case float32:
		return vv != 0
	default:
		return false
	}
}
