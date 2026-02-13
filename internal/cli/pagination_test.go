package cli

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
)

func TestCloneQueryIsIndependent(t *testing.T) {
	in := url.Values{"cursor": {"a"}, "limit": {"50"}}
	out := cloneQuery(in)
	out.Set("cursor", "b")
	if in.Get("cursor") != "a" {
		t.Fatalf("expected input cursor unchanged, got %q", in.Get("cursor"))
	}
}

func TestFetchPaginatedCollectsPages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = w.Write([]byte(`{"results":[{"id":"1","name":"A"}],"next_cursor":"c1"}`))
		case "c1":
			_, _ = w.Write([]byte(`{"results":[{"id":"2","name":"B"}],"next_cursor":""}`))
		default:
			http.Error(w, "bad cursor", http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	ctx := &Context{
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 2},
	}
	query := url.Values{}
	query.Set("limit", "50")
	items, next, err := fetchPaginated[api.Project](ctx, "/projects", query, true)
	if err != nil {
		t.Fatalf("fetchPaginated: %v", err)
	}
	if len(items) != 2 || items[0].ID != "1" || items[1].ID != "2" {
		t.Fatalf("unexpected items: %#v", items)
	}
	if next != "" {
		t.Fatalf("expected empty final cursor, got %q", next)
	}
}
