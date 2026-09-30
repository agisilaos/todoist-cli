package cli

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func TestTaskDetailReusesOnlyCompleteSectionCollection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		global      []string
		failLast    bool
		unrelated   bool
		wantScoped  int
		wantSection string
	}{
		{"complete pages", []string{`{"results":[{"id":"section-B","project_id":"other","name":"Wrong project"}],"next_cursor":"last"}`, `{"results":[{"id":"section-B","project_id":"project-A","name":"Launch"}],"next_cursor":""}`}, false, false, 0, "Launch"},
		{"complete empty", []string{`{"results":[],"next_cursor":""}`}, false, false, 0, "section-B (name unavailable)"},
		{"missing section", []string{`{"results":[{"id":"other-section","project_id":"project-A","name":"Other"}],"next_cursor":""}`}, false, false, 0, "section-B (name unavailable)"},
		{"foreign section", []string{`{"results":[{"id":"section-B","project_id":"other","name":"Wrong project"}],"next_cursor":""}`}, false, false, 0, "section-B (name unavailable)"},
		{"failed final page", []string{`{"results":[{"id":"section-B","project_id":"project-A","name":"Partial collection"}],"next_cursor":"last"}`, ""}, true, false, 1, "Launch"},
		{"no collection", nil, false, false, 1, "Launch"},
		{"unrelated scoped cache", nil, false, true, 1, "Launch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			globalHits, scopedHits := 0, 0
			ctx, out := captureTestContext(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if r.Method != http.MethodGet {
					t.Errorf("unexpected mutation: %s", r.Method)
				}
				switch r.URL.Path {
				case "/projects":
					io.WriteString(w, `{"results":[{"id":"project-A","name":"Work"}]}`)
				case "/sections":
					if project := r.URL.Query().Get("project_id"); project != "" {
						scopedHits++
						if project != "project-A" {
							t.Errorf("wrong scope: %s", project)
						}
						io.WriteString(w, `{"results":[{"id":"section-B","project_id":"project-A","name":"Launch"}]}`)
						return
					}
					globalHits++
					if tc.failLast && r.URL.Query().Get("cursor") == "last" {
						http.Error(w, "unavailable", http.StatusForbidden)
						return
					}
					if globalHits > len(tc.global) {
						t.Errorf("unexpected global request: %s", r.URL)
						http.Error(w, "unexpected read", http.StatusBadRequest)
						return
					}
					io.WriteString(w, tc.global[globalHits-1])
				default:
					t.Errorf("unexpected request: %s", r.URL)
					http.NotFound(w, r)
				}
			})
			if tc.unrelated {
				ctx.cache().sectionsByProject["other"] = []api.Section{{ID: "section-B", ProjectID: "other", Name: "Wrong project"}}
			}
			if tc.global != nil {
				_, err := listAllSections(ctx, "")
				if (err != nil) != tc.failLast {
					t.Fatalf("global read: %v", err)
				}
				if _, loaded := ctx.cache().sectionsByProject[""]; loaded == tc.failLast {
					t.Fatalf("incorrect global completeness after read: loaded=%v", loaded)
				}
			}
			if err := writeTaskView(ctx, detailTask(t, detailTaskJSON), false); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if scopedHits != tc.wantScoped || globalHits != len(tc.global) {
				t.Errorf("requests: global=%d, scoped=%d; want %d/%d", globalHits, scopedHits, len(tc.global), tc.wantScoped)
			}
			if !strings.Contains(out.String(), fmt.Sprintf("Section: %s\n", tc.wantSection)) || strings.Contains(out.String(), "Wrong project") {
				t.Errorf("incorrect destination: %s", out)
			}
		})
	}
}
