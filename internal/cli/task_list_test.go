package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestListTasksByFilterFallbacksToSearchQueryForLiteralText(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/filter" {
			http.NotFound(w, r)
			return
		}
		hits++
		query := r.URL.Query().Get("query")
		switch query {
		case "Call mom":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"The search query is incorrect","error_tag":"INVALID_SEARCH_QUERY","http_code":400}`))
		case `search: "Call mom"`:
			_, _ = w.Write([]byte(`{"results":[{"id":"t1","content":"Call mom"}],"next_cursor":""}`))
		default:
			t.Fatalf("unexpected query %q", query)
		}
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	tasks, _, err := listTasksByFilter(ctx, "Call mom", "", 50, true)
	if err != nil {
		t.Fatalf("listTasksByFilter: %v", err)
	}
	if hits != 2 {
		t.Fatalf("expected 2 requests (original+fallback), got %d", hits)
	}
	if len(tasks) != 1 || tasks[0].ID != "t1" {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
}

func TestListTasksByFilterDoesNotFallbackStructuredQuery(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/filter" {
			http.NotFound(w, r)
			return
		}
		hits++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"The search query is incorrect","error_tag":"INVALID_SEARCH_QUERY","http_code":400}`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	if _, _, err := listTasksByFilter(ctx, "@work & today", "", 50, true); err == nil {
		t.Fatalf("expected error")
	}
	if hits != 1 {
		t.Fatalf("expected one request, got %d", hits)
	}
}

func TestNormalizeCompletedDateRangeDefaultsUntilAndParsesRelative(t *testing.T) {
	now := time.Date(2026, 2, 22, 18, 0, 0, 0, time.UTC)
	ctx := &Context{Now: func() time.Time { return now }}

	since, until, err := normalizeCompletedDateRange(ctx, "30 days ago", "")
	if err != nil {
		t.Fatalf("normalizeCompletedDateRange: %v", err)
	}
	if since != "2026-01-23" {
		t.Fatalf("unexpected since: %q", since)
	}
	if until != "2026-02-22" {
		t.Fatalf("unexpected until: %q", until)
	}
}

func TestNormalizeCompletedDateRangeRejectsInvalid(t *testing.T) {
	ctx := &Context{Now: func() time.Time { return time.Date(2026, 2, 22, 18, 0, 0, 0, time.UTC) }}
	if _, _, err := normalizeCompletedDateRange(ctx, "a while back", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWriteTaskListJSONWritesEmptyArrayForNilTasks(t *testing.T) {
	var out bytes.Buffer
	ctx := &Context{Stdout: &out, Mode: output.ModeJSON}
	if err := writeTaskList(ctx, nil, "", false); err != nil {
		t.Fatalf("writeTaskList: %v", err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("expected empty array output, got %q", out.String())
	}
	var decoded []any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if len(decoded) != 0 {
		t.Fatalf("expected zero items, got %d", len(decoded))
	}
}
