package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

type taskActionFixture struct {
	mu       sync.Mutex
	tasks    map[string]map[string]any
	requests []string
	mode     string
	response *string
	url      string
}

func newTaskActionFixture(t *testing.T) (*taskActionFixture, *Context, *bytes.Buffer) {
	t.Helper()
	f := &taskActionFixture{tasks: map[string]map[string]any{}}
	for _, id := range []string{"task-123", "task-456"} {
		f.tasks[id] = map[string]any{"id": id, "content": "Prepare launch checklist", "project_id": "source", "section_id": "old-section", "parent_id": nil, "checked": false, "due": nil}
	}
	f.tasks["new-parent"] = map[string]any{"id": "new-parent", "content": "Parent", "parent_id": nil, "project_id": "destination", "section_id": nil}
	ctx, out := captureTestContext(t, f.serve)
	f.url = ctx.Config.BaseURL
	return f, ctx, out
}

func (f *taskActionFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	write := func(value any) { _ = json.NewEncoder(w).Encode(value) }
	if r.Method == "GET" {
		switch r.URL.Path {
		case "/projects":
			write(map[string]any{"results": []any{map[string]any{"id": "destination", "name": "Home"}, map[string]any{"id": "adjusted", "name": "Archive"}}})
		case "/sections":
			if r.URL.Query().Get("project_id") != "destination" {
				http.Error(w, "wrong section scope", 400)
				return
			}
			write(map[string]any{"results": []any{map[string]any{"id": "new-section", "name": "Backlog", "project_id": "destination"}}})
		case "/tasks", "/tasks/filter":
			rows := []any{f.tasks["task-123"]}
			if f.mode == "bulk" {
				rows = append(rows, f.tasks["task-456"])
			}
			write(map[string]any{"results": rows, "next_cursor": nil})
		default:
			if task := f.tasks[strings.TrimPrefix(r.URL.Path, "/tasks/")]; task != nil {
				write(task)
			} else {
				http.Error(w, "not found", 404)
			}
		}
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/tasks/"), "/")
	if len(parts) != 2 || f.tasks[parts[0]] == nil {
		http.Error(w, "not found", 404)
		return
	}
	if f.mode == "rejected" || f.mode == "bulk" && parts[0] == "task-456" {
		http.Error(w, "forbidden", 403)
		return
	}
	task := f.tasks[parts[0]]
	switch parts[1] {
	case "close":
		if due, ok := task["due"].(map[string]any); ok && due["is_recurring"] == true {
			due["date"] = "2026-10-01"
		} else {
			task["checked"] = true
		}
	case "move":
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		for key, value := range body {
			task[key] = value
		}
		if _, ok := body["project_id"]; !ok {
			task["project_id"] = "destination"
		}
		if f.mode == "adjusted" {
			task["project_id"] = "adjusted"
			task["section_id"] = nil
			task["parent_id"] = nil
		}
		if f.mode == "partial" {
			task["content"] = "Saved title"
			task["section_id"] = "returned-section"
		}
		if f.mode == "none" {
			task["section_id"], task["parent_id"] = nil, nil
		}
	default:
		http.Error(w, "unexpected action", 400)
		return
	}
	if f.mode == "uncertain" {
		http.Error(w, "request timed out after applying", 408)
		return
	}
	if f.mode == "truncated" {
		w.Header().Set("Content-Length", "1000")
		io.WriteString(w, `{"project_id":"destination"}`)
		return
	}
	if f.response != nil {
		io.WriteString(w, *f.response)
		return
	}
	if parts[1] == "close" {
		io.WriteString(w, "null")
	} else {
		write(task)
	}
}

func (f *taskActionFixture) snapshot() ([]string, map[string]map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, _ := json.Marshal(f.tasks)
	var tasks map[string]map[string]any
	_ = json.Unmarshal(data, &tasks)
	return append([]string(nil), f.requests...), tasks
}

func TestTaskActionCompletionContextAndState(t *testing.T) {
	for _, tc := range []struct {
		name, ref string
		due       any
		recurring bool
	}{
		{"ordinary", "id:task-123", nil, false},
		{"recurring", "id:task-123", map[string]any{"date": "2026-09-30", "string": "every day", "is_recurring": true}, true},
		{"unknown recurrence", "id:task-123", map[string]any{"date": "2026-09-30", "string": "every day"}, false},
		{"malformed recurrence", "id:task-123", map[string]any{"date": "2026-09-30", "is_recurring": "true"}, false},
		{"unique text", "Prepare launch checklist", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, ctx, out := newTaskActionFixture(t)
			f.tasks["task-123"]["due"] = tc.due
			if err := taskComplete(ctx, []string{tc.ref}); err != nil {
				t.Fatal(err)
			}
			requests, state := f.snapshot()
			lookup := "GET /tasks/task-123"
			if tc.ref == "Prepare launch checklist" {
				lookup = "GET /tasks"
			}
			if !reflect.DeepEqual(requests, []string{lookup, "POST /tasks/task-123/close"}) || state["task-123"]["checked"] != !tc.recurring {
				t.Fatalf("completion counts/state: %v %#v", requests, state)
			}
			want := "Completion accepted\nTask before completion: Prepare launch checklist\nID: task-123\n"
			if tc.recurring {
				want += "Recurring task; next due date not returned.\n"
				if state["task-123"]["due"].(map[string]any)["date"] != "2026-10-01" {
					t.Fatal(state)
				}
			}
			if out.String() != want {
				t.Fatalf("wrong acknowledgement: %q", out)
			}
		})
	}
}

func TestTaskActionMoveReturnedDestinationAndCounts(t *testing.T) {
	for _, adjusted := range []bool{false, true} {
		t.Run(fmt.Sprint(adjusted), func(t *testing.T) {
			f, ctx, out := newTaskActionFixture(t)
			if adjusted {
				f.mode = "adjusted"
			}
			if err := taskMove(ctx, []string{"id:task-123", "--project", "Home", "--section", "Backlog"}); err != nil {
				t.Fatal(err)
			}
			requests, state := f.snapshot()
			wantProject, wantSection := "destination", "new-section"
			want := "Project: Home\nSection: Backlog\n"
			if adjusted {
				wantProject, wantSection, want = "adjusted", "", "Project: Archive\nSection: None\n"
			}
			if !reflect.DeepEqual(requests, []string{"GET /tasks/task-123", "GET /projects", "GET /sections", "POST /tasks/task-123/move"}) || state["task-123"]["project_id"] != wantProject || !strings.Contains(out.String(), want) {
				t.Fatalf("returned destination/counts: %v %#v %s", requests, state, out)
			}
			if wantSection != "" && state["task-123"]["section_id"] != wantSection {
				t.Fatal(state)
			}
			if !strings.HasPrefix(out.String(), "Move accepted\nTask: Prepare launch checklist\nID: task-123\n") {
				t.Fatal(out)
			}
		})
	}
}

func TestTaskActionMoveUnknownFactsRetainAcceptedState(t *testing.T) {
	for _, tc := range []struct {
		name, response, mode string
		want, absent         string
	}{
		{"empty", "", "", "Requested section: new-section (name unavailable)", "Project:"},
		{"malformed", "{broken", "", "Destination details unavailable.", "Project:"},
		{"mismatch", `{"id":"other","project_id":"wrong"}`, "", "Requested section: new-section", "wrong"},
		{"partial", `{"id":"task-123","content":"Saved title","section_id":"returned-section"}`, "partial", "Section: returned-section (name unavailable)", "Section: old-section"},
		{"null", `{"project_id":"destination","section_id":null,"parent_id":null}`, "none", "Section: None", "Destination details unavailable"},
		{"wrong types", `{"content":42,"project_id":false,"section_id":5,"parent_id":{}}`, "", "Requested section: new-section", "Section: None"},
		{"truncated", "", "truncated", "Destination details unavailable.", "Project:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, ctx, out := newTaskActionFixture(t)
			f.response, f.mode = &tc.response, tc.mode
			if err := taskMove(ctx, []string{"--id", "task-123", "--project", "id:destination", "--section", "id:new-section"}); err != nil {
				t.Fatal(err)
			}
			requests, state := f.snapshot()
			var section, parent any = "new-section", nil
			if tc.mode == "partial" {
				section = "returned-section"
			}
			if tc.mode == "none" {
				section, parent = nil, nil
			}
			if !reflect.DeepEqual(requests, []string{"POST /tasks/task-123/move"}) || state["task-123"]["project_id"] != "destination" || state["task-123"]["section_id"] != section || state["task-123"]["parent_id"] != parent {
				t.Fatalf("accepted move was repeated or lost: %v %#v", requests, state)
			}
			if !strings.HasPrefix(out.String(), "Move accepted\n") || !strings.Contains(out.String(), tc.want) || strings.Contains(out.String(), tc.absent) || strings.Contains(out.String(), "retry") {
				t.Fatalf("unknown facts fabricated: %s", out)
			}
		})
	}
}

func TestTaskActionFailuresAndBulkPreserveOutcome(t *testing.T) {
	for _, action := range []string{"complete", "move"} {
		for _, mode := range []string{"rejected", "uncertain", "bulk"} {
			t.Run(action+"/"+mode, func(t *testing.T) {
				f, ctx, out := newTaskActionFixture(t)
				f.mode = mode
				args := []string{"--id", "task-123"}
				if mode == "bulk" {
					args = []string{"--filter", "today", "--yes"}
				}
				if action == "move" {
					args = append(args, "--project", "id:destination")
				}
				err := taskCompleteOrMove(ctx, action, args)
				requests, state := f.snapshot()
				changed := state["task-123"]["checked"] == true
				if action == "move" {
					changed = state["task-123"]["project_id"] == "destination"
				}
				if changed != (mode != "rejected") {
					t.Fatalf("failure state lost: %#v", state)
				}
				if mode == "bulk" {
					want := "task complete batch: accepted=1 rejected=1 uncertain=0 unattempted=0 dispatched=2 unchanged=0\n"
					if action == "move" {
						want = "task move batch: accepted=1 rejected=1 uncertain=0 unattempted=0 dispatched=2 unchanged=0\n"
					}
					if err == nil || !strings.HasPrefix(out.String(), want) || len(requests) != 3 || state["task-456"]["checked"] != false || state["task-456"]["project_id"] != "source" {
						t.Fatalf("bulk changed: %v %v %#v %s", err, requests, state, out)
					}
				} else {
					code := exitAuth
					if mode == "uncertain" {
						code = exitError
					}
					if toExitCode(err) != code || out.Len() != 0 || len(requests) != 1 {
						t.Fatalf("failure contract changed: %v %v %q", err, requests, out)
					}
				}
			})
		}
	}
}

func taskCompleteOrMove(ctx *Context, action string, args []string) error {
	if action == "move" {
		return taskMove(ctx, args)
	}
	return taskComplete(ctx, args)
}

func TestTaskActionCompatibilityAndDryRuns(t *testing.T) {
	for _, action := range []string{"complete", "move"} {
		for _, mode := range []output.Mode{output.ModeHuman, output.ModeJSON, output.ModeNDJSON, output.ModePlain} {
			t.Run(action+"/"+string(mode), func(t *testing.T) {
				f, ctx, out := newTaskActionFixture(t)
				ctx.Mode = mode
				ctx.Global.Quiet = mode == output.ModeHuman
				args := []string{"--id", "task-123"}
				if action == "move" {
					args = append(args, "--project", "id:destination")
				}
				if err := taskCompleteOrMove(ctx, action, args); err != nil {
					t.Fatal(err)
				}
				status := "completed"
				if action == "move" {
					status = "moved"
				}
				want := status + " task-123\n"
				if mode == output.ModeJSON {
					want = fmt.Sprintf("{\n  \"id\": \"task-123\",\n  \"status\": \"%s\"\n}\n", status)
				}
				if mode == output.ModeNDJSON {
					want = fmt.Sprintf("{\"id\":\"task-123\",\"status\":\"%s\"}\n", status)
				}
				requests, state := f.snapshot()
				changed := state["task-123"]["checked"] == true
				if action == "move" {
					changed = state["task-123"]["project_id"] == "destination"
				}
				if out.String() != want || len(requests) != 1 || !changed {
					t.Fatalf("compatibility: %q %v %#v", out, requests, state)
				}
				ctx.Global.DryRun = true
				out.Reset()
				if err := taskCompleteOrMove(ctx, action, args); err != nil {
					t.Fatal(err)
				}
				requestsAfter, stateAfter := f.snapshot()
				if !reflect.DeepEqual(requestsAfter, requests) || !reflect.DeepEqual(stateAfter, state) {
					t.Fatal("preview dispatched a mutation")
				}
				if mode == output.ModeHuman || mode == output.ModePlain {
					if !strings.HasPrefix(out.String(), "dry run: task "+action+";") {
						t.Fatal(out)
					}
				} else {
					var preview map[string]any
					if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
						t.Fatal(err)
					}
					payload := map[string]any{"id": "task-123"}
					if action == "move" {
						payload = map[string]any{"project_id": "destination"}
					}
					if preview["dry_run"] != true || preview["action"] != "task "+action || !reflect.DeepEqual(preview["payload"], payload) {
						t.Fatal(preview)
					}
				}
			})
		}
		f, ctx, out := newTaskActionFixture(t)
		ctx.Global.DryRun = true
		args := []string{"id:task-123"}
		if action == "move" {
			args = append(args, "--project", "Home", "--section", "Backlog")
		}
		if err := taskCompleteOrMove(ctx, action, args); err != nil {
			t.Fatal(err)
		}
		requests, state := f.snapshot()
		for _, request := range requests {
			if strings.HasPrefix(request, "POST") {
				t.Fatal(requests)
			}
		}
		if state["task-123"]["checked"] != false || state["task-123"]["project_id"] != "source" || !strings.Contains(out.String(), "no task changed.") || !strings.Contains(out.String(), "ID: task-123") || !strings.Contains(out.String(), "Authorization:") {
			t.Fatalf("human preview: %#v %s", state, out)
		}
		if action == "move" && !strings.Contains(out.String(), "Requested section: Backlog") {
			t.Fatal(out)
		}
	}
}

func TestTaskActionRedirectedExecute(t *testing.T) {
	f, _, _ := newTaskActionFixture(t)
	t.Setenv("TODOIST_TOKEN", "fixture")
	t.Setenv("TODOIST_BASE_URL", f.url)
	t.Setenv("TODOIST_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	var out, stderr bytes.Buffer
	code := executeTest([]string{"task", "move", "--id", "task-123", "--project", "id:destination"}, &out, &stderr)
	requests, state := f.snapshot()
	if code != exitOK || out.String() != "moved task-123\n" || stderr.Len() != 0 || len(requests) != 1 || state["task-123"]["project_id"] != "destination" {
		t.Fatalf("redirect contract: %d %q %q %v %#v", code, &out, &stderr, requests, state)
	}
}

func TestTaskActionIDOnlyAndDestinationShapes(t *testing.T) {
	for _, tc := range []struct{ action, flag, value, want string }{
		{"complete", "", "", "Completion accepted\nID: task-123\n"},
		{"move", "--section", "id:new-section", "Section: new-section (name unavailable)"},
		{"move", "--parent", "new-parent", "Parent task: new-parent (name unavailable)"},
	} {
		t.Run(tc.action+tc.flag, func(t *testing.T) {
			f, ctx, out := newTaskActionFixture(t)
			args := []string{"--id", "task-123"}
			if tc.flag != "" {
				args = append(args, tc.flag, tc.value)
			}
			if err := taskCompleteOrMove(ctx, tc.action, args); err != nil {
				t.Fatal(err)
			}
			requests, state := f.snapshot()
			expectedRequests := 1
			if tc.flag == "--parent" {
				expectedRequests = 3
			}
			if len(requests) != expectedRequests || !strings.Contains(out.String(), tc.want) || strings.Contains(out.String(), "before") {
				t.Fatalf("invented context: %v %s", requests, out)
			}
			if tc.action == "complete" && state["task-123"]["checked"] != true || tc.flag == "--section" && state["task-123"]["section_id"] != "new-section" || tc.flag == "--parent" && state["task-123"]["parent_id"] != "new-parent" {
				t.Fatal(state)
			}
		})
	}
}

func TestTaskActionControlsAndCachedSectionScope(t *testing.T) {
	f, ctx, out := newTaskActionFixture(t)
	title := "Launch\nID: forged\x1b[31m" + strings.Repeat("界", 160)
	f.tasks["task-123"]["content"] = title
	ctx.lookupCache = &lookupCache{
		projects:          []api.Project{{ID: "destination", Name: "Home\x1b[0m"}},
		sectionsByProject: map[string][]api.Section{"wrong": {{ID: "new-section", Name: "Wrong section", ProjectID: "wrong"}}},
	}
	if err := taskMove(ctx, []string{"id:task-123", "--project", "id:destination", "--section", "id:new-section"}); err != nil {
		t.Fatal(err)
	}
	requests, state := f.snapshot()
	if len(requests) != 2 || state["task-123"]["content"] != title || !strings.Contains(out.String(), strings.Repeat("界", 160)) || strings.ContainsAny(out.String(), "\x1b\r\t") || strings.Contains(out.String(), "\nID: forged") || strings.Contains(out.String(), "Wrong section") || !strings.Contains(out.String(), "Section: new-section (name unavailable)") {
		t.Fatalf("unsafe or misnamed output: %v %s", requests, out)
	}
}
