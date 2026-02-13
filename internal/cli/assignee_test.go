package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
)

func TestResolveAssigneeIDMe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sync" {
			_, _ = w.Write([]byte(`{"user":{"id":"u-me"}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	ctx := &Context{
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	id, err := resolveAssigneeID(ctx, "me", "", "")
	if err != nil {
		t.Fatalf("resolveAssigneeID: %v", err)
	}
	if id != "u-me" {
		t.Fatalf("unexpected id: %q", id)
	}
}

func TestResolveAssigneeIDByEmailRequiresProject(t *testing.T) {
	ctx := &Context{
		Client: api.NewClient("https://example.com", "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	_, err := resolveAssigneeID(ctx, "ada@example.com", "", "")
	if err == nil {
		t.Fatalf("expected project-required error")
	}
}

func TestResolveAssigneeIDByEmailWithProject(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/p1/collaborators":
			_, _ = w.Write([]byte(`{"results":[{"id":"u1","name":"Ada Lovelace","email":"ada@example.com"}],"next_cursor":""}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	ctx := &Context{
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	id, err := resolveAssigneeID(ctx, "ada@example.com", "id:p1", "")
	if err != nil {
		t.Fatalf("resolveAssigneeID: %v", err)
	}
	if id != "u1" {
		t.Fatalf("unexpected id: %q", id)
	}
}
