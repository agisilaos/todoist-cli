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

func TestProjectMoveWorkspaceDryRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
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
	if err := projectMove(ctx, []string{"id:p1", "--to-workspace", "id:w1", "--visibility", "team"}); err != nil {
		t.Fatalf("projectMove: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"action": "project move"`) || !strings.Contains(got, `"workspace_id": "w1"`) || !strings.Contains(got, `"visibility": "team"`) {
		t.Fatalf("unexpected output: %s", got)
	}
}

func TestProjectMoveWorkspacePreviewWithoutYes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
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
		Mode:   output.ModeHuman,
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	if err := projectMove(ctx, []string{"id:p1", "--to-workspace", "id:w1"}); err != nil {
		t.Fatalf("projectMove: %v", err)
	}
	if !strings.Contains(out.String(), `Would move "Home" to workspace "id:w1"`) || !strings.Contains(out.String(), "Use --yes to confirm.") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestProjectMoveWorkspaceWithYes(t *testing.T) {
	var moved bool
	var gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/p1":
			_, _ = w.Write([]byte(`{"id":"p1","name":"Home"}`))
		case "/projects/move_to_workspace":
			moved = true
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(r.Body)
			gotBody = buf.String()
			_, _ = w.Write([]byte(`{"project":{"id":"p1","name":"Home","workspace_id":"w1"}}`))
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
	if err := projectMove(ctx, []string{"id:p1", "--to-workspace", "id:w1", "--visibility", "restricted", "--yes"}); err != nil {
		t.Fatalf("projectMove: %v", err)
	}
	if !moved {
		t.Fatalf("expected move endpoint to be called")
	}
	if !strings.Contains(gotBody, `"workspace_id":"w1"`) || !strings.Contains(gotBody, `"visibility":"restricted"`) {
		t.Fatalf("unexpected body: %s", gotBody)
	}
}

func TestProjectMoveToPersonal(t *testing.T) {
	var moved bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/p1":
			_, _ = w.Write([]byte(`{"id":"p1","name":"Team","workspace_id":"w1"}`))
		case "/projects/move_to_personal":
			moved = true
			_, _ = w.Write([]byte(`{"project":{"id":"p1","name":"Team"}}`))
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
	if err := projectMove(ctx, []string{"id:p1", "--to-personal", "--yes"}); err != nil {
		t.Fatalf("projectMove: %v", err)
	}
	if !moved {
		t.Fatalf("expected move_to_personal call")
	}
}

func TestProjectMoveToPersonalAlreadyPersonal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/p1":
			_, _ = w.Write([]byte(`{"id":"p1","name":"Home"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeHuman,
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	err := projectMove(ctx, []string{"id:p1", "--to-personal", "--yes"})
	if err == nil || !strings.Contains(err.Error(), "already personal") {
		t.Fatalf("unexpected error: %v", err)
	}
}
