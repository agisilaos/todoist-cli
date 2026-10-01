package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestRescheduleExactRequestAndInputRefusals(t *testing.T) {
	f, ctx, _ := editingTestContext(t)
	if err := taskReschedule(ctx, []string{"--id", "t", "--due-date", "2026-10-03"}); err != nil {
		t.Fatal(err)
	}
	writes := f.writes()
	want := map[string]any{"type": "item_update", "args": map[string]any{"id": "t", "due": map[string]any{"date": "2026-10-03", "timezone": nil, "is_recurring": true, "string": "every day", "lang": "en"}}}
	if len(writes) != 1 || !reflect.DeepEqual(writes[0].body, want) {
		t.Fatal(writes, want)
	}
	for _, args := range [][]string{{"--due-date", "2026-10-02", "--due-datetime", "2026-10-02T12:00:00Z"}, {"--due-date", ""}, {"--due-datetime", "2026-10-02T12:00:00+24:00"}, {"--due-datetime", "2026-10-02T12:00:00,1Z"}, {"--due-local-datetime", "2026-10-02T1:00:00"}} {
		f, ctx, _ := editingTestContext(t)
		if err := taskReschedule(ctx, append([]string{"--id", "t"}, args...)); toExitCode(err) != exitUsage || len(f.requests) != 0 {
			t.Fatal(err, f.requests)
		}
	}
}
func TestTaskSortEveryKeyDirectionAndMissingLast(t *testing.T) {
	for _, key := range []string{"due", "deadline", "priority", "added", "updated", "completed", "content", "order"} {
		for _, direction := range []string{"asc", "desc"} {
			t.Run(key+"/"+direction, func(t *testing.T) {
				low, high := `{"id":"b","content":"Alpha","priority":1,"child_order":0,"due":{"date":"2026-10-01","timezone":null},"deadline":{"date":"2026-10-01"},"added_at":"2026-10-01T00:00:00Z","updated_at":"2026-10-01T00:00:00Z","completed_at":"2026-10-01T00:00:00Z"}`, `{"id":"c","content":"beta","priority":4,"child_order":9007199254740993,"due":{"date":"2026-10-02","timezone":null},"deadline":{"date":"2026-10-02"},"added_at":"2026-10-02T00:00:00Z","updated_at":"2026-10-02T00:00:00Z","completed_at":"2026-10-02T00:00:00Z"}`
				a := strings.Replace(low, `"id":"b"`, `"id":"a"`, 1)
				tasks := []api.Task{taskFromJSON(t, `{"id":"missing"}`), taskFromJSON(t, high), taskFromJSON(t, low), taskFromJSON(t, a)}
				if err := (taskSortOptions{key: key, direction: direction}).apply(tasks); err != nil {
					t.Fatal(err)
				}
				want := []string{"a", "b", "c", "missing"}
				if direction == "desc" {
					want = []string{"c", "a", "b", "missing"}
				}
				if got := taskIDs(tasks); !reflect.DeepEqual(got, want) {
					t.Fatal(got, want)
				}
			})
		}
	}
	tasks := []api.Task{taskFromJSON(t, `{"id":"offset","due":{"date":"2026-10-01T23:30:00-10:00"},"added_at":"2026-10-01T23:30:00-10:00"}`), taskFromJSON(t, `{"id":"utc","due":{"date":"2026-10-02T00:01:00Z"},"added_at":"2026-10-02T00:01:00Z"}`)}
	if err := (taskSortOptions{key: "due"}).apply(tasks); err != nil || tasks[0].ID != "offset" {
		t.Fatal(err, tasks)
	}
	if err := (taskSortOptions{key: "added", direction: "asc"}).apply(tasks); err != nil || tasks[0].ID != "utc" {
		t.Fatal(err, tasks)
	}
	for _, key := range []string{"", "none"} {
		before := taskIDs(tasks)
		_ = (taskSortOptions{key: key}).apply(tasks)
		if !reflect.DeepEqual(taskIDs(tasks), before) {
			t.Fatal(tasks)
		}
	}
}
func taskIDs(tasks []api.Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}
func TestTaskSortCollectionSurfacesAndPagination(t *testing.T) {
	for _, command := range []string{"active", "filter", "preset", "history", "completed", "today", "upcoming", "inbox", "filter show"} {
		t.Run(command, func(t *testing.T) {
			f, ctx, out := editingTestContext(t)
			ctx.Now = func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }
			f.tasks["t"]["content"] = "Zed"
			f.tasks["u"] = map[string]any{"id": "u", "content": "Alpha", "due": map[string]any{"date": "2026-10-01"}, "parent_id": nil}
			sorting := []string{"--sort", "content"}
			var err error
			switch command {
			case "active":
				err = taskList(ctx, append([]string{"--all-projects"}, sorting...))
			case "filter":
				err = taskList(ctx, append([]string{"--filter", "today"}, sorting...))
			case "preset":
				err = taskList(ctx, append([]string{"--preset", "today"}, sorting...))
			case "history":
				err = taskList(ctx, append([]string{"--completed"}, sorting...))
			case "completed":
				err = completedCommand(ctx, sorting)
			case "today":
				err = todayCommand(ctx, sorting)
			case "upcoming":
				err = upcomingCommand(ctx, sorting)
			case "inbox":
				err = inboxCommand(ctx, sorting)
			case "filter show":
				err = filterShow(ctx, append([]string{"Work"}, sorting...))
			}
			if err != nil {
				t.Fatal(err)
			}
			var rows []map[string]any
			if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 2 || rows[0]["id"] != "u" {
				t.Fatal(err, out)
			}
		})
	}
	for _, all := range []bool{false, true} {
		f, ctx, out := editingTestContext(t)
		pages := 0
		f.page = func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != "/tasks" {
				return false
			}
			pages++
			if r.URL.Query().Get("cursor") == "" {
				fmt.Fprint(w, `{"results":[{"id":"z","content":"Z"}],"next_cursor":"next"}`)
			} else {
				fmt.Fprint(w, `{"results":[{"id":"a","content":"A"}],"next_cursor":null}`)
			}
			return true
		}
		args := []string{"--all-projects", "--sort", "content"}
		if all {
			args = append(args, "--all")
		}
		if err := taskList(ctx, args); err != nil {
			t.Fatal(err)
		}
		var rows []map[string]any
		_ = json.Unmarshal(out.Bytes(), &rows)
		if all {
			if pages != 2 || len(rows) != 2 || rows[0]["id"] != "a" {
				t.Fatal(pages, out)
			}
		} else {
			if pages != 1 || len(rows) != 1 {
				t.Fatal(pages, out)
			}
		}
	}
}
func TestExpandedTaskViewAllPagesAndVersions(t *testing.T) {
	for _, version := range []int{1, 2} {
		for _, mode := range []output.Mode{output.ModeJSON, output.ModeNDJSON} {
			f, ctx, out := editingTestContext(t)
			ctx.Mode = mode
			ctx.Global.TaskOutputVersion = version
			childCalls := 0
			f.page = func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != "/tasks" {
					return false
				}
				childCalls++
				if r.URL.Query().Get("parent_id") != "t" {
					t.Error(r.URL)
				}
				if childCalls == 1 {
					fmt.Fprint(w, `{"results":[{"id":"c","parent_id":"t","child_order":2}],"next_cursor":"second"}`)
				} else {
					fmt.Fprint(w, `{"results":[{"id":"b","parent_id":"t","child_order":0}],"next_cursor":null}`)
				}
				return true
			}
			if err := taskView(ctx, []string{"--id", "t", "--include-children", "--sort", "order"}); err != nil {
				t.Fatal(err)
			}
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			var children []map[string]any
			_ = json.Unmarshal(envelope["children"], &children)
			if childCalls != 2 || string(envelope["children_complete"]) != "true" || len(children) != 2 || children[0]["id"] != "b" {
				t.Fatal(out)
			}
			if version == 2 {
				if children[0]["child_order"] != float64(0) {
					t.Fatal(children)
				}
			}
			if mode == output.ModeNDJSON && strings.Count(out.String(), "\n") != 1 {
				t.Fatal(out)
			}
		}
	}
	f, ctx, _ := editingTestContext(t)
	if err := taskView(ctx, []string{"--id", "t"}); err != nil || len(f.requests) != 1 {
		t.Fatal(err, f.requests)
	}
}
func TestExpandedTaskViewFailuresHaveEmptyStdout(t *testing.T) {
	for _, kind := range []string{"missing collection", "null collection", "missing cursor", "null id", "duplicate", "wrong parent", "root child", "completed", "deleted", "self", "cycle", "later failure"} {
		t.Run(kind, func(t *testing.T) {
			f, ctx, out := editingTestContext(t)
			calls := 0
			f.page = func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != "/tasks" {
					return false
				}
				calls++
				switch kind {
				case "missing collection":
					fmt.Fprint(w, `{"next_cursor":null}`)
				case "null collection":
					fmt.Fprint(w, `{"results":null,"next_cursor":null}`)
				case "missing cursor":
					fmt.Fprint(w, `{"results":[]}`)
				case "null id":
					fmt.Fprint(w, `{"results":[{"id":null}],"next_cursor":null}`)
				case "duplicate":
					fmt.Fprint(w, `{"results":[{"id":"c"},{"id":"c"}],"next_cursor":null}`)
				case "wrong parent":
					fmt.Fprint(w, `{"results":[{"id":"c","parent_id":"other"}],"next_cursor":null}`)
				case "root child":
					fmt.Fprint(w, `{"results":[{"id":"c","parent_id":null}],"next_cursor":null}`)
				case "completed":
					fmt.Fprint(w, `{"results":[{"id":"c","checked":true}],"next_cursor":null}`)
				case "deleted":
					fmt.Fprint(w, `{"results":[{"id":"c","is_deleted":true}],"next_cursor":null}`)
				case "self":
					fmt.Fprint(w, `{"results":[{"id":"t","parent_id":"t"}],"next_cursor":null}`)
				case "cycle":
					fmt.Fprint(w, `{"results":[],"next_cursor":"same"}`)
				case "later failure":
					if calls == 1 {
						fmt.Fprint(w, `{"results":[{"id":"c"}],"next_cursor":"second"}`)
					} else {
						http.Error(w, "denied", 403)
					}
				}
				return true
			}
			if err := taskView(ctx, []string{"--id", "t", "--include-children"}); err == nil || out.Len() != 0 {
				t.Fatal(err, out)
			}
		})
	}
	for _, args := range [][]string{{"--sort", "content"}, {"--sort", ""}, {"--include-children", "--sort-order", "asc"}, {"--include-children", "--sort", "none", "--sort-order", "desc"}, {"--include-children", "--sort-order", ""}} {
		f, ctx, _ := editingTestContext(t)
		if err := taskView(ctx, append([]string{"--id", "t"}, args...)); toExitCode(err) != exitUsage || len(f.requests) != 0 {
			t.Fatal(err, f.requests)
		}
	}
}
