package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func overviewFixture() []api.Task {
	recurring := true
	return []api.Task{
		{ID: "101000001", Content: "Send revised estimate", ProjectID: "201", Priority: 4, Due: &api.Due{Date: "2026-09-28"}},
		{ID: "102000002", Content: "Prepare launch checklist and confirm the rollback owner with the platform team", ProjectID: "201", Priority: 3, Due: &api.Due{Date: "2026-09-29", IsRecurring: &recurring}},
		{ID: "103000003", Content: "Book dentist appointment", ProjectID: "202", Priority: 1, DueReturned: true},
		{ID: "104000004", Content: "Pick up prescription", ProjectID: "203", Priority: 2, Due: &api.Due{Date: "2026-09-30"}},
	}
}

func overviewContext() *Context {
	return &Context{
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman,
		Config: config.Config{TableWidth: 80},
		Now:    func() time.Time { return time.Date(2026, 9, 29, 23, 30, 0, 0, time.FixedZone("UTC-2", -7200)) },
		lookupCache: &lookupCache{projectsLoaded: true, projects: []api.Project{
			{ID: "201", Name: "Work"}, {ID: "202", Name: "Inbox"}, {ID: "203", Name: "Personal"},
		}},
	}
}

func TestTaskOverviewWidthsAndOrder(t *testing.T) {
	for _, width := range []int{40, 80, 120} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			ctx := overviewContext()
			ctx.Config.TableWidth = width
			tasks := overviewFixture()
			view := taskOverview{Scope: "All projects · Active tasks", ReferenceDate: "2026-09-29"}
			if err := writeTaskOverview(ctx, tasks, "", false, view); err != nil {
				t.Fatal(err)
			}
			got := ctx.Stdout.(*bytes.Buffer).String()
			for _, line := range strings.Split(got, "\n") {
				if utf8.RuneCountInString(line) > width {
					t.Fatalf("line exceeds %d columns: %q", width, line)
				}
			}
			last := -1
			for _, task := range tasks {
				pos := strings.Index(got, task.ID)
				if pos <= last {
					t.Fatalf("missing/reordered full ID %s: %s", task.ID, got)
				}
				last = pos
			}
			flat := strings.ReplaceAll(strings.Join(strings.Fields(got), " "), " · ", " ")
			for _, want := range []string{"4 shown · All pages fetched", "1 overdue · 1 due today", "P1 Send revised estimate", "Repeats · Work", "No due date · Inbox", "Due 2026-09-30 · Personal"} {
				if !strings.Contains(flat, strings.ReplaceAll(want, " · ", " ")) {
					t.Errorf("missing %q: %s", want, got)
				}
			}
		})
	}
}

func TestTaskOverviewLongTitlesAndSafeText(t *testing.T) {
	ctx := overviewContext()
	ctx.Config.TableWidth = 40
	tasks := []api.Task{{ID: "full-id-123", Content: strings.Repeat("長い題名", 30) + "\x1b[2J", ProjectID: "999", Priority: 1, DueReturned: true}}
	if err := writeTaskOverview(ctx, tasks, "", false, taskOverview{Scope: "Filter\x1b[2J"}); err != nil {
		t.Fatal(err)
	}
	got := ctx.Stdout.(*bytes.Buffer).String()
	start := strings.Index(got, "P4  ")
	lines := strings.Split(got[start:], "\n")
	if len(lines) < 4 || !strings.HasSuffix(lines[2], "…") || !strings.Contains(lines[3], "No due date") {
		t.Fatalf("title did not stop at three lines: %s", got)
	}
	// Eighteen two-cell CJK characters fill the first title line's 36 cells.
	if utf8.RuneCountInString(strings.TrimPrefix(lines[0], "P4  ")) != 18 {
		t.Fatalf("incorrect CJK wrapping: %q", lines[0])
	}
	if strings.Contains(got, "\x1b") || !strings.Contains(got, `Filter\x1b[2J`) || !strings.Contains(got, "full-id-123") || !strings.Contains(got, "999 (name unavailable)") {
		t.Fatalf("unsafe or incomplete output: %q", got)
	}
	if got := wrapOverviewText("cafe\u0301 cafe\u0301", 4, true); len(got) != 2 || got[0] != "cafe\u0301" {
		t.Fatalf("combining character split: %#v", got)
	}
}

func TestTaskOverviewDueFacts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		task     api.Task
		want     string
		category string
	}{
		{"missing", api.Task{}, "Due unavailable", ""},
		{"null", api.Task{DueReturned: true}, "No due date", ""},
		{"overdue", api.Task{Due: &api.Due{Date: "2026-09-28"}}, "Overdue · 2026-09-28", "overdue"},
		{"earlier today", api.Task{Due: &api.Due{Datetime: "2026-09-29T01:00:00Z"}}, "Today · 2026-09-29T01:00:00Z", "today"},
		{"offset crosses UTC day", api.Task{Due: &api.Due{Datetime: "2026-09-30T01:00:00+02:00"}}, "Today · 2026-09-30T01:00:00+02:00", "today"},
		{"date precedence", api.Task{Due: &api.Due{Date: "2026-09-30", Datetime: "2026-09-29T23:00:00Z"}}, "Due 2026-09-30 · 2026-09-29T23:00:00Z", "future"},
		{"floating date time", api.Task{Due: &api.Due{Date: "2026-09-29T09:30:00"}}, "Today · 2026-09-29T09:30:00", "today"},
		{"malformed", api.Task{Due: &api.Due{Date: "2026-09-28garbage"}}, "Due 2026-09-28garbage (date unavailable)", ""},
		{"expression only", api.Task{Due: &api.Due{String: "every weekday"}}, "Due every weekday (date unavailable)", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, category := overviewDue(tc.task, "2026-09-29")
			if got != tc.want || category != tc.category {
				t.Fatalf("got %q %q, want %q %q", got, category, tc.want, tc.category)
			}
		})
	}
	// The date driving the label must remain visible across the UTC boundary,
	// even when Todoist returns the instant on a different calendar date.
	task := api.Task{Due: &api.Due{Date: "2026-09-30", Datetime: "2026-09-29T23:00:00Z"}}
	if got, category := overviewDue(task, "2026-09-30"); got != "Today · 2026-09-30 · 2026-09-29T23:00:00Z" || category != "today" {
		t.Fatalf("hidden classification date: %q %q", got, category)
	}
	ctx := overviewContext()
	if err := writeTaskOverview(ctx, nil, "", false, taskOverview{Scope: "Today", Empty: "Empty"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ctx.Stdout.(*bytes.Buffer).String(), "2026-09-30 (UTC)") {
		t.Fatal("reference date must use UTC, not the clock's local date")
	}
}

func TestTaskOverviewKeepsMetadataTogether(t *testing.T) {
	ctx := overviewContext()
	ctx.Config.TableWidth = 40
	if err := writeTaskOverview(ctx, overviewFixture(), "", false, taskOverview{Scope: "All projects · Active tasks"}); err != nil {
		t.Fatal(err)
	}
	got := ctx.Stdout.(*bytes.Buffer).String()
	for _, line := range strings.Split(got, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "·") || strings.HasSuffix(line, "·") || strings.HasSuffix(line, " ID") {
			t.Fatalf("orphaned metadata: %q", line)
		}
	}
	for _, task := range overviewFixture() {
		if !strings.Contains(got, "ID "+task.ID) {
			t.Fatalf("split reference: %q", got)
		}
	}
}

func TestTaskOverviewPageCoverageAndQuiet(t *testing.T) {
	for _, tc := range []struct {
		name, cursor     string
		continued, quiet bool
		want             string
	}{
		{"empty selection", "", false, false, "No active tasks in Inbox."},
		{"empty partial page", "next", false, false, "No tasks on this page; more available."},
		{"end continuation", "", true, false, "Continuation · End of results"},
		{"partial continuation", "next", true, false, "Continuation · More available"},
		{"quiet", "next", false, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := overviewContext()
			ctx.Global.Quiet = tc.quiet
			if err := writeTaskOverview(ctx, nil, tc.cursor, false, taskOverview{Scope: "Inbox", Empty: "No active tasks in Inbox.", Continued: tc.continued}); err != nil {
				t.Fatal(err)
			}
			got := ctx.Stdout.(*bytes.Buffer).String()
			if !strings.Contains(got, tc.want) || (tc.quiet && got != "") || ((tc.continued || tc.cursor != "") && strings.Contains(got, "All pages fetched")) {
				t.Fatalf("incorrect coverage: %q", got)
			}
			if notice := ctx.Stderr.(*bytes.Buffer).String(); (notice != "") != (tc.cursor != "") {
				t.Fatalf("continuation notice: %q", notice)
			}
		})
	}
}

func TestTaskOverviewMachineContractsUnchanged(t *testing.T) {
	for _, mode := range []output.Mode{output.ModePlain, output.ModeJSON, output.ModeNDJSON, output.ModeIDsOnly} {
		for _, accessible := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", mode, accessible), func(t *testing.T) {
				old, current := overviewContext(), overviewContext()
				for _, ctx := range []*Context{old, current} {
					ctx.Mode, ctx.Accessible = mode, accessible
					ctx.lookupCache = nil
				}
				if err := writeTaskList(old, overviewFixture(), "next", false); err != nil {
					t.Fatal(err)
				}
				if err := writeTaskOverview(current, overviewFixture(), "next", false, taskOverview{Scope: "Ignored"}); err != nil {
					t.Fatal(err)
				}
				if old.Stdout.(*bytes.Buffer).String() != current.Stdout.(*bytes.Buffer).String() || old.Stderr.(*bytes.Buffer).String() != current.Stderr.(*bytes.Buffer).String() {
					t.Fatal("machine stdout/stderr changed")
				}
			})
		}
	}
}

func TestTaskOverviewCommandsScopeAndFetching(t *testing.T) {
	for _, tc := range []struct {
		name               string
		run                func(*Context) error
		path, query, scope string
		pages              int
	}{
		{"today", func(c *Context) error { return todayCommand(c, nil) }, "/tasks/filter", "overdue | today", "Today · Across projects", 2},
		{"filter", func(c *Context) error {
			return taskList(c, []string{"--filter", "today", "--sort", "priority"})
		}, "/tasks/filter", "today", `Filter: "today"`, 1},
		{"preset", func(c *Context) error { return taskList(c, []string{"--preset", "today"}) }, "/tasks/filter", "today", `Filter: "today"`, 1},
		{"inbox", func(c *Context) error { return inboxCommand(c, nil) }, "/tasks", "", "Inbox · Active tasks", 2},
		{"all projects", func(c *Context) error { return taskList(c, []string{"--all-projects"}) }, "/tasks", "", "All projects · Active tasks", 1},
		{"upcoming", func(c *Context) error { return upcomingCommand(c, nil) }, "/tasks", "", "Upcoming · 2026-09-29 to 2026-10-05 (UTC)", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pages, projects := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/projects":
					projects++
					fmt.Fprint(w, `{"results":[{"id":"201","name":"Work"},{"id":"202","name":"Inbox","inbox_project":true}]}`)
				case tc.path:
					pages++
					if r.URL.Query().Get("query") != tc.query {
						t.Errorf("wrong query: %s", r.URL)
					}
					if tc.name == "inbox" && r.URL.Query().Get("project_id") != "202" {
						t.Errorf("wrong Inbox: %s", r.URL)
					}
					next := ""
					if r.URL.Query().Get("cursor") == "" {
						next = "second"
					}
					if err := json.NewEncoder(w).Encode(api.Paginated[api.Task]{Results: overviewFixture()[pages-1 : pages], NextCursor: next}); err != nil {
						t.Error(err)
					}
				default:
					t.Errorf("unexpected enrichment request %s", r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			ctx := overviewContext()
			ctx.Token, ctx.Config.BaseURL, ctx.Config.TimeoutSeconds = "synthetic", server.URL, 2
			ctx.lookupCache = nil
			ctx.Now = func() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) }
			if err := tc.run(ctx); err != nil {
				t.Fatal(err)
			}
			got := ctx.Stdout.(*bytes.Buffer).String()
			if !strings.Contains(got, tc.scope) || strings.Contains(got, "Ignored") || pages != tc.pages || projects != 1 {
				t.Fatalf("scope/fetch changed: tasks=%d projects=%d output=%s", pages, projects, got)
			}
			if tc.pages == 1 && !strings.Contains(got, "More available") {
				t.Fatal("partial page not marked")
			}
		})
	}
}

func TestTaskOverviewWideAndQuietKeepDetails(t *testing.T) {
	ctx := overviewContext()
	ctx.Global.Quiet = true
	ctx.lookupCache.sectionsByProject = map[string][]api.Section{"": {{ID: "301", Name: "Launch"}}}
	task := overviewFixture()[0]
	task.SectionID, task.Labels = "301", []string{"focus"}
	if err := writeTaskOverview(ctx, []api.Task{task}, "next", true, taskOverview{Scope: "Hidden"}); err != nil {
		t.Fatal(err)
	}
	got := ctx.Stdout.(*bytes.Buffer).String()
	for _, want := range []string{"Content", "Section", "Completed", "Launch", "focus", "101000001"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "Hidden") || strings.Contains(got, "Due labels:") || strings.Contains(got, "P1") {
		t.Fatalf("changed quiet/table convention: %s", got)
	}
}
