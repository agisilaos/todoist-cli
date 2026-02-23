package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestFetchRemindersSyncRequest(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
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
	client := NewClient("https://example.com", "token", time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		commands := values.Get("commands")
		if !strings.Contains(commands, `"type":"reminder_add"`) || !strings.Contains(commands, `"item_id":"t1"`) {
			t.Fatalf("unexpected commands payload: %s", commands)
		}
		payload := `{"temp_id_mapping":{"tmp1":"r9"}}`
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
