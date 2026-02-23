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

func TestNotificationListFiltersUnread(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"live_notifications":[
{"id":"n1","notification_type":"item_assigned","is_unread":true,"is_deleted":false,"created_at":"2026-02-23T10:00:00Z"},
{"id":"n2","notification_type":"item_completed","is_unread":false,"is_deleted":false,"created_at":"2026-02-22T10:00:00Z"}
]}`))
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
	if err := notificationList(ctx, []string{"--unread"}); err != nil {
		t.Fatalf("notificationList: %v", err)
	}
	if !strings.Contains(out.String(), `"id": "n1"`) || strings.Contains(out.String(), `"id": "n2"`) {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestNotificationReadDryRun(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"notification", "read", "--id", "n1", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"action": "notification read"`) {
		t.Fatalf("unexpected dry-run output: %q", stdout.String())
	}
}

func TestNotificationUnreadDryRun(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "dummy")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"notification", "unread", "--id", "n1", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"action": "notification unread"`) {
		t.Fatalf("unexpected dry-run output: %q", stdout.String())
	}
}
