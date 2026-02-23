package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestActivityListBuildsExpectedQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Home"}],"next_cursor":""}`))
		case "/activities":
			q := r.URL.Query()
			if q.Get("object_type") != "item" || q.Get("event_type") != "completed" || q.Get("parent_project_id") != "p1" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"results":[{"id":"a1","event_type":"completed","event_date":"2026-02-23T10:00:00Z","object_type":"item","object_id":"t1","parent_project_id":"p1","extra_data":{"content":"Task done"}}],"next_cursor":""}`))
		default:
			http.NotFound(w, r)
		}
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
	}
	if err := activityCommand(ctx, []string{"--type", "task", "--event", "completed", "--project", "Home"}); err != nil {
		t.Fatalf("activityCommand: %v", err)
	}
}

func TestActivityListByMeUsesSyncUserID(t *testing.T) {
	hitSync := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sync":
			hitSync = true
			_, _ = w.Write([]byte(`{"user":{"id":"u123"}}`))
		case "/activities":
			if r.URL.Query().Get("initiator_id") != "u123" {
				t.Fatalf("expected initiator_id=u123, got %q", r.URL.Query().Get("initiator_id"))
			}
			_, _ = w.Write([]byte(`{"results":[],"next_cursor":""}`))
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
	}
	if err := activityCommand(ctx, []string{"--by", "me"}); err != nil {
		t.Fatalf("activityCommand: %v", err)
	}
	if !hitSync {
		t.Fatalf("expected sync user call")
	}
}
