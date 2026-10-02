package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

type editingRequest struct {
	method, path string
	body         map[string]any
	requestID    string
}
type editingFixture struct {
	tasks      map[string]map[string]any
	requests   []editingRequest
	restStatus int
	syncStatus string
	optional   string
	page       func(http.ResponseWriter, *http.Request) bool
}

func editingTestContext(t *testing.T) (*editingFixture, *Context, *bytes.Buffer) {
	t.Helper()
	f := &editingFixture{tasks: map[string]map[string]any{}, restStatus: 200, syncStatus: `"ok"`}
	f.tasks["t"] = map[string]any{"id": "t", "content": "Report", "description": "Notes", "project_id": "p", "section_id": nil, "parent_id": nil, "labels": []any{"work"}, "priority": 4, "child_order": 9, "checked": false, "due": map[string]any{"date": "2026-10-01", "timezone": nil, "is_recurring": true, "string": "every day", "lang": "en"}}
	ctx, out := captureTestContext(t, f.serve)
	ctx.Mode = output.ModeJSON
	ctx.Stderr = &bytes.Buffer{}
	ctx.Stdin = strings.NewReader("")
	ctx.ConfigPath = filepath.Join(t.TempDir(), "config.json")
	return f, ctx, out
}
func (f *editingFixture) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	request := editingRequest{method: r.Method, path: r.URL.Path, requestID: r.Header.Get("X-Request-Id")}
	if r.Method != "GET" && r.URL.Path != "/sync" {
		_ = json.NewDecoder(r.Body).Decode(&request.body)
	}
	f.requests = append(f.requests, request)
	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	if r.Method == "GET" {
		if f.page != nil && f.page(w, r) {
			return
		}
		switch r.URL.Path {
		case "/projects":
			write(map[string]any{"results": []any{map[string]any{"id": "p", "name": "Inbox", "inbox_project": true}}, "next_cursor": nil})
		case "/labels":
			write(map[string]any{"results": []any{map[string]any{"id": "l", "name": "work"}}, "next_cursor": nil})
		case "/tasks", "/tasks/filter", "/tasks/completed/by_completion_date", "/tasks/completed/by_due_date":
			rows := []any{}
			for _, id := range []string{"t", "u", "v"} {
				if task := f.tasks[id]; task != nil {
					if parent := r.URL.Query().Get("parent_id"); parent != "" && task["parent_id"] != parent {
						continue
					}
					rows = append(rows, task)
				}
			}
			key := "results"
			if strings.Contains(r.URL.Path, "completed") {
				key = "items"
			}
			write(map[string]any{key: rows, "next_cursor": nil})
		default:
			if task := f.tasks[strings.TrimPrefix(r.URL.Path, "/tasks/")]; task != nil {
				write(task)
			} else {
				http.NotFound(w, r)
			}
		}
		return
	}
	if r.URL.Path == "/sync" {
		_ = r.ParseForm()
		if r.Form.Get("commands") == "" {
			write(map[string]any{"filters": []any{map[string]any{"id": "f", "name": "Work", "query": "today"}}})
			return
		}
		var commands []struct {
			Type, UUID string
			Args       map[string]any
		}
		_ = json.Unmarshal([]byte(r.Form.Get("commands")), &commands)
		if len(commands) != 1 {
			http.Error(w, "one command expected", 400)
			return
		}
		c := commands[0]
		f.requests[len(f.requests)-1].body = map[string]any{"type": c.Type, "args": c.Args}
		id, _ := c.Args["id"].(string)
		task := f.tasks[id]
		if f.syncStatus == `"ok"` && task != nil {
			switch c.Type {
			case "item_update":
				for k, v := range c.Args {
					if k != "id" {
						task[k] = v
					}
				}
			case "item_move":
				task["parent_id"] = nil
				if section, ok := c.Args["section_id"]; ok {
					task["section_id"] = section
				} else {
					task["section_id"] = nil
				}
			case "item_complete":
				task["checked"] = true
			}
		}
		item, _ := json.Marshal(task)
		if f.optional != "" {
			item = []byte(f.optional)
		}
		fmt.Fprintf(w, `{"sync_status":{%q:%s},"items":[%s]}`, c.UUID, f.syncStatus, item)
		return
	}
	if f.restStatus != 200 {
		w.WriteHeader(f.restStatus)
		fmt.Fprint(w, `{"error":"fixture failure"}`)
		return
	}
	if r.URL.Path == "/tasks" {
		task := request.body
		task["id"] = "created"
		f.tasks["created"] = task
		if f.optional != "" {
			fmt.Fprint(w, f.optional)
		} else {
			write(task)
		}
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/tasks/")
	id, action, _ := strings.Cut(path, "/")
	task := f.tasks[id]
	if task == nil {
		http.NotFound(w, r)
		return
	}
	if action == "close" {
		task["checked"] = true
	} else {
		for k, v := range request.body {
			task[k] = v
		}
	}
	if f.optional != "" {
		fmt.Fprint(w, f.optional)
	} else {
		write(task)
	}
}
func (f *editingFixture) writes() []editingRequest {
	var result []editingRequest
	for _, r := range f.requests {
		if r.method != "GET" {
			result = append(result, r)
		}
	}
	return result
}
func taskFromJSON(t *testing.T, value string) api.Task {
	t.Helper()
	var task api.Task
	if err := json.Unmarshal([]byte(value), &task); err != nil {
		t.Fatal(err)
	}
	return task
}

func TestTaskEditingPresenceAndExactRequests(t *testing.T) {
	f, ctx, out := editingTestContext(t)
	ctx.Stdin = strings.NewReader("  exact notes\n")
	if err := taskUpdate(ctx, []string{"--id", "t", "--description", "-", "--clear-labels", "--clear-assignee", "--clear-deadline", "--reference=false", "--order", "0"}); err != nil {
		t.Fatal(err)
	}
	writes := f.writes()
	want := map[string]any{"description": "  exact notes\n", "labels": []any{}, "assignee_id": nil, "deadline_date": nil, "content": "Report", "child_order": float64(0)}
	if len(writes) != 1 || writes[0].path != "/tasks/t" || !reflect.DeepEqual(writes[0].body, want) || writes[0].requestID == "" {
		t.Fatalf("writes=%#v want=%#v", writes, want)
	}
	if !strings.Contains(out.String(), "Report") {
		t.Fatal(out)
	}
	for _, input := range []string{"", "\n"} {
		f, ctx, _ := editingTestContext(t)
		ctx.Stdin = strings.NewReader(input)
		if err := taskUpdate(ctx, []string{"--id", "t", "--description", "-"}); err != nil {
			t.Fatal(err)
		}
		if f.writes()[0].body["description"] != input {
			t.Fatal(f.writes())
		}
	}
	f, ctx, _ = editingTestContext(t)
	if err := taskAdd(ctx, []string{"--content", "Report", "--reference=true", "--order", "-2147483648"}); err != nil {
		t.Fatal(err)
	}
	if body := f.writes()[0].body; body["content"] != "* Report" || body["order"] != float64(-2147483648) {
		t.Fatal(body)
	}
}
func TestTaskEditingConflictsHaveZeroRequests(t *testing.T) {
	for _, args := range [][]string{
		{"--clear-due", "--due", "today"}, {"--due-date", "2026-10-01", "--due-datetime", "2026-10-01T12:00:00Z"},
		{"--clear-labels", "--label", "work"}, {"--clear-description", "--description", "-"},
		{"--clear-assignee", "--assignee", "me"}, {"--clear-deadline", "--deadline", "2026-10-01"},
		{"--content", "-", "--description", "-"}, {"--label", ""}, {"--duration-unit", ""},
		{"--duration", "5"}, {"--duration-unit", "minute"}, {"--description", "-", "--content", "-"},
		{"--order", "2147483648"}, {"--content", "* * Ambiguous", "--reference=true"},
		{"--clear-due", "--natural", "--content", "Title DUE:today"},
		{"--due", "tomorrow", "--natural", "--content", "Title DUE:today"},
		{"--priority", "p2", "--natural", "--content", "Title P1"},
		{"--natural", "--content", "Title p1 P2"}, {"--natural", "--content", "Title due:today DUE:tomorrow"},
		{"--due-datetime", "2026-10-01T1:00:00Z"}, {"--due-datetime", "2026-10-01T12:00:00+24:00"},
		{"--due-datetime", "2026-10-01T12:00:00.1234567899Z"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			f, ctx, _ := editingTestContext(t)
			err := taskUpdate(ctx, append([]string{"--id", "t"}, args...))
			if toExitCode(err) != exitUsage || len(f.requests) != 0 {
				t.Fatalf("err=%v requests=%v", err, f.requests)
			}
		})
	}
	f, ctx, _ := editingTestContext(t)
	if err := taskUpdate(ctx, []string{"--id=", "Title", "--clear-due"}); toExitCode(err) != exitUsage || len(f.requests) != 0 {
		t.Fatal(err, f.requests)
	}
}
func TestTaskEditingReferenceNoopAndMissingIdentity(t *testing.T) {
	for _, reference := range []bool{false, true} {
		f, ctx, out := editingTestContext(t)
		if reference {
			f.tasks["t"]["content"] = "* Report"
		}
		if err := taskUpdate(ctx, []string{"--id", "t", fmt.Sprintf("--reference=%v", reference)}); err != nil {
			t.Fatal(err)
		}
		if len(f.writes()) != 0 || !strings.Contains(out.String(), `"status": "unchanged"`) {
			t.Fatal(f.writes(), out)
		}
	}
	f, ctx, _ := editingTestContext(t)
	delete(f.tasks["t"], "id")
	if err := taskUpdate(ctx, []string{"Report", "--description", "x"}); err == nil || len(f.writes()) != 0 {
		t.Fatal(err, f.writes())
	}
}
func TestTaskEditingSequentialClearAndRecovery(t *testing.T) {
	for _, status := range []int{200, 403, 408, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f, ctx, out := editingTestContext(t)
			f.restStatus = status
			err := taskUpdate(ctx, []string{"--id", "t", "--clear-due", "--description", ""})
			writes := f.writes()
			if len(writes) != 2 || writes[0].path != "/sync" || writes[1].path != "/tasks/t" || f.tasks["t"]["due"] != nil {
				t.Fatal(err, writes)
			}
			if status == 200 {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil || !strings.Contains(out.String(), `"status": "partial"`) || !strings.Contains(out.String(), `"operation": "clear_due"`) {
					t.Fatal(err, out)
				}
			}
		})
	}
	for _, status := range []string{`{"error":"rejected"}`, `{}`, `"unknown"`} {
		f, ctx, out := editingTestContext(t)
		f.syncStatus = status
		if err := taskUpdate(ctx, []string{"--id", "t", "--clear-due", "--clear-labels"}); err == nil || len(f.writes()) != 1 || out.Len() != 0 {
			t.Fatal(err, f.writes(), out)
		}
	}
	f, ctx, out := editingTestContext(t)
	f.optional = `{broken`
	if err := taskUpdate(ctx, []string{"--id", "t", "--description", ""}); err != nil || len(f.writes()) != 1 || !strings.Contains(out.String(), `"result_available": false`) {
		t.Fatal(err, out)
	}
}
func TestTaskEditingReadOnlyAndDryRunZeroMutations(t *testing.T) {
	for _, args := range [][]string{{"task", "update", "--id", "t", "--clear-due", "--clear-labels"}, {"task", "move", "--id", "t", "--clear-parent"}, {"task", "reschedule", "--id", "t", "--due-date", "2026-10-02"}, {"task", "complete", "--id", "t", "--forever"}, {"task", "update", "--id", "t", "--reference=true"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			f, _, _ := editingTestContext(t)
			f.tasks["t"]["parent_id"] = "parent"
			f.tasks["parent"] = map[string]any{"id": "parent", "parent_id": nil, "project_id": "p", "section_id": nil}
			server := httptest.NewServer(http.HandlerFunc(f.serve))
			defer server.Close()
			path := authorizationFixture(t, readOnlyMetadata)
			code, _, stderr := executeAuthorization(t, path, append([]string{"--base-url", server.URL, "--json"}, args...)...)
			if code != exitAuth || len(f.writes()) != 0 || !strings.Contains(stderr, "READ_ONLY") {
				t.Fatal(code, stderr, f.writes())
			}
			code, _, stderr = executeAuthorization(t, path, append([]string{"--base-url", server.URL, "--json", "--dry-run"}, args...)...)
			if code != exitOK || len(f.writes()) != 0 {
				t.Fatal(code, stderr, f.writes())
			}
		})
	}
}
func TestTaskHierarchyClearingRequestsAndNoops(t *testing.T) {
	for _, tc := range []struct {
		name                                      string
		child, section, clearParent, clearSection bool
		want                                      string
		fail                                      bool
	}{
		{"child section detach", true, true, true, false, "section_id", false},
		{"child project detach", true, false, true, false, "project_id", false},
		{"child inherited section", true, true, false, true, "", true},
		{"child both", true, true, true, true, "project_id", false},
		{"root clear section", false, true, false, true, "project_id", false},
		{"root detach noop", false, true, true, false, "", false},
		{"sectionless child section noop", true, false, false, true, "", false},
		{"root both noop", false, false, true, true, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, ctx, out := editingTestContext(t)
			var section any
			if tc.section {
				section = "s"
			}
			f.tasks["t"]["section_id"] = section
			if tc.child {
				f.tasks["t"]["parent_id"] = "parent"
				f.tasks["t"]["section_id"] = nil
				f.tasks["parent"] = map[string]any{"id": "parent", "parent_id": nil, "section_id": section, "project_id": "p"}
			}
			args := []string{"--id", "t"}
			if tc.clearParent {
				args = append(args, "--clear-parent")
			}
			if tc.clearSection {
				args = append(args, "--clear-section")
			}
			err := taskMove(ctx, args)
			if (err != nil) != tc.fail {
				t.Fatal(err)
			}
			if tc.want == "" {
				if len(f.writes()) != 0 {
					t.Fatal(f.writes())
				}
				if !tc.fail && !strings.Contains(out.String(), "unchanged") {
					t.Fatal(out)
				}
				return
			}
			writes := f.writes()
			if len(writes) != 1 || writes[0].path != "/sync" {
				t.Fatal(writes)
			}
			body := writes[0].body["args"].(map[string]any)
			want := map[string]any{"id": "t", tc.want: "p"}
			if tc.want == "section_id" {
				want[tc.want] = "s"
			}
			if !reflect.DeepEqual(body, want) {
				t.Fatal(body, want)
			}
		})
	}
}
func TestTaskHierarchyAndBatchPreflightRefusals(t *testing.T) {
	for _, kind := range []string{"cycle", "contradictory", "missing", "duplicate", "overlap"} {
		t.Run(kind, func(t *testing.T) {
			f, ctx, out := editingTestContext(t)
			args := []string{"--id", "t", "--clear-parent"}
			f.tasks["t"]["parent_id"] = "parent"
			f.tasks["parent"] = map[string]any{"id": "parent", "parent_id": nil, "project_id": "p", "section_id": nil}
			switch kind {
			case "cycle":
				f.tasks["parent"]["parent_id"] = "t"
			case "contradictory":
				f.tasks["t"]["section_id"] = "wrong"
			case "missing":
				delete(f.tasks, "parent")
			case "duplicate":
				f.page = func(w http.ResponseWriter, r *http.Request) bool {
					if r.URL.Path != "/tasks/filter" {
						return false
					}
					data, _ := json.Marshal(f.tasks["t"])
					fmt.Fprintf(w, `{"results":[%s,%s],"next_cursor":null}`, data, data)
					return true
				}
				args = []string{"--filter", "today", "--yes", "--clear-parent"}
			case "overlap":
				f.tasks["u"] = map[string]any{"id": "u", "parent_id": "t", "project_id": "p", "section_id": nil}
				args = []string{"--filter", "today", "--yes", "--clear-parent"}
			}
			if err := taskMove(ctx, args); err == nil || len(f.writes()) != 0 {
				t.Fatal(err, f.writes(), out)
			}
		})
	}
}
func TestTaskBatchUncertaintyStopsAndCountsNoop(t *testing.T) {
	f, ctx, out := editingTestContext(t)
	f.tasks["t"]["parent_id"] = "parent"
	f.tasks["parent"] = map[string]any{"id": "parent", "parent_id": nil, "project_id": "p", "section_id": nil}
	f.tasks["u"] = map[string]any{"id": "u", "parent_id": nil, "project_id": "p", "section_id": nil}
	f.tasks["v"] = map[string]any{"id": "v", "parent_id": "other", "project_id": "p", "section_id": nil}
	f.tasks["other"] = map[string]any{"id": "other", "parent_id": nil, "project_id": "p", "section_id": nil}
	f.syncStatus = `{}`
	if err := taskMove(ctx, []string{"--filter", "today", "--yes", "--clear-parent"}); err == nil {
		t.Fatal("missing error")
	}
	var result map[string]any
	_ = json.Unmarshal(out.Bytes(), &result)
	if len(f.writes()) != 1 || result["accepted"] != float64(1) || result["uncertain"] != float64(1) || result["unattempted"] != float64(1) || result["unchanged"] != float64(1) {
		t.Fatal(f.writes(), result)
	}
}
func TestTaskForeverUsesNativeCompletion(t *testing.T) {
	f, ctx, _ := editingTestContext(t)
	if err := taskComplete(ctx, []string{"--id", "t", "--forever"}); err != nil {
		t.Fatal(err)
	}
	writes := f.writes()
	if len(writes) != 1 || writes[0].path != "/sync" || writes[0].body["type"] != "item_complete" || f.tasks["t"]["checked"] != true || f.tasks["t"]["due"] == nil {
		t.Fatal(writes, f.tasks)
	}
}
func TestOrdinaryAgentTaskPendingRecovery(t *testing.T) {
	for _, status := range []int{200, 403, 408, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f, ctx, _ := editingTestContext(t)
			f.restStatus = status
			actions := []Action{{Type: "task_update", TaskID: "t", Content: "Changed"}, {Type: "task_update", TaskID: "t", Description: "Next"}}
			results, err := applyActionsWithMode(ctx, "token", actions, applyErrorModeContinue)
			journal, readErr := readReplayJournal(replayJournalPath(ctx))
			if readErr != nil {
				t.Fatal(readErr)
			}
			uncertain := status == 408 || status == 503
			if uncertain {
				if err == nil || len(results) != 1 || len(f.writes()) != 1 || len(journal.Pending) != 1 {
					t.Fatal(err, results, f.writes(), journal)
				}
				before := len(f.requests)
				if _, err := applyActionsWithMode(ctx, "token", actions, applyErrorModeContinue); err == nil || len(f.requests) != before {
					t.Fatal(err, f.requests)
				}
			} else {
				if err != nil || len(f.writes()) != 2 || len(journal.Pending) != 0 {
					t.Fatal(err, f.writes(), journal)
				}
				if status == 200 && len(journal.Applied) != 2 {
					t.Fatal(journal)
				}
			}
		})
	}
	f, ctx, _ := editingTestContext(t)
	ctx.ConfigPath = filepath.Join(t.TempDir(), "missing", "config.json")
	if err := os.WriteFile(filepath.Dir(ctx.ConfigPath), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := applyActionsWithMode(ctx, "token", []Action{{Type: "task_update", TaskID: "t", Content: "Changed"}}, applyErrorModeFail); err == nil || len(f.writes()) != 0 {
		t.Fatal(err, f.writes())
	}
}

func TestHierarchyRefreshRefusesChangedRootAndMalformedPlacement(t *testing.T) {
	for _, kind := range []string{"changed root", "malformed section"} {
		f, ctx, _ := editingTestContext(t)
		task := taskFromJSON(t, `{"id":"t","parent_id":null}`)
		if kind == "changed root" {
			f.tasks["t"]["parent_id"] = "parent"
		} else {
			f.tasks["t"]["section_id"] = true
		}
		ancestry, err := newTaskAncestry(ctx, []api.Task{task})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := ancestry.clearDestination("t", true, false); err == nil || len(f.writes()) != 0 {
			t.Fatal(kind, err)
		}
	}
}
func TestInboxSortedMachineModesAndExpandedIDsOnlyGuard(t *testing.T) {
	f, ctx, _ := editingTestContext(t)
	t.Setenv("TODOIST_TOKEN", "synthetic")
	t.Setenv("TODOIST_BASE_URL", ctx.Client.BaseURL)
	t.Setenv("TODOIST_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	for _, flags := range [][]string{{"--ids-only"}, {"--json", "--task-output-version", "2"}, {"--ndjson", "--task-output-version", "2"}} {
		var out, stderr bytes.Buffer
		code := Execute(append([]string{"inbox", "--sort", "order"}, flags...), &out, &stderr)
		if code != exitOK || out.Len() == 0 || stderr.Len() != 0 {
			t.Fatal(code, out.String(), stderr.String())
		}
	}
	before := len(f.requests)
	var out, stderr bytes.Buffer
	if code := Execute([]string{"task", "view", "--id", "t", "--include-children", "--ids-only"}, &out, &stderr); code != exitUsage || len(f.requests) != before {
		t.Fatal(code, f.requests)
	}
}

func TestTaskWriteErrorPreservesLatestRequestID(t *testing.T) {
	_, ctx, _ := editingTestContext(t)
	ctx.RequestID = "earlier-read"
	writeError(ctx, &api.TaskWriteError{Outcome: "uncertain", RequestID: "dispatched-write", Err: fmt.Errorf("lost response")})
	if !strings.Contains(ctx.Stderr.(*bytes.Buffer).String(), `"request_id": "dispatched-write"`) {
		t.Fatal(ctx.Stderr)
	}
}

func TestTaskEditingRejectsEmptyNormalizedSelectorsBeforeReads(t *testing.T) {
	for _, command := range []string{"add", "move", "reschedule"} {
		f, ctx, _ := editingTestContext(t)
		var err error
		switch command {
		case "add":
			err = taskAdd(ctx, []string{"--content", "Title", "--parent", "id:"})
		case "move":
			err = taskMove(ctx, []string{"--id", "t", "--parent", "id:"})
		case "reschedule":
			err = taskReschedule(ctx, []string{"id:t", "--id=", "--due-date", "2026-10-02"})
		}
		if toExitCode(err) != exitUsage || len(f.requests) != 0 {
			t.Fatal(command, err, f.requests)
		}
	}
}
func TestTaskHierarchyRejectsEmptyAndUnsafePlacementEvidence(t *testing.T) {
	for _, field := range []string{"parent_id", "section_id", "project_id"} {
		for _, value := range []string{"", " ", "bad\nID"} {
			f, ctx, _ := editingTestContext(t)
			f.tasks["t"][field] = value
			if err := taskMove(ctx, []string{"--id", "t", "--clear-parent"}); err == nil || len(f.writes()) != 0 {
				t.Fatal(field, value, err, f.writes())
			}
		}
	}
}
func TestTaskWriteFallbackQuotesOpaqueReference(t *testing.T) {
	_, ctx, out := editingTestContext(t)
	ctx.Mode = output.ModePlain
	if err := writeTaskAcknowledgement(ctx, "a;b'c", "task_update", "accepted"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `view 'id:a;b'"'"'c' --json`) {
		t.Fatal(out)
	}
}

func TestTaskBatchHumanRecoveryIncludesEveryTarget(t *testing.T) {
	_, ctx, out := editingTestContext(t)
	ctx.Mode = output.ModePlain
	targets := []taskBatchTarget{{ID: "accepted", Outcome: "accepted", Dispatched: true}, {ID: "uncertain", Outcome: "uncertain", Dispatched: true, RequestID: "write-id"}, {ID: "remaining", Outcome: "unattempted"}}
	if err := writeTaskBatch(ctx, "today", "task complete", targets, nil); err == nil {
		t.Fatal("missing incomplete error")
	}
	for _, part := range []string{"ID: accepted · accepted · dispatched=true", "ID: uncertain · uncertain · dispatched=true · request_id=write-id", "ID: remaining · unattempted · dispatched=false"} {
		if !strings.Contains(out.String(), part) {
			t.Fatal(out)
		}
	}
}
func TestTaskEditingKnownNoopAllowedReadOnly(t *testing.T) {
	f, ctx, _ := editingTestContext(t)
	path := authorizationFixture(t, readOnlyMetadata)
	for _, args := range [][]string{{"task", "update", "--id", "t", "--reference=false"}, {"task", "move", "--id", "t", "--clear-parent"}} {
		code, out, stderr := executeAuthorization(t, path, append([]string{"--base-url", ctx.Client.BaseURL, "--json"}, args...)...)
		if code != exitOK || len(f.writes()) != 0 || !strings.Contains(out, `"status": "unchanged"`) || stderr != "" {
			t.Fatal(code, out, stderr)
		}
	}
}
func TestTaskEditingBatchRedactsRejectedCredential(t *testing.T) {
	_, ctx, out := editingTestContext(t)
	ctx.Token = "secret-marker"
	_ = writeBatchPreflightFailure(ctx, "today", []api.Task{{ID: "t"}}, "task move", fmt.Errorf("upstream secret-marker"))
	if strings.Contains(out.String(), ctx.Token) {
		t.Fatal(out)
	}
}

func TestReferenceOnlyTextEditUsesExactTask(t *testing.T) {
	for _, tc := range []struct {
		name, exact, flag, want string
		writes                  int
		missing                 bool
	}{
		{name: "add prefix to current title", exact: "Report changed", flag: "--reference=true", want: "* Report changed", writes: 1},
		{name: "remove current prefix", exact: "* Report changed", flag: "--reference=false", want: "Report changed", writes: 1},
		{name: "current reference already matches", exact: "* Report changed", flag: "--reference=true", want: "* Report changed"},
		{name: "missing exact task refuses write", flag: "--reference=true", missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, ctx, _ := editingTestContext(t)
			f.tasks["t"]["content"] = tc.exact
			if tc.missing {
				delete(f.tasks, "t")
			}
			f.page = func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != "/tasks" {
					return false
				}
				fmt.Fprint(w, `{"results":[{"id":"t","content":"Report"}],"next_cursor":null}`)
				return true
			}
			err := taskUpdate(ctx, []string{"Report", tc.flag})
			if tc.missing {
				if toExitCode(err) != exitNotFound {
					t.Fatalf("expected exact-task lookup failure, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if len(f.requests) < 2 || f.requests[0].path != "/tasks" || f.requests[1].method != "GET" || f.requests[1].path != "/tasks/t" {
				t.Fatalf("expected selection then exact read, got %#v", f.requests)
			}
			writes := f.writes()
			if len(writes) != tc.writes {
				t.Fatalf("writes=%d, want %d: %#v", len(writes), tc.writes, writes)
			}
			if tc.writes != 0 && writes[0].body["content"] != tc.want {
				t.Fatalf("title=%v, want %q", writes[0].body["content"], tc.want)
			}
		})
	}
}
