package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/authorization"
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
		Client: api.NewClient(ts.URL, "token", time.Second, authorization.Resolve(nil, "credentials", true)),
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
		Client: api.NewClient(ts.URL, "token", time.Second, authorization.Resolve(nil, "credentials", true)),
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
		Client: api.NewClient(ts.URL, "token", time.Second, authorization.Resolve(nil, "credentials", true)),
		Config: config.Config{TimeoutSeconds: 2},
	}
	project, err := resolveViewTarget("https://app.todoist.com/app/project/home-2203306141", ctx)
	if err != nil || project.Command != "project" || project.Args[0] != "view" {
		t.Fatalf("unexpected project target: %#v err=%v", project, err)
	}
}

func TestViewLabelURLUsesCompletePaginatedCollection(t *testing.T) {
	for _, failLastPage := range []bool{false, true} {
		labelRequests, taskRequests := 0, 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/labels":
				labelRequests++
				if r.URL.Query().Get("cursor") == "" {
					w.Write([]byte(`{"results":[],"next_cursor":"last"}`))
				} else if failLastPage {
					http.Error(w, "label lookup denied", http.StatusForbidden)
				} else {
					w.Write([]byte(`{"results":[{"id":"labelid","name":"work"}]}`))
				}
			case "/tasks":
				taskRequests++
				if r.URL.Query().Get("label") != "work" {
					t.Errorf("wrong label selection: %s", r.URL)
				}
				w.Write([]byte(`{"results":[{"id":"task-id","duration":null}]}`))
			default:
				t.Errorf("unexpected request %s", r.URL)
			}
		}))
		var out bytes.Buffer
		ctx := &Context{Token: "token", Stdout: &out, Stderr: &bytes.Buffer{}, Mode: output.ModeJSON, Global: GlobalOptions{TaskOutputVersion: 2}, Client: api.NewClient(server.URL, "token", time.Second, authorization.Resolve(nil, "credentials", true)), Config: config.Config{TimeoutSeconds: 2}}
		err := viewCommand(ctx, []string{"https://app.todoist.com/app/label/work-labelid"})
		if failLastPage {
			if err == nil || taskRequests != 0 || out.Len() != 0 {
				t.Fatalf("failed label lookup selected tasks: %v requests=%d stdout=%s", err, taskRequests, out.String())
			}
		} else if err != nil || taskRequests != 1 || !strings.Contains(out.String(), `"duration": null`) {
			t.Fatalf("label page two did not select tasks: %v requests=%d stdout=%s", err, taskRequests, out.String())
		}
		if labelRequests != 2 {
			t.Errorf("expected exactly two label requests with cache reuse, got %d", labelRequests)
		}
		server.Close()
	}
}
