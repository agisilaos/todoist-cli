package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestReminderListForTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tasks/t1":
			_, _ = w.Write([]byte(`{"id":"t1","content":"Call mom"}`))
		case "/sync":
			_, _ = w.Write([]byte(`{"reminders":[{"id":"r1","item_id":"t1","type":"absolute","minute_offset":30,"is_deleted":false}]}`))
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
	if err := reminderList(ctx, []string{"id:t1"}); err != nil {
		t.Fatalf("reminderList: %v", err)
	}
	if !strings.Contains(out.String(), `"id": "r1"`) {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestReminderUpdateDryRun(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	t.Setenv("TODOIST_TOKEN", "dummy")
	code := Execute([]string{"reminder", "update", "--id", "r1", "--before", "30m", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"action": "reminder update"`) {
		t.Fatalf("unexpected dry-run output: %q", stdout.String())
	}
}
