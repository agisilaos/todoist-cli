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

func TestFetchUserSettings(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/sync" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		if values.Get("resource_types") != `["user","user_settings"]` {
			t.Fatalf("unexpected resource types: %q", values.Get("resource_types"))
		}
		payload := `{
		  "user": {
		    "timezone": "Europe/London",
		    "time_format": 0,
		    "date_format": 1,
		    "start_day": 1,
		    "theme_id": 6,
		    "auto_reminder": 60,
		    "next_week": 1,
		    "start_page": "today"
		  },
		  "user_settings": {
		    "reminder_push": true,
		    "reminder_desktop": true,
		    "reminder_email": false,
		    "completed_sound_desktop": true,
		    "completed_sound_mobile": true
		  }
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(payload)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	out, _, err := client.FetchUserSettings(context.Background())
	if err != nil {
		t.Fatalf("FetchUserSettings: %v", err)
	}
	if out.Timezone != "Europe/London" || out.Theme != 6 || out.AutoReminder != 60 {
		t.Fatalf("unexpected settings payload: %#v", out)
	}
}

func TestUpdateUserSettingsBuildsCommands(t *testing.T) {
	client := NewClient("https://example.com", "token", time.Second)
	client.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		commands := values.Get("commands")
		if !strings.Contains(commands, `"type":"user_update"`) || !strings.Contains(commands, `"type":"user_settings_update"`) {
			t.Fatalf("unexpected commands: %s", commands)
		}
		if !strings.Contains(commands, `"timezone":"UTC"`) || !strings.Contains(commands, `"reminder_email":true`) {
			t.Fatalf("missing expected settings fields: %s", commands)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	timezone := "UTC"
	emailOn := true
	if _, err := client.UpdateUserSettings(context.Background(), UpdateUserSettingsInput{
		Timezone:      &timezone,
		ReminderEmail: &emailOn,
	}); err != nil {
		t.Fatalf("UpdateUserSettings: %v", err)
	}
}
