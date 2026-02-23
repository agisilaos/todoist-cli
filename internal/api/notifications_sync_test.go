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

func TestFetchLiveNotifications(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/sync" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		if values.Get("resource_types") != `["live_notifications"]` {
			t.Fatalf("unexpected resource_types: %q", values.Get("resource_types"))
		}
		payload := `{"live_notifications":[{"id":"n1","notification_type":"item_assigned","is_unread":true,"is_deleted":false,"created_at":"2026-02-23T10:00:00Z","item_id":"t1","item_content":"Write docs"}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(payload)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	items, reqID, err := client.FetchLiveNotifications(context.Background())
	if err != nil {
		t.Fatalf("FetchLiveNotifications: %v", err)
	}
	if reqID == "" {
		t.Fatalf("expected request id")
	}
	if len(items) != 1 || items[0].ID != "n1" || items[0].TaskID != "t1" {
		t.Fatalf("unexpected notifications: %#v", items)
	}
}

func TestMarkNotificationsReadCommand(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		commands := values.Get("commands")
		if !strings.Contains(commands, `"type":"live_notifications_mark_read"`) {
			t.Fatalf("unexpected commands: %s", commands)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	if _, err := client.MarkNotificationsRead(context.Background(), []string{"n1"}); err != nil {
		t.Fatalf("MarkNotificationsRead: %v", err)
	}
}

func TestAcceptInvitationCommand(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		commands := values.Get("commands")
		if !strings.Contains(commands, `"type":"accept_invitation"`) {
			t.Fatalf("unexpected commands: %s", commands)
		}
		if !strings.Contains(commands, `"invitation_id":123`) || !strings.Contains(commands, `"invitation_secret":"abc"`) {
			t.Fatalf("missing invitation args: %s", commands)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	if _, err := client.AcceptInvitation(context.Background(), "123", "abc"); err != nil {
		t.Fatalf("AcceptInvitation: %v", err)
	}
}

func TestRejectInvitationCommand(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		commands := values.Get("commands")
		if !strings.Contains(commands, `"type":"reject_invitation"`) {
			t.Fatalf("unexpected commands: %s", commands)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	if _, err := client.RejectInvitation(context.Background(), "123", "abc"); err != nil {
		t.Fatalf("RejectInvitation: %v", err)
	}
}
