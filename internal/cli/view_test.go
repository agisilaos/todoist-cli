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

func TestResolveViewTargetEntityURLs(t *testing.T) {
	ctx := &Context{}

	task, err := resolveViewTarget("https://app.todoist.com/app/task/call-mom-abc123", ctx)
	if err != nil || task.Command != "task" || len(task.Args) < 2 || task.Args[0] != "view" {
		t.Fatalf("unexpected task target: %#v err=%v", task, err)
	}
	filter, err := resolveViewTarget("https://app.todoist.com/app/filter/today-f1", ctx)
	if err != nil || filter.Command != "filter" || filter.Args[0] != "show" {
		t.Fatalf("unexpected filter target: %#v err=%v", filter, err)
	}
}

func TestResolveViewTargetPageURLs(t *testing.T) {
	ctx := &Context{}
	target, err := resolveViewTarget("https://app.todoist.com/app/settings", ctx)
	if err != nil {
		t.Fatalf("resolveViewTarget: %v", err)
	}
	if target.Command != "settings" || target.Args[0] != "view" {
		t.Fatalf("unexpected settings target: %#v", target)
	}
}

func TestViewCommandTaskURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tasks/t1":
			_, _ = w.Write([]byte(`{"id":"t1","content":"Call mom","project_id":"p1","priority":1}`))
		case "/projects":
			_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Home"}],"next_cursor":""}`))
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
	}
	if err := viewCommand(ctx, []string{"https://app.todoist.com/app/task/call-mom-t1"}); err != nil {
		t.Fatalf("viewCommand: %v", err)
	}
	if !strings.Contains(out.String(), `"id": "t1"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestResolveProjectRefFromURLFallsBackToSlugName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Home"},{"id":"p2","name":"Work"}],"next_cursor":""}`))
		default:
			http.NotFound(w, r)
		}
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
	ref, err := resolveProjectRefFromURL(ctx, "https://app.todoist.com/app/project/home-2203306141", "2203306141")
	if err != nil {
		t.Fatalf("resolveProjectRefFromURL: %v", err)
	}
	if ref != "Home" {
		t.Fatalf("expected slug fallback to project name, got %q", ref)
	}
}

func TestResolveViewTargetProjectURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			_, _ = w.Write([]byte(`{"results":[{"id":"2203306141","name":"Home"}],"next_cursor":""}`))
		default:
			http.NotFound(w, r)
		}
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
	project, err := resolveViewTarget("https://app.todoist.com/app/project/home-2203306141", ctx)
	if err != nil || project.Command != "task" || project.Args[0] != "list" {
		t.Fatalf("unexpected project target: %#v err=%v", project, err)
	}
}
