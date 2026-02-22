package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestUpcomingCommandDefaultsToSevenDays(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"results":[
{"id":"t1","content":"Today","due":{"date":"2026-02-22"}},
{"id":"t2","content":"Tomorrow","due":{"date":"2026-02-23"}},
{"id":"t3","content":"Day7","due":{"date":"2026-02-28"}},
{"id":"t4","content":"Day8","due":{"date":"2026-03-01"}},
{"id":"t5","content":"No due"}
],"next_cursor":""}`))
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
			return time.Date(2026, 2, 22, 9, 0, 0, 0, time.UTC)
		},
	}
	if err := upcomingCommand(ctx, nil); err != nil {
		t.Fatalf("upcomingCommand: %v", err)
	}
	var tasks []api.Task
	if err := json.Unmarshal(out.Bytes(), &tasks); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "t1" || tasks[1].ID != "t2" || tasks[2].ID != "t3" {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
}

func TestUpcomingCommandProjectFilterResolvesProjectName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Home"}],"next_cursor":""}`))
		case "/tasks":
			if got := r.URL.Query().Get("project_id"); got != "p1" {
				t.Fatalf("expected project_id=p1, got %q", got)
			}
			_, _ = w.Write([]byte(`{"results":[{"id":"t1","content":"Task","due":{"date":"2026-02-22"}}],"next_cursor":""}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeJSON,
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
		Now: func() time.Time {
			return time.Date(2026, 2, 22, 9, 0, 0, 0, time.UTC)
		},
	}
	if err := upcomingCommand(ctx, []string{"--project", "Home"}); err != nil {
		t.Fatalf("upcomingCommand: %v", err)
	}
}

func TestUpcomingCommandRejectsInvalidDays(t *testing.T) {
	ctx := &Context{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	if err := upcomingCommand(ctx, []string{"0"}); err == nil {
		t.Fatalf("expected error")
	}
	if err := upcomingCommand(ctx, []string{"abc"}); err == nil {
		t.Fatalf("expected parse error")
	}
}
