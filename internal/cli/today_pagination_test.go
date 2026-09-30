package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func todayPaginationContext(server *httptest.Server, mode output.Mode) *Context {
	return &Context{
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Mode: mode,
		Token:  "synthetic-today-token",
		Client: api.NewClient(server.URL, "synthetic-today-token", time.Second, authorization.Resolve(nil, "env", true)),
		Config: config.Config{BaseURL: server.URL, TimeoutSeconds: 2, TableWidth: 120},
		Now:    func() time.Time { return time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC) },
	}
}

func TestTodayPaginationPreservesEveryResult(t *testing.T) {
	for _, mode := range []output.Mode{output.ModeHuman, output.ModeJSON, output.ModeIDsOnly} {
		for _, count := range []int{0, 12, 201} {
			t.Run(fmt.Sprintf("%s/%d", mode, count), func(t *testing.T) {
				tasks := make([]api.Task, count)
				for i := range tasks {
					// Reverse IDs make preserving Todoist's returned order observable.
					tasks[i] = api.Task{ID: fmt.Sprintf("task-%03d", count-i), Content: fmt.Sprintf("Task %d", i), ProjectID: "work", Priority: 4, Due: &api.Due{Date: "2026-09-30"}}
				}
				var mu sync.Mutex
				var cursors []string
				projects := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/projects" {
						mu.Lock()
						projects++
						mu.Unlock()
						fmt.Fprint(w, `{"results":[{"id":"work","name":"Work"}],"next_cursor":null}`)
						return
					}
					if r.Method != http.MethodGet || r.URL.Path != "/tasks/filter" || r.URL.Query().Get("query") != "overdue | today" || r.URL.Query().Get("limit") != "200" {
						t.Errorf("unexpected Today request: %s %s", r.Method, r.URL)
						http.Error(w, "unexpected request", http.StatusBadRequest)
						return
					}
					cursor := r.URL.Query().Get("cursor")
					mu.Lock()
					cursors = append(cursors, cursor)
					mu.Unlock()
					start := 0
					if cursor != "" {
						if cursor != "after-200" {
							t.Errorf("unexpected cursor: %q", cursor)
							http.Error(w, "unexpected cursor", http.StatusBadRequest)
							return
						}
						start = 200
					}
					end := min(start+200, len(tasks))
					next := ""
					if end < len(tasks) {
						next = "after-200"
					}
					if err := json.NewEncoder(w).Encode(api.Paginated[api.Task]{Results: tasks[start:end], NextCursor: next}); err != nil {
						t.Error(err)
					}
				}))
				defer server.Close()
				ctx := todayPaginationContext(server, mode)
				if err := todayCommand(ctx, nil); err != nil {
					t.Fatal(err)
				}
				mu.Lock()
				defer mu.Unlock()
				wantCursors := []string{""}
				if count > 200 {
					wantCursors = append(wantCursors, "after-200")
				}
				if !reflect.DeepEqual(cursors, wantCursors) {
					t.Fatalf("pages/cursors = %q, want %q", cursors, wantCursors)
				}
				wantProjects := 0
				if mode == output.ModeHuman && count > 0 {
					wantProjects = 1
				}
				if projects != wantProjects || ctx.Stderr.(*bytes.Buffer).Len() != 0 {
					t.Fatalf("enrichment/coverage changed: projects=%d stderr=%q", projects, ctx.Stderr)
				}
				got := ctx.Stdout.(*bytes.Buffer).String()
				switch mode {
				case output.ModeJSON:
					var returned []api.Task
					if err := json.Unmarshal([]byte(got), &returned); err != nil {
						t.Fatal(err)
					}
					if len(returned) != count || (count == 0 && strings.TrimSpace(got) != "[]") {
						t.Fatalf("incomplete JSON collection: %s", got)
					}
					for i, task := range returned {
						if task.ID != tasks[i].ID || task.Content != tasks[i].Content || task.ProjectID != tasks[i].ProjectID || task.Due == nil || task.Due.Date != tasks[i].Due.Date {
							t.Fatalf("result %d changed: %#v", i, task)
						}
					}
				case output.ModeIDsOnly:
					var want strings.Builder
					for _, task := range tasks {
						fmt.Fprintln(&want, task.ID)
					}
					if got != want.String() {
						t.Fatalf("ID result collection changed: %q", got)
					}
				case output.ModeHuman:
					if !strings.Contains(got, fmt.Sprintf("%d shown · All pages fetched", count)) {
						t.Fatalf("incorrect coverage: %s", got)
					}
					if count == 0 && !strings.Contains(got, "No overdue or due-today tasks returned.") {
						t.Fatalf("missing empty Today message: %s", got)
					}
					last := -1
					for _, task := range tasks {
						pos := strings.Index(got, "ID "+task.ID)
						if pos <= last || strings.Count(got, "ID "+task.ID) != 1 {
							t.Fatalf("missing, duplicated or reordered task %s", task.ID)
						}
						last = pos
					}
				}
			})
		}
	}
}

func TestTodayLatePageFailureDoesNotEmitPartialResults(t *testing.T) {
	for _, mode := range []output.Mode{output.ModeHuman, output.ModeJSON, output.ModeIDsOnly} {
		t.Run(string(mode), func(t *testing.T) {
			var mu sync.Mutex
			var cursors []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/tasks/filter" || r.URL.Query().Get("limit") != "200" || r.URL.Query().Get("query") != "overdue | today" {
					t.Errorf("unexpected request after page failure: %s", r.URL)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				cursor := r.URL.Query().Get("cursor")
				mu.Lock()
				cursors = append(cursors, cursor)
				mu.Unlock()
				if cursor == "" {
					// A short page must still follow its cursor.
					fmt.Fprint(w, `{"results":[{"id":"first","content":"First task","project_id":"work"}],"next_cursor":"late-page"}`)
					return
				}
				http.Error(w, "late page unavailable", http.StatusForbidden)
			}))
			defer server.Close()
			ctx := todayPaginationContext(server, mode)
			if err := todayCommand(ctx, nil); err == nil {
				t.Fatal("late-page failure reported success")
			}
			mu.Lock()
			defer mu.Unlock()
			if !reflect.DeepEqual(cursors, []string{"", "late-page"}) || ctx.Stdout.(*bytes.Buffer).Len() != 0 || ctx.Stderr.(*bytes.Buffer).Len() != 0 {
				t.Fatalf("partial results/coverage emitted: cursors=%q stdout=%q stderr=%q", cursors, ctx.Stdout, ctx.Stderr)
			}
		})
	}
}

func TestTodayPageSizeDoesNotChangeExplicitTaskListLimit(t *testing.T) {
	var mu sync.Mutex
	var cursors []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/filter" || r.URL.Query().Get("limit") != "7" || r.URL.Query().Get("query") != "overdue | today" {
			t.Errorf("explicit task-list limit changed: %s", r.URL)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		cursor := r.URL.Query().Get("cursor")
		mu.Lock()
		cursors = append(cursors, cursor)
		mu.Unlock()
		index, next := 1, "last-page"
		if cursor != "" {
			index, next = 2, ""
		}
		if err := json.NewEncoder(w).Encode(api.Paginated[api.Task]{Results: []api.Task{{ID: strconv.Itoa(index)}}, NextCursor: next}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	ctx := todayPaginationContext(server, output.ModeIDsOnly)
	if err := taskList(ctx, []string{"--filter", "overdue | today", "--limit", "7", "--all"}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(cursors, []string{"", "last-page"}) || ctx.Stdout.(*bytes.Buffer).String() != "1\n2\n" || ctx.Stderr.(*bytes.Buffer).Len() != 0 {
		t.Fatalf("explicit task-list results changed: cursors=%q stdout=%q stderr=%q", cursors, ctx.Stdout, ctx.Stderr)
	}
}
