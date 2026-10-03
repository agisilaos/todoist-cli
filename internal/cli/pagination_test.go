package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/authorization"
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
		Client: api.NewClient(ts.URL, "token", time.Second, authorization.Resolve(nil, "credentials", true)),
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

func TestPaginationCursorCycles(t *testing.T) {
	for _, strict := range []bool{false, true} {
		for _, tc := range []struct {
			name, initial string
			cursors       []string
			calls         int
		}{
			{"repeat", "", []string{"a", "a"}, 2}, {"long cycle", "", []string{"a", "b", "a"}, 3}, {"initial", "seed", []string{"seed"}, 1},
		} {
			t.Run(fmt.Sprint(strict)+tc.name, func(t *testing.T) {
				calls := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if calls > len(tc.cursors) {
						fmt.Fprint(w, `{"results":[],"next_cursor":null}`)
						return
					}
					fmt.Fprintf(w, `{"results":[{"id":"%d"}],"next_cursor":%q}`, calls, tc.cursors[calls-1])
				}))
				defer srv.Close()
				ctx := &Context{Client: api.NewClient(srv.URL, "synthetic", time.Second, authorization.Resolve(nil, "credentials", true)), Config: config.Config{TimeoutSeconds: 2}}
				items, next, err := fetchPaginated[api.Project](ctx, "/projects", url.Values{"cursor": {tc.initial}}, true, strict)
				if err == nil || !strings.Contains(err.Error(), "cursor cycle") || items != nil || next != "" || calls != tc.calls {
					t.Fatalf("items=%v next=%q err=%v calls=%d", items, next, err, calls)
				}
			})
		}
	}
}

func TestPaginationTerminalAndSinglePage(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		all, strict   bool
		wantNext      string
	}{
		{"null", `{"results":[],"next_cursor":null}`, true, false, ""},
		{"empty", `{"results":[],"next_cursor":""}`, true, false, ""},
		{"strict null", `{"results":[],"next_cursor":null}`, true, true, ""},
		{"single continuation", `{"results":[],"next_cursor":"seed"}`, false, false, "seed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, tc.payload) }))
			defer srv.Close()
			ctx := &Context{Client: api.NewClient(srv.URL, "synthetic", time.Second, authorization.Resolve(nil, "credentials", true)), Config: config.Config{TimeoutSeconds: 2}}
			items, next, err := fetchPaginated[api.Project](ctx, "/projects", url.Values{"cursor": {"seed"}}, tc.all, tc.strict)
			if err != nil || len(items) != 0 || next != tc.wantNext || calls != 1 {
				t.Fatalf("items=%v next=%q err=%v calls=%d", items, next, err, calls)
			}
		})
	}
}
