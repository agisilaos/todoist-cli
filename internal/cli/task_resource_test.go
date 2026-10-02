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
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func taskResourceFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../api/testdata/tasks/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeTaskResource(t *testing.T, data string) map[string]any {
	t.Helper()
	var result map[string]any
	decoder := json.NewDecoder(strings.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTaskResourceV2CommandSurfaces(t *testing.T) {
	data := taskResourceFixture(t, "populated")
	want := decodeTaskResource(t, string(data))
	// Execute uses the wall clock. Keep this fidelity fixture inside the upcoming
	// window, with room for a UTC midnight rollover during the command matrix.
	want["due"].(map[string]any)["date"] = time.Now().UTC().AddDate(0, 0, 3).Format("2006-01-02T07:00:00Z")
	var err error
	data, err = json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	delete(want, "future_field")
	delete(want["due"].(map[string]any), "future_due_field")
	want["reference_item"] = map[string]any{"is_reference": true, "source": "content_prefix"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/projects":
			fmt.Fprint(w, `{"results":[{"id":"project-fixture","name":"Inbox","inbox_project":true}]}`)
		case r.URL.Path == "/labels":
			fmt.Fprint(w, `{"results":[{"id":"labelfixture","name":"work"}]}`)
		case r.URL.Path == "/sync":
			fmt.Fprint(w, `{"filters":[{"id":"filterfixture","name":"Work","query":"@work"}]}`)
		case strings.HasPrefix(r.URL.Path, "/tasks/completed/"):
			fmt.Fprintf(w, `{"items":[%s],"next_cursor":null}`, data)
		case r.Method == http.MethodGet && (r.URL.Path == "/tasks" || r.URL.Path == "/tasks/filter"):
			fmt.Fprintf(w, `{"results":[%s],"next_cursor":null}`, data)
		case strings.HasPrefix(r.URL.Path, "/tasks"):
			w.Write(data)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	path := authorizationFixture(t, "")
	for _, tc := range []struct {
		args []string
		list bool
	}{
		{[]string{"task", "view", "id:taskfixture"}, false},
		{[]string{"task", "show", "id:taskfixture", "--full"}, false},
		{[]string{"task", "list"}, true},
		{[]string{"task", "ls"}, true},
		{[]string{"task", "list", "--completed"}, true},
		{[]string{"task", "add", "--content", "input differs from returned content"}, true},
		{[]string{"task", "add", "--quick", "--content", "input differs"}, true},
		{[]string{"task", "update", "id:taskfixture", "--content", "input differs"}, true},
		{[]string{"add", "input differs"}, true},
		{[]string{"inbox", "add", "input differs"}, true},
		{[]string{"inbox"}, true},
		{[]string{"today"}, true},
		{[]string{"upcoming"}, true},
		{[]string{"completed"}, true},
		{[]string{"completed", "--completed-by", "due"}, true},
		{[]string{"filter", "show", "id:filterfixture"}, true},
		{[]string{"view", "https://app.todoist.com/app/task/title-taskfixture"}, false},
		{[]string{"view", "--help=false", "https://app.todoist.com/app/task/title-taskfixture"}, false},
		{[]string{"view", "-h=false", "https://app.todoist.com/app/task/title-taskfixture"}, false},
		{[]string{"view", "https://app.todoist.com/app/filter/title-filterfixture"}, true},
		{[]string{"view", "https://app.todoist.com/app/label/title-labelfixture"}, true},
		{[]string{"view", "https://app.todoist.com/app/inbox"}, true},
		{[]string{"view", "https://app.todoist.com/app/today"}, true},
		{[]string{"view", "https://app.todoist.com/app/upcoming"}, true},
		{[]string{"view", "https://app.todoist.com/app/completed"}, true},
	} {
		for _, mode := range []string{"--json", "--ndjson"} {
			t.Run(strings.Join(tc.args, " ")+mode, func(t *testing.T) {
				args := append([]string{"--base-url", server.URL, "--no-input", mode, "--task-output-version", "2"}, tc.args...)
				code, out, errOut := executeAuthorization(t, path, args...)
				if code != 0 || errOut != "" {
					t.Fatalf("exit %d stdout %s stderr %s", code, out, errOut)
				}
				if mode == "--json" && tc.list {
					var records []json.RawMessage
					if err := json.Unmarshal([]byte(out), &records); err != nil || len(records) != 1 {
						t.Fatalf("expected raw one-task array: %s (%v)", out, err)
					}
					out = string(records[0])
				} else if mode == "--ndjson" && (!strings.HasSuffix(out, "\n") || strings.Count(out, "\n") != 1) {
					t.Fatalf("expected one newline-terminated record: %q", out)
				}
				if got := decodeTaskResource(t, out); !reflect.DeepEqual(got, want) {
					t.Fatalf("returned facts changed: got %#v want %#v", got, want)
				}
			})
		}
	}
}

func TestTaskResourceV2PresenceAndDiagnostics(t *testing.T) {
	for _, name := range []string{"absent", "null", "false-zero-empty", "malformed", "integral-numbers"} {
		var task api.Task
		data := taskResourceFixture(t, name)
		if err := json.Unmarshal(data, &task); err != nil {
			t.Fatal(err)
		}
		var resources []map[string]any
		for _, mode := range []output.Mode{output.ModeJSON, output.ModeNDJSON} {
			var out, errOut bytes.Buffer
			ctx := &Context{Stdout: &out, Stderr: &errOut, Mode: mode, Global: GlobalOptions{TaskOutputVersion: 2}}
			if err := writeTaskView(ctx, task, false); err != nil {
				t.Fatal(err)
			}
			resources = append(resources, decodeTaskResource(t, out.String()))
			if name == "malformed" {
				if strings.Count(errOut.String(), "malformed fact omitted.") != 8 || !strings.Contains(errOut.String(), "due.lang unavailable (expected string, returned number)") {
					t.Fatalf("diagnostics missing or on wrong stream: %s", errOut.String())
				}
			} else if errOut.Len() != 0 {
				t.Fatal(errOut.String())
			}
		}
		if !reflect.DeepEqual(resources[0], resources[1]) {
			t.Fatal("JSON and NDJSON differ")
		}
		got := resources[0]
		switch name {
		case "absent":
			for _, key := range []string{"checked", "due", "child_order", "responsible_uid", "deadline", "duration"} {
				if _, present := got[key]; present {
					t.Fatalf("absent %s invented: %#v", key, got)
				}
			}
		case "null":
			for _, key := range []string{"content", "due", "child_order", "responsible_uid", "deadline", "duration"} {
				if value, present := got[key]; !present || value != nil {
					t.Fatalf("null %s lost: %#v", key, got)
				}
			}
			if _, present := got["reference_item"]; present {
				t.Fatal("null content classified")
			}
		case "false-zero-empty":
			if got["checked"] != false || got["child_order"] != json.Number("0") || got["responsible_uid"] != "" || len(got["labels"].([]any)) != 0 || got["due"].(map[string]any)["is_recurring"] != false {
				t.Fatalf("false/zero/empty lost: %#v", got)
			}
		case "malformed":
			if _, present := got["responsible_uid"]; present || got["deadline"].(map[string]any)["date"] != "2026-10-10" {
				t.Fatalf("invalid facts included or valid sibling lost: %#v", got)
			}
		case "integral-numbers":
			if got["child_order"] != json.Number("0.0") || got["day_order"] != json.Number("9007199254740993.0") || got["duration"].(map[string]any)["amount"] != json.Number("6e1") {
				t.Fatalf("integral numeric representation changed: %#v", got)
			}
		}
	}
}

func TestTaskResourceLegacySelectionUnchanged(t *testing.T) {
	var task api.Task
	if err := json.Unmarshal(taskResourceFixture(t, "populated"), &task); err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []int{0, 1} {
		for _, mode := range []output.Mode{output.ModeJSON, output.ModeNDJSON} {
			var out, errOut bytes.Buffer
			ctx := &Context{Stdout: &out, Stderr: &errOut, Mode: mode, Global: GlobalOptions{TaskOutputVersion: version}}
			if err := writeTaskView(ctx, task, true); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decodeTaskResource(t, out.String()), decodeTaskResource(t, string(want))) || errOut.Len() != 0 {
				t.Fatalf("legacy output changed: %s %s", out.String(), errOut.String())
			}
		}
	}
}

func TestTaskOutputVersionRejectedBeforeSideEffects(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "uncreated", "config.json")
	for _, args := range [][]string{
		{"task", "view", "id:taskfixture"},
		{"--plain", "task", "view", "id:taskfixture"},
		{"--ids-only", "task", "list"},
		{"--json", "task", "complete", "id:taskfixture", "--yes"},
		{"--json", "task", "delete", "id:taskfixture", "--yes"},
		{"--json", "project", "list"},
		{"--json", "agent", "schedule", "print"},
		{"--json", "review", "--out", filepath.Join(dir, "review")},
		{"--json", "--dry-run", "task", "add", "--content", "-"},
		{"--json", "view", "https://app.todoist.com/app/project/title-projectfixture"},
		{"--json", "inbox", "remove"},
		{"--json", "skill", "install", "codex", "--path", filepath.Join(dir, "skills")},
		{"--json", "skill", "list", "--path", filepath.Join(dir, "skills")},
	} {
		for _, version := range []string{"1", "2"} {
			fullArgs := append([]string{"--base-url", server.URL, "--task-output-version=" + version}, args...)
			code, out, errOut := executeAuthorization(t, configPath, fullArgs...)
			if code != exitUsage || out != "" || !strings.Contains(errOut, "--task-output-version") {
				t.Fatalf("%v: exit %d stdout %s stderr %s", fullArgs, code, out, errOut)
			}
		}
	}
	for _, flag := range []string{"--task-output-version=0", "--task-output-version=3", "--task-output-version=02", "--task-output-version="} {
		code, out, errOut := executeAuthorization(t, configPath, flag, "--json", "task", "list")
		if code != exitUsage || out != "" || !strings.Contains(errOut, "requires 1 (legacy) or 2 (faithful)") {
			t.Fatalf("invalid selector %s: %d %s %s", flag, code, out, errOut)
		}
	}
	code, out, errOut := executeAuthorization(t, configPath, "--task-output-version=2", "task", "view", "--help")
	if code != 0 || !strings.Contains(out, "task-output-version 2") || errOut != "" {
		t.Fatalf("help must not require config: %d %s %s", code, out, errOut)
	}
	if requests != 0 {
		t.Fatalf("rejected selection made %d requests", requests)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected selection created files: %v %v", entries, err)
	}
}

func TestTaskOutputSelectionPreservesSkillUsageContract(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := executeAuthorization(t, filepath.Join(dir, "config.json"),
		"--json", "--task-output-version=2", "--progress-jsonl", filepath.Join(dir, "progress.jsonl"),
		"skill", "install", "codex", "--path", filepath.Join(dir, "skills"))
	var failure struct {
		Code string `json:"code"`
	}
	if code != exitUsage || out != "" || json.Unmarshal([]byte(errOut), &failure) != nil || failure.Code != "SKILL_USAGE" {
		t.Fatalf("skill selection must preserve usage errors: exit=%d stdout=%s stderr=%s", code, out, errOut)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("skill selection created files: %v %v", entries, err)
	}
}

func TestTaskOutputBooleanHelpBeforeSideEffects(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer server.Close()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "uncreated", "config.json")
	commands := [][]string{
		{"task", "list"}, {"task", "ls"}, {"task", "view"}, {"task", "show"},
		{"task", "add"}, {"task", "update"}, {"add"}, {"inbox"}, {"inbox", "add"},
		{"today"}, {"upcoming"}, {"completed"}, {"filter", "show"},
		{"view"}, {"view", "https://app.todoist.com/app/settings"},
	}
	for _, command := range commands {
		for _, mode := range [][]string{nil, {"--json"}, {"--ndjson"}} {
			for _, flag := range []string{"--help=true", "-h=true", "--help=1", "-h=T"} {
				args := append([]string{"--base-url", server.URL, "--task-output-version=2", "--progress-jsonl", filepath.Join(dir, "progress.jsonl")}, mode...)
				args = append(args, command...)
				args = append(args, flag)
				code, out, errOut := executeAuthorization(t, configPath, args...)
				if code != 0 || !strings.Contains(out, "Usage:") || errOut != "" {
					t.Errorf("help must precede selection and config: %v: %d %s %s", args, code, out, errOut)
				}
			}
		}
	}
	for _, flag := range []string{"--help=invalid", "-h="} {
		code, out, errOut := executeAuthorization(t, configPath, "--progress-jsonl", filepath.Join(dir, "progress.jsonl"), "--json", "--task-output-version=2", "task", "view", flag)
		if code != exitUsage || out != "" || !strings.Contains(errOut, "invalid value for --help") {
			t.Errorf("invalid help must fail before side effects: %s: %d %s %s", flag, code, out, errOut)
		}
	}
	entries, err := os.ReadDir(dir)
	if requests != 0 || err != nil || len(entries) != 0 {
		t.Fatalf("help created side effects: requests=%d files=%v error=%v", requests, entries, err)
	}
}

func TestCompletedItemsPaginationPreservesTaskFacts(t *testing.T) {
	for _, path := range []string{"/tasks/completed/by_completion_date", "/tasks/completed/by_due_date"} {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			if r.URL.Path != path {
				t.Errorf("unexpected endpoint: %s", r.URL.Path)
			}
			if r.URL.Query().Get("cursor") == "" {
				fmt.Fprint(w, `{"items":[{"id":"first","duration":null}],"next_cursor":"second-page"}`)
			} else {
				fmt.Fprint(w, `{"items":[{"id":"second","child_order":0}]}`)
			}
		}))
		ctx := &Context{Token: "token", Client: api.NewClient(server.URL, "token", time.Second, authorization.Resolve(nil, "credentials", true)), Config: config.Config{TimeoutSeconds: 2}}
		tasks, cursor, err := fetchPaginated[api.Task](ctx, path, nil, false)
		if err != nil || len(tasks) != 1 || cursor != "second-page" || tasks[0].ResponseFact("duration").State != api.ResponseNull || requests != 1 {
			t.Fatalf("first page lost: %+v %s %v requests=%d", tasks, cursor, err, requests)
		}
		requests = 0
		tasks, cursor, err = fetchPaginated[api.Task](ctx, path, nil, true)
		if err != nil || len(tasks) != 2 || cursor != "" || tasks[1].FaithfulResource()["child_order"] != json.Number("0") || requests != 2 {
			t.Fatalf("all pages lost: %+v %s %v requests=%d", tasks, cursor, err, requests)
		}
		if path == "/tasks/completed/by_completion_date" {
			tasks, err = listCompletedTasks(ctx, "2026-09-01")
			if err != nil || len(tasks) != 2 {
				t.Fatalf("agent context lost completed items: %+v %v", tasks, err)
			}
		}
		server.Close()
	}
}

func TestTaskDetailFaithfulFacts(t *testing.T) {
	for _, tc := range []struct {
		name string
		want []string
	}{
		{"populated", []string{"Deadline: 2026-10-10", "Duration: 60 minutes", "Assignee ID: user-assignee", "Due language: en", "Recurrence: Yes", "Timezone: Europe/Berlin", "Sibling order: 0", "Reference item: Yes (title syntax)", "Uncompletable (returned): No"}},
		{"absent", []string{"Deadline: Not returned", "Duration: Not returned", "Assignee ID: Not returned", "Due language: Not returned", "Sibling order: Not returned"}},
		{"null", []string{"Deadline: No deadline", "Duration: No duration", "Assignee ID: Unassigned", "Assigner ID: None", "Due language: None (no due date)", "Sibling order: Unavailable (returned null)", "Reference item: Not returned (content unavailable)"}},
		{"false-zero-empty", []string{"Duration: 0 minutes", "Assignee ID: (empty returned value)", "Recurrence: None", "Timezone: None (date-only or floating time)", "Due language: (empty returned value)", "Sibling order: 0", "Reference item: No (title syntax)"}},
		{"malformed", []string{"Duration: Amount: Unavailable (invalid returned type); unit: minute", "Assignee ID: Unavailable (invalid returned type)", "Recurrence: Unavailable (invalid returned type)", "Timezone: Unavailable (invalid returned type)", "Deadline: 2026-10-10"}},
		{"integral-numbers", []string{"Duration: 60 minutes", "Sibling order: 0.0", "Day order: 9007199254740993.0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var task api.Task
			if err := json.Unmarshal(taskResourceFixture(t, tc.name), &task); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			ctx := &Context{Stdout: &out, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman, Config: config.Config{TableWidth: 160}}
			if err := writeTaskDetail(ctx, task, true); err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in %s", want, out.String())
				}
			}
		})
	}
	var task api.Task
	if err := json.Unmarshal([]byte(`{"due":{"date":"2026-10-01T09:00:00","timezone":"","is_recurring":null},"duration":{"unit":"minute"}}`), &task); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := &Context{Stdout: &out, Stderr: &bytes.Buffer{}, Mode: output.ModeHuman, Config: config.Config{TableWidth: 160}}
	if err := writeTaskDetail(ctx, task, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Timezone: (empty returned value)", "Recurrence: Unavailable (returned null)", "Duration: Amount: Not returned; unit: minute"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in %s", want, out.String())
		}
	}
}
