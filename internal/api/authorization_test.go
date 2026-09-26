package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/authorization"
)

func TestReadOnlyAuthorizationCoversRESTAndEverySyncMutationFamily(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write([]byte(`{}`)) }))
	defer server.Close()
	report := authorization.Resolve(json.RawMessage(`{"version":1,"mode":"read-only","origin":"oauth-device","requested_scopes":["data:read"],"effective_scopes":["data:read"],"scope_evidence":"oauth-request"}`), "credentials", true)
	client := api.NewClient(server.URL, "secret", time.Second, report)
	ctx := context.Background()
	blocked := func(name string, call func() error) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			var denied *authorization.Error
			if err := call(); !errors.As(err, &denied) || denied.Code != "READ_ONLY" {
				t.Fatalf("expected permission denial, got %v", err)
			}
		})
	}
	for _, path := range []string{"/tasks", "/tasks/id", "/tasks/id/move", "/tasks/id/close", "/tasks/id/reopen", "/projects", "/projects/id", "/projects/id/move", "/projects/id/archive", "/projects/id/unarchive", "/sections", "/sections/id", "/labels", "/labels/id", "/comments", "/comments/id", "/filters", "/filters/id", "/future"} {
		blocked("post"+path, func() error { _, err := client.Post(ctx, path, nil, map[string]any{}, nil, true); return err })
		blocked("delete"+path, func() error { _, err := client.Delete(ctx, path, nil); return err })
	}
	blocked("quick-add", func() error { _, _, err := client.QuickAdd(ctx, "proposed"); return err })
	blocked("filter-add", func() error {
		_, _, err := client.AddFilter(ctx, map[string]any{"name": "Focus", "query": "today"})
		return err
	})
	blocked("filter-update", func() error { _, _, err := client.UpdateFilter(ctx, "f1", map[string]any{"name": "Focus"}); return err })
	blocked("filter-delete", func() error { _, err := client.DeleteFilter(ctx, "f1"); return err })
	blocked("reminder-add", func() error { _, _, err := client.AddReminder(ctx, api.ReminderAddInput{ItemID: "task"}); return err })
	blocked("reminder-update", func() error { _, err := client.UpdateReminder(ctx, api.ReminderUpdateInput{ID: "r"}); return err })
	blocked("reminder-delete", func() error { _, err := client.DeleteReminder(ctx, "r"); return err })
	blocked("notification-read", func() error { _, err := client.MarkNotificationsRead(ctx, []string{"n"}); return err })
	blocked("notification-unread", func() error { _, err := client.MarkNotificationsUnread(ctx, []string{"n"}); return err })
	blocked("notification-read-all", func() error { _, err := client.MarkAllNotificationsRead(ctx); return err })
	blocked("accept", func() error { _, err := client.AcceptInvitation(ctx, "123", "invitation"); return err })
	blocked("reject", func() error { _, err := client.RejectInvitation(ctx, "123", "invitation"); return err })
	goal := 5
	vacation := true
	theme := 2
	blocked("goals", func() error { _, err := client.UpdateGoals(ctx, api.UpdateGoalsInput{DailyGoal: &goal}); return err })
	blocked("vacation", func() error {
		_, err := client.UpdateGoals(ctx, api.UpdateGoalsInput{VacationMode: &vacation})
		return err
	})
	blocked("settings", func() error {
		_, err := client.UpdateUserSettings(ctx, api.UpdateUserSettingsInput{Theme: &theme})
		return err
	})
	blocked("unclassified-read", func() error { _, err := client.Get(ctx, "/future", nil, nil); return err })
	blocked("sync-commands", func() error {
		_, err := client.Post(ctx, "/sync", nil, map[string]any{"resource_types": []string{"user"}, "commands": []any{map[string]any{"type": "user_update"}}}, nil, false)
		return err
	})
	if requests.Load() != 0 {
		t.Fatalf("unauthorized HTTP requests: %d", requests.Load())
	}
}

func TestReadOnlyAuthorizationAllowsRESTAndSyncReads(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write([]byte(`{"results":[],"user":{"id":"u"},"reminders":[],"live_notifications":[],"workspaces":[]}`))
	}))
	defer server.Close()
	report := authorization.Resolve(json.RawMessage(`{"version":1,"mode":"read-only","origin":"oauth-pkce","requested_scopes":["data:read"],"effective_scopes":["data:read"],"scope_evidence":"token-response"}`), "credentials", true)
	client := api.NewClient(server.URL, "secret", time.Second, report)
	ctx := context.Background()
	calls := []func() error{
		func() error { _, err := client.Get(ctx, "/projects", nil, nil); return err },
		func() error { _, err := client.Get(ctx, "/tasks/taskID", nil, nil); return err },
		func() error { _, _, err := client.SyncWorkspaces(ctx); return err },
		func() error { _, _, err := client.SyncCurrentUserID(ctx); return err },
		func() error { _, _, err := client.FetchReminders(ctx); return err },
		func() error { _, _, err := client.FetchFilters(ctx); return err },
		func() error { _, _, err := client.FetchUserSettings(ctx); return err },
		func() error { _, _, err := client.FetchLiveNotifications(ctx); return err },
		func() error { _, _, err := client.FetchProductivityStats(ctx); return err },
	}
	for i, call := range calls {
		if err := call(); err != nil {
			t.Errorf("read %d: %v", i, err)
		}
	}
	if requests.Load() != int32(len(calls)) {
		t.Fatalf("reads dispatched %d want %d", requests.Load(), len(calls))
	}
}

func TestReadOnlyAuthorizationChecksRedirects(t *testing.T) {
	for _, tc := range []struct {
		name, from, to string
		post, blocked  bool
	}{
		{"sync-to-mutation", "/sync", "/tasks/id/close", true, true},
		{"read-to-unclassified", "/projects", "/future", false, true},
		{"read-to-read", "/projects", "/tasks", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var redirected atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1"+tc.from {
					http.Redirect(w, r, "/api/v1"+tc.to, http.StatusTemporaryRedirect)
					return
				}
				redirected.Add(1)
				w.Write([]byte(`{}`))
			}))
			defer server.Close()
			report := authorization.Resolve(json.RawMessage(`{"version":1,"mode":"read-only","origin":"oauth-pkce","requested_scopes":["data:read"],"effective_scopes":["data:read"],"scope_evidence":"oauth-request"}`), "credentials", true)
			client := api.NewClient(server.URL+"/api/v1", "secret", time.Second, report)
			var err error
			if tc.post {
				_, err = client.Post(context.Background(), tc.from, nil, map[string]any{"resource_types": []string{"user"}}, nil, false)
			} else {
				_, err = client.Get(context.Background(), tc.from, nil, nil)
			}
			if tc.blocked {
				var denied *authorization.Error
				if !errors.As(err, &denied) || denied.Code != "READ_ONLY" || redirected.Load() != 0 || err.Error() != "Todoist mutation blocked: the active credential is read-only." {
					t.Fatalf("redirect bypassed authorization: requests=%d error=%v", redirected.Load(), err)
				}
			} else if err != nil || redirected.Load() != 1 {
				t.Fatalf("read redirect: requests=%d error=%v", redirected.Load(), err)
			}
		})
	}
}
