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

func TestNotificationListHumanEmptyState(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"live_notifications":[]}`))
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
	}
	if err := notificationList(ctx, nil); err != nil {
		t.Fatalf("notificationList: %v", err)
	}
	if !strings.Contains(out.String(), "No notifications.") {
		t.Fatalf("expected empty state, got %q", out.String())
	}
}

func TestNotificationViewJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"live_notifications":[{"id":"n1","notification_type":"item_assigned","is_unread":true,"is_deleted":false,"created_at":"2026-02-23T10:00:00Z"}]}`))
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
	if err := notificationView(ctx, []string{"--id", "n1"}); err != nil {
		t.Fatalf("notificationView: %v", err)
	}
	if !strings.Contains(out.String(), `"id": "n1"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestNotificationAcceptDryRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"live_notifications":[{"id":"n1","notification_type":"share_invitation_sent","invitation_id":"123","invitation_secret":"sec","is_unread":true,"is_deleted":false,"created_at":"2026-02-23T10:00:00Z"}]}`))
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
		Global: GlobalOptions{DryRun: true},
	}
	if err := notificationAccept(ctx, []string{"--id", "n1"}); err != nil {
		t.Fatalf("notificationAccept: %v", err)
	}
	if !strings.Contains(out.String(), `"action": "notification accept"`) {
		t.Fatalf("unexpected dry-run output: %s", out.String())
	}
}

func TestNotificationAcceptMarksRead(t *testing.T) {
	call := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" {
			http.NotFound(w, r)
			return
		}
		call++
		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(r.Body)
		values, _ := url.ParseQuery(body.String())
		commands := values.Get("commands")
		switch call {
		case 1:
			_, _ = w.Write([]byte(`{"live_notifications":[{"id":"n1","notification_type":"share_invitation_sent","invitation_id":"123","invitation_secret":"sec","is_unread":true,"is_deleted":false,"created_at":"2026-02-23T10:00:00Z"}]}`))
			return
		case 2:
			if !strings.Contains(commands, `"type":"accept_invitation"`) {
				t.Fatalf("expected accept_invitation command, got %s", commands)
			}
		case 3:
			if !strings.Contains(commands, `"type":"live_notifications_mark_read"`) {
				t.Fatalf("expected live_notifications_mark_read command, got %s", commands)
			}
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
	}
	if err := notificationAccept(ctx, []string{"--id", "n1"}); err != nil {
		t.Fatalf("notificationAccept: %v", err)
	}
	if call != 3 {
		t.Fatalf("expected 3 sync calls, got %d", call)
	}
}

func TestNotificationViewHumanShowsActionHints(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"live_notifications":[{"id":"n1","notification_type":"share_invitation_sent","project_name":"Project A","from_user":{"name":"Alice"},"is_unread":true,"is_deleted":false,"created_at":"2026-02-23T10:00:00Z"}]}`))
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
	}
	if err := notificationView(ctx, []string{"--id", "n1"}); err != nil {
		t.Fatalf("notificationView: %v", err)
	}
	if !strings.Contains(out.String(), "Actions:") || !strings.Contains(out.String(), "notification accept") {
		t.Fatalf("expected action hints, got %q", out.String())
	}
}
