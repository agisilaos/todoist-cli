package api

import (
	"bytes"
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

func TestFetchRemindersSyncRequest(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second, authorization.Resolve(nil, "credentials", true))
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/sync" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		if values.Get("resource_types") != `["reminders"]` {
			t.Fatalf("unexpected resource_types: %q", values.Get("resource_types"))
		}
		payload := `{"reminders":[{"id":"r1","item_id":"t1","type":"absolute","minute_offset":30,"is_deleted":false},{"id":"r2","item_id":"t2","type":"absolute","is_deleted":true}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(payload)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	reminders, reqID, err := client.FetchReminders(context.Background())
	if err != nil {
		t.Fatalf("FetchReminders: %v", err)
	}
	if reqID == "" {
		t.Fatalf("expected request id")
	}
	if len(reminders) != 1 || reminders[0].ID != "r1" {
		t.Fatalf("unexpected reminders: %#v", reminders)
	}
}

func TestAddReminderBuildsSyncCommand(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second, authorization.Resolve(nil, "credentials", true))
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		commands := values.Get("commands")
		if !strings.Contains(commands, `"type":"reminder_add"`) || !strings.Contains(commands, `"item_id":"t1"`) {
			t.Fatalf("unexpected commands payload: %s", commands)
		}
		var cmds []map[string]any
		json.Unmarshal([]byte(commands), &cmds)
		payload := fmt.Sprintf(`{"sync_status":{%q:"ok"},"temp_id_mapping":{"tmp1":"r9"}}`, cmds[0]["uuid"].(string))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(payload)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	id, _, err := client.AddReminder(context.Background(), ReminderAddInput{
		TempID:       "tmp1",
		ItemID:       "t1",
		MinuteOffset: 30,
	})
	if err != nil {
		t.Fatalf("AddReminder: %v", err)
	}
	if id != "r9" {
		t.Fatalf("unexpected reminder id: %q", id)
	}
}

func TestReminderCommandsRequireAcknowledgement(t *testing.T) {
	for _, kind := range []string{"add", "update", "delete"} {
		for _, outcome := range []string{"missing-ack", "rejected", "missing-id", "ok"} {
			t.Run(kind+"/"+outcome, func(t *testing.T) {
				client := NewClient("https://example.com", "synthetic", time.Second, authorization.Resolve(nil, "env", true))
				client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(r.Body)
					form, _ := url.ParseQuery(string(body))
					var commands []map[string]any
					json.Unmarshal([]byte(form.Get("commands")), &commands)
					response := map[string]any{"temp_id_mapping": map[string]string{"temporary": "real"}}
					if outcome != "missing-ack" {
						var status any = "ok"
						if outcome == "rejected" {
							status = map[string]any{"http_code": 403, "error": "denied"}
						}
						response["sync_status"] = map[string]any{commands[0]["uuid"].(string): status}
					}
					if outcome == "missing-id" {
						delete(response, "temp_id_mapping")
					}
					data, _ := json.Marshal(response)
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
				}))
				var id, requestID string
				var err error
				switch kind {
				case "add":
					id, requestID, err = client.AddReminder(context.Background(), ReminderAddInput{TempID: "temporary", ItemID: "task"})
				case "update":
					requestID, err = client.UpdateReminder(context.Background(), ReminderUpdateInput{ID: "real", MinuteOffset: 10})
				case "delete":
					requestID, err = client.DeleteReminder(context.Background(), "real")
				}
				wantErr := outcome == "missing-ack" || outcome == "rejected" || (outcome == "missing-id" && kind == "add")
				if (err != nil) != wantErr || requestID == "" || (kind == "add" && wantErr && id != "") {
					t.Fatalf("unconfirmed success: id=%q requestID=%q err=%v", id, requestID, err)
				}
				if outcome == "rejected" {
					var apiErr *APIError
					if !errors.As(err, &apiErr) || apiErr.Status != 403 {
						t.Fatalf("provider error lost: %v", err)
					}
				}
				if kind == "add" && !wantErr && id != "real" {
					t.Fatalf("ID=%q", id)
				}
			})
		}
	}
}

func TestRetriedReminderWithoutMappingNeverReturnsTemporaryID(t *testing.T) {
	calls := 0
	client := NewClient("https://example.com", "synthetic", time.Second, authorization.Resolve(nil, "env", true))
	client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		var commands []map[string]any
		json.Unmarshal([]byte(form.Get("commands")), &commands)
		status, response := 503, "lost original response"
		if calls == 2 {
			status = 200
			response = fmt.Sprintf(`{"sync_status":{%q:"ok"}}`, commands[0]["uuid"].(string))
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": {"0"}}, Body: io.NopCloser(strings.NewReader(response))}, nil
	}))
	id, _, err := client.AddReminder(context.Background(), ReminderAddInput{TempID: "temporary", ItemID: "task"})
	if err == nil || id != "" || calls != 2 || !strings.Contains(err.Error(), "confirm") {
		t.Fatalf("temporary ID escaped retry: id=%q err=%v calls=%d", id, err, calls)
	}
}
