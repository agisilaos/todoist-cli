package cli

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestProjectBrowseDryRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Home"}],"next_cursor":""}`))
		case "/projects/p1":
			_, _ = w.Write([]byte(`{"id":"p1","name":"Home"}`))
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
		Global: GlobalOptions{DryRun: true},
	}
	if err := projectBrowse(ctx, []string{"Home"}); err != nil {
		t.Fatalf("projectBrowse: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"action": "project browse"`) || !strings.Contains(got, `"url": "https://app.todoist.com/app/project/home-p1"`) {
		t.Fatalf("unexpected output: %s", got)
	}
}

func TestProjectBrowseOpensBrowser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/p1":
			_, _ = w.Write([]byte(`{"id":"p1","name":"Home"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	var opened string
	prev := openProjectBrowserFn
	openProjectBrowserFn = func(u string) error {
		opened = u
		return nil
	}
	defer func() { openProjectBrowserFn = prev }()

	var out bytes.Buffer
	ctx := &Context{
		Stdout: &out,
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeJSON,
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	if err := projectBrowse(ctx, []string{"id:p1"}); err != nil {
		t.Fatalf("projectBrowse: %v", err)
	}
	if opened != "https://app.todoist.com/app/project/home-p1" {
		t.Fatalf("unexpected opened url: %q", opened)
	}
	if !strings.Contains(out.String(), `"opened": true`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestProjectBrowseOpenError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/p1":
			_, _ = w.Write([]byte(`{"id":"p1","name":"Home"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	prev := openProjectBrowserFn
	openProjectBrowserFn = func(_ string) error { return errors.New("boom") }
	defer func() { openProjectBrowserFn = prev }()

	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeHuman,
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	err := projectBrowse(ctx, []string{"id:p1"})
	if err == nil || !strings.Contains(err.Error(), "open browser: boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectBrowseRequiresRef(t *testing.T) {
	ctx := &Context{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman}
	err := projectBrowse(ctx, nil)
	if err == nil || !strings.Contains(err.Error(), "requires --id or a project reference") {
		t.Fatalf("unexpected error: %v", err)
	}
}
