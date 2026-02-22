package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
)

func TestParseNaturalTaskReference(t *testing.T) {
	content, hint, ok := parseNaturalTaskReference("Call mom today")
	if !ok || content != "Call mom" || hint != "today" {
		t.Fatalf("unexpected parse result: content=%q hint=%q ok=%v", content, hint, ok)
	}
}

func TestFilterTasksByDueHint(t *testing.T) {
	now := time.Date(2026, time.February, 22, 10, 0, 0, 0, time.UTC)
	tasks := []api.Task{
		{ID: "t1", Content: "A", Due: &api.Due{Date: "2026-02-22"}},
		{ID: "t2", Content: "B", Due: &api.Due{Date: "2026-02-21"}},
		{ID: "t3", Content: "C", Due: &api.Due{Date: "2026-02-23"}},
	}
	got := filterTasksByDueHint(tasks, "overdue", now)
	if len(got) != 1 || got[0].ID != "t2" {
		t.Fatalf("unexpected overdue filtering: %#v", got)
	}
}

func TestResolveTaskRefNaturalToday(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"t1","content":"Call mom","due":{"date":"2026-02-22"}},{"id":"t2","content":"Call mom","due":{"date":"2026-02-23"}}],"next_cursor":""}`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
		Now: func() time.Time {
			return time.Date(2026, time.February, 22, 9, 0, 0, 0, time.UTC)
		},
	}
	task, err := resolveTaskRef(ctx, "Call mom today")
	if err != nil {
		t.Fatalf("resolveTaskRef: %v", err)
	}
	if task.ID != "t1" {
		t.Fatalf("expected t1, got %q", task.ID)
	}
}

func TestResolveTaskRefNaturalTodayAmbiguous(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"t1","content":"Call mom","due":{"date":"2026-02-22"}},{"id":"t2","content":"Call mom","due":{"date":"2026-02-22"}}],"next_cursor":""}`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
		Global: GlobalOptions{NoInput: true},
		Now: func() time.Time {
			return time.Date(2026, time.February, 22, 9, 0, 0, 0, time.UTC)
		},
	}
	_, err := resolveTaskRef(ctx, "Call mom today")
	if err == nil {
		t.Fatalf("expected ambiguity error")
	}
	var ambiguous *AmbiguousMatchError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected AmbiguousMatchError, got %T", err)
	}
}
