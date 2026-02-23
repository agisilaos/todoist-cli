package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestStatsCommandJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/completed/stats" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"karma": 42,
			"karma_trend": "up",
			"completed_count": 100,
			"days_items": [{"date":"2026-02-23","total_completed":2}],
			"week_items": [{"from":"2026-02-17","to":"2026-02-23","total_completed":9}],
			"goals": {"daily_goal":5,"weekly_goal":20,"current_daily_streak":{"count":2},"current_weekly_streak":{"count":1},"vacation_mode":0,"ignore_days":[]}
		}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := &Context{
		Stdout: &out,
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeJSON,
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
		Now: func() time.Time {
			return time.Date(2026, 2, 23, 9, 0, 0, 0, time.UTC)
		},
	}
	if err := statsCommand(ctx, nil); err != nil {
		t.Fatalf("statsCommand: %v", err)
	}
	if !strings.Contains(out.String(), `"today_completed": 2`) {
		t.Fatalf("expected today_completed in output, got: %s", out.String())
	}
	if !strings.Contains(out.String(), `"week_completed": 9`) {
		t.Fatalf("expected week_completed in output, got: %s", out.String())
	}
}

func TestStatsCommandHumanVacation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"karma": 50,
			"karma_trend": "down",
			"completed_count": 101,
			"days_items": [],
			"week_items": [],
			"goals": {"daily_goal":3,"weekly_goal":15,"current_daily_streak":{"count":0},"current_weekly_streak":{"count":0},"vacation_mode":1,"ignore_days":[]}
		}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := &Context{
		Stdout: &out,
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeHuman,
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
		Now:    time.Now,
	}
	if err := statsCommand(ctx, nil); err != nil {
		t.Fatalf("statsCommand: %v", err)
	}
	if !strings.Contains(out.String(), "Vacation mode is on") {
		t.Fatalf("expected vacation mode warning, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Karma: 50") {
		t.Fatalf("expected karma line, got %q", out.String())
	}
}

func TestStatsGoalsDryRun(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"stats", "goals", "--daily", "5", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d stderr=%q", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"action": "stats goals"`) {
		t.Fatalf("expected dry-run action, got %s", stdout.String())
	}
}

func TestStatsVacationUpdatesGoals(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(r.Body)
		values, _ := url.ParseQuery(body.String())
		commands := values.Get("commands")
		if !strings.Contains(commands, `"type":"update_goals"`) || !strings.Contains(commands, `"vacation_mode":1`) {
			t.Fatalf("unexpected sync payload: %s", commands)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeJSON,
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
		Now:    time.Now,
	}
	if err := statsCommand(ctx, []string{"vacation", "--on"}); err != nil {
		t.Fatalf("statsCommand vacation: %v", err)
	}
}
