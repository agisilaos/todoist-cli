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

func TestSettingsViewJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"user":{"timezone":"Europe/London","time_format":0,"date_format":1,"start_day":1,"theme_id":6,"auto_reminder":30,"next_week":5,"start_page":"today"},
			"user_settings":{"reminder_push":true,"reminder_desktop":true,"reminder_email":false,"completed_sound_desktop":true,"completed_sound_mobile":true}
		}`))
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
	if err := settingsCommand(ctx, []string{"view"}); err != nil {
		t.Fatalf("settingsCommand: %v", err)
	}
	if !strings.Contains(out.String(), `"timezone": "Europe/London"`) {
		t.Fatalf("expected timezone in json output, got: %s", out.String())
	}
}

func TestSettingsUpdateDryRun(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute([]string{"settings", "update", "--timezone", "UTC", "--dry-run", "--json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d (stderr=%q)", exitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"action": "settings update"`) {
		t.Fatalf("expected dry run output, got %s", stdout.String())
	}
}

func TestSettingsUpdateCallsSync(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(r.Body)
		values, _ := url.ParseQuery(body.String())
		commands := values.Get("commands")
		if !strings.Contains(commands, `"type":"user_update"`) || !strings.Contains(commands, `"timezone":"UTC"`) {
			t.Fatalf("unexpected commands payload: %s", commands)
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
	if err := settingsCommand(ctx, []string{"update", "--timezone", "UTC"}); err != nil {
		t.Fatalf("settings update: %v", err)
	}
}
