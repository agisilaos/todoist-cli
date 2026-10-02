package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/authorization"
)

func TestSyncReadsCompleteLargeResponses(t *testing.T) {
	padding := strings.Repeat("x", 300*1024)
	for _, resource := range []string{"workspaces", "user", "reminders"} {
		t.Run(resource, func(t *testing.T) {
			client := NewClient("https://example.com", "synthetic", time.Second, authorization.Resolve(nil, "env", true))
			client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"padding":%q,"workspaces":[{"id":"w1","name":"Team"}],"user":{"id":"u1"},"reminders":[{"id":"r1"}]}`, padding)))}, nil
			}))
			var err error
			switch resource {
			case "workspaces":
				var workspaces []Workspace
				workspaces, _, err = client.SyncWorkspaces(context.Background())
				if err == nil && (len(workspaces) != 1 || workspaces[0].ID != "w1") {
					t.Fatal(workspaces)
				}
			case "user":
				var id string
				id, _, err = client.SyncCurrentUserID(context.Background())
				if err == nil && id != "u1" {
					t.Fatal(id)
				}
			case "reminders":
				var reminders []Reminder
				reminders, _, err = client.FetchReminders(context.Background())
				if err == nil && (len(reminders) != 1 || reminders[0].ID != "r1") {
					t.Fatal(reminders)
				}
			}
			if err != nil {
				t.Fatalf("valid large response truncated: %v", err)
			}
		})
	}
}

type syncFailingBody struct {
	closed bool
	err    error
	prefix string
}

func (b *syncFailingBody) Read(p []byte) (int, error) {
	n := copy(p, b.prefix)
	b.prefix = ""
	return n, b.err
}
func (b *syncFailingBody) Close() error { b.closed = true; return nil }

func TestSyncPropagatesBodyReadFailure(t *testing.T) {
	for _, resource := range []string{"workspaces", "user", "reminders"} {
		t.Run(resource, func(t *testing.T) {
			failure := errors.New("synthetic read failure")
			body := &syncFailingBody{err: failure, prefix: `{"workspaces":[],"user":{"id":"u1"},"reminders":[]}`}
			client := NewClient("https://example.com", "synthetic", time.Second, authorization.Resolve(nil, "env", true))
			client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: body}, nil
			}))
			var requestID string
			var err error
			switch resource {
			case "workspaces":
				_, requestID, err = client.SyncWorkspaces(context.Background())
			case "user":
				_, requestID, err = client.SyncCurrentUserID(context.Background())
			case "reminders":
				_, requestID, err = client.FetchReminders(context.Background())
			}
			if !errors.Is(err, failure) || requestID == "" || !body.closed {
				t.Fatalf("read error lost: err=%v requestID=%q closed=%v", err, requestID, body.closed)
			}
		})
	}
}

func TestSyncRetriesPreserveRequestIdentity(t *testing.T) {
	for _, command := range []bool{false, true} {
		t.Run(fmt.Sprint(command), func(t *testing.T) {
			calls := 0
			firstID, firstBody := "", ""
			client := NewClient("https://example.com", "synthetic", time.Second, authorization.Resolve(nil, "env", true))
			client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				data, _ := io.ReadAll(r.Body)
				if calls == 1 {
					firstID, firstBody = r.Header.Get("X-Request-Id"), string(data)
				}
				if firstID == "" || firstID != r.Header.Get("X-Request-Id") || firstBody != string(data) || r.Header.Get("Authorization") != "Bearer synthetic" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
					t.Fatal("retry changed request identity or encoding")
				}
				status, response := http.StatusOK, `{"workspaces":[]}`
				if calls == 1 {
					status, response = http.StatusServiceUnavailable, "temporary"
				}
				if command && calls > 1 {
					form, _ := url.ParseQuery(string(data))
					var cmds []map[string]any
					json.Unmarshal([]byte(form.Get("commands")), &cmds)
					response = fmt.Sprintf(`{"sync_status":{%q:"ok"}}`, cmds[0]["uuid"].(string))
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": {"0"}}, Body: io.NopCloser(strings.NewReader(response))}, nil
			}))
			var err error
			if command {
				_, err = client.UpdateReminder(context.Background(), ReminderUpdateInput{ID: "synthetic", MinuteOffset: 10})
			} else {
				_, _, err = client.SyncWorkspaces(context.Background())
			}
			if err != nil || calls != 2 {
				t.Fatalf("safe Sync retry failed: err=%v calls=%d", err, calls)
			}
			form, _ := url.ParseQuery(firstBody)
			if command && !strings.Contains(form.Get("commands"), `"uuid":`) {
				t.Fatal("mutation retry has no command identity")
			}
		})
	}
}

func TestSyncUnsafeAndNonRetryableFailuresStop(t *testing.T) {
	for _, tc := range []struct {
		name   string
		form   map[string]string
		status int
		body   string
	}{
		{"missing-uuid", map[string]string{"commands": `[{"type":"reminder_delete","args":{"id":"r1"}}]`}, 503, "temporary"},
		{"mixed-uuid", map[string]string{"commands": `[{"type":"reminder_delete","uuid":"stable","args":{"id":"r1"}},{"type":"reminder_delete","args":{"id":"r2"}}]`}, 503, "temporary"},
		{"invalid-commands", map[string]string{"commands": `invalid`}, 503, "temporary"},
		{"provider-rejection", map[string]string{"sync_token": "*", "resource_types": `["workspaces"]`}, 403, "denied"},
		{"provider-payload-error", map[string]string{"sync_token": "*", "resource_types": `["workspaces"]`}, 200, `{"error":"denied","error_tag":"TEST"}`},
		{"malformed-response", map[string]string{"sync_token": "*", "resource_types": `["workspaces"]`}, 200, `invalid`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := NewClient("https://example.com", "synthetic", time.Second, authorization.Resolve(nil, "env", true))
			client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Retry-After": {"0"}}, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			}))
			_, id, err := client.syncRequest(context.Background(), tc.form)
			if err == nil || calls != 1 || id == "" {
				t.Fatalf("failure retried or lost: err=%v calls=%d id=%q", err, calls, id)
			}
		})
	}
}

func TestSyncRetryLimitAndCancellation(t *testing.T) {
	for _, cancelRetry := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelRetry), func(t *testing.T) {
			operation, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			client := NewClient("https://example.com", "synthetic", time.Second, authorization.Resolve(nil, "env", true))
			client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if cancelRetry {
					cancel()
				}
				return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"0"}}, Body: io.NopCloser(strings.NewReader("temporary"))}, nil
			}))
			_, id, err := client.SyncWorkspaces(operation)
			wantCalls := maxRetries + 1
			if cancelRetry {
				wantCalls = 1
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation lost: %v", err)
				}
			}
			if err == nil || id == "" || calls != wantCalls {
				t.Fatalf("retry limit violated: err=%v calls=%d id=%q", err, calls, id)
			}
		})
	}
}

func TestSyncRetriesTransportFailure(t *testing.T) {
	calls := 0
	client := NewClient("https://example.com", "synthetic", time.Second, authorization.Resolve(nil, "env", true))
	client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("synthetic temporary transport failure")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"workspaces":[{"id":"w1","name":"Team"}]}`))}, nil
	}))
	workspaces, _, err := client.SyncWorkspaces(context.Background())
	if err != nil || calls != 2 || len(workspaces) != 1 || workspaces[0].ID != "w1" {
		t.Fatalf("transport retry failed: err=%v calls=%d workspaces=%v", err, calls, workspaces)
	}
}
