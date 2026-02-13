package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
)

func TestListAllProjectsUsesCache(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects" {
			http.NotFound(w, r)
			return
		}
		hits++
		_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Home"}],"next_cursor":""}`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}

	a, err := listAllProjects(ctx)
	if err != nil {
		t.Fatalf("first listAllProjects: %v", err)
	}
	b, err := listAllProjects(ctx)
	if err != nil {
		t.Fatalf("second listAllProjects: %v", err)
	}
	if len(a) != 1 || len(b) != 1 || a[0].ID != "p1" || b[0].ID != "p1" {
		t.Fatalf("unexpected projects: %#v %#v", a, b)
	}
	if hits != 1 {
		t.Fatalf("expected one API hit, got %d", hits)
	}
}

func TestListAllFiltersUsesCache(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filters" {
			http.NotFound(w, r)
			return
		}
		hits++
		_, _ = w.Write([]byte(`[{"id":"f1","name":"Today","query":"today"}]`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}

	a, _, err := listAllFilters(ctx)
	if err != nil {
		t.Fatalf("first listAllFilters: %v", err)
	}
	b, _, err := listAllFilters(ctx)
	if err != nil {
		t.Fatalf("second listAllFilters: %v", err)
	}
	if len(a) != 1 || len(b) != 1 || a[0].ID != "f1" || b[0].ID != "f1" {
		t.Fatalf("unexpected filters: %#v %#v", a, b)
	}
	if hits != 1 {
		t.Fatalf("expected one API hit, got %d", hits)
	}
}
