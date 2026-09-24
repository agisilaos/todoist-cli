package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContractIDsOnlyParsingAndPagination(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "test")
	t.Setenv("TODOIST_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" {
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("cursor") == "page2" {
			fmt.Fprint(w, `{"results":[{"id":"second"}],"next_cursor":""}`)
		} else {
			fmt.Fprint(w, `{"results":[{"id":"first","content":"not output"}],"next_cursor":"page2"}`)
		}
	}))
	defer server.Close()
	for _, tc := range []struct {
		args         []string
		want, notice string
	}{
		{[]string{"--ids-only", "task", "list", "--all-projects"}, "first\n", "More available. Use --cursor \"page2\"\n"},
		{[]string{"task", "ls", "--all-projects", "--ids-only"}, "first\n", "More available. Use --cursor \"page2\"\n"},
		{[]string{"task", "--ids-only", "list", "--all-projects", "--all"}, "first\nsecond\n", ""},
		{[]string{"task", "list", "--ids-only", "--all-projects", "--cursor", "page2"}, "second\n", ""},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := append([]string{"--base-url", server.URL}, tc.args...)
			code := Execute(args, &stdout, &stderr)
			if code != 0 || stdout.String() != tc.want || stderr.String() != tc.notice {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestContractIDsOnlySupportedCommands(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "test")
	t.Setenv("TODOIST_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tasks", "/tasks/filter", "/tasks/completed/by_completion_date":
			fmt.Fprintf(w, `{"results":[{"id":"task-id","due":{"date":%q}}]}`, time.Now().UTC().Format("2006-01-02"))
		case "/tasks/task-id":
			fmt.Fprint(w, `{"id":"task-id"}`)
		case "/projects":
			fmt.Fprint(w, `{"results":[{"id":"project-id","is_inbox_project":true}]}`)
		case "/projects/project-id/collaborators":
			fmt.Fprint(w, `{"results":[{"id":"user-id"}]}`)
		case "/sections":
			fmt.Fprint(w, `{"results":[{"id":"section-id"}]}`)
		case "/labels":
			fmt.Fprint(w, `{"results":[{"id":"label-id"}]}`)
		case "/comments":
			fmt.Fprint(w, `{"results":[{"id":"comment-id"}]}`)
		case "/activities":
			fmt.Fprint(w, `{"results":[{"id":"event-id","object_id":"task-id"}]}`)
		case "/filters":
			fmt.Fprint(w, `[{"id":"filter-id","name":"Test","query":"today"}]`)
		case "/sync":
			fmt.Fprint(w, `{"workspaces":[{"id":"workspace-id"}],"reminders":[{"id":"reminder-id","item_id":"task-id"}],"live_notifications":[{"id":"notification-id","notification_type":"item_assigned","is_unread":true}]}`)
		default:
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	for _, tc := range []struct {
		command []string
		id      string
	}{
		{[]string{"task", "list", "--all-projects"}, "task-id"},
		{[]string{"project", "list"}, "project-id"},
		{[]string{"project", "collaborators", "id:project-id"}, "user-id"},
		{[]string{"section", "list"}, "section-id"}, {[]string{"label", "list"}, "label-id"},
		{[]string{"comment", "list", "--task", "task-id"}, "comment-id"},
		{[]string{"filter", "list"}, "filter-id"}, {[]string{"workspace", "list"}, "workspace-id"},
		{[]string{"reminder", "list", "id:task-id"}, "reminder-id"},
		{[]string{"notification", "list"}, "notification-id"}, {[]string{"activity"}, "event-id"},
		{[]string{"today"}, "task-id"}, {[]string{"inbox"}, "task-id"},
		{[]string{"upcoming"}, "task-id"}, {[]string{"completed"}, "task-id"},
		{[]string{"filter", "show", "id:filter-id"}, "task-id"},
	} {
		commands := [][]string{tc.command}
		if len(tc.command) > 1 && tc.command[1] == "list" {
			alias := append([]string{}, tc.command...)
			alias[1] = "ls"
			commands = append(commands, alias)
		}
		for _, command := range commands {
			t.Run(strings.Join(command, " "), func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				args := append([]string{"--base-url", server.URL, "--ids-only"}, command...)
				code := Execute(args, &stdout, &stderr)
				if code != 0 || stdout.String() != tc.id+"\n" || stderr.Len() != 0 {
					t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
				}
			})
		}
	}
}

func TestContractIDsOnlyUsageErrors(t *testing.T) {
	t.Setenv("TODOIST_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	commands := [][]string{
		{"task", "add", "--content", "no mutation"}, {"inbox", "add", "no mutation"},
		{"task", "view", "id:123"}, {"project", "view", "id:123"}, {"view", "https://app.todoist.com/app/today"},
		{"settings", "themes"}, {"schema"}, {"stats"}, {"unknown"}, {"project", "browse"}, {"completion", "install", "bash"},
	}
	for _, mode := range []string{"--json", "--plain", "--ndjson"} {
		commands = append(commands, []string{"task", "list", mode}, []string{mode, "task", "list"})
	}
	for _, command := range commands {
		for _, first := range []bool{true, false} {
			args := append([]string{"--ids-only"}, command...)
			if !first {
				args = append(append([]string{}, command...), "--ids-only")
			}
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				code := Execute(args, &stdout, &stderr)
				if code != 2 || stdout.Len() != 0 {
					t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
				}
				assertIDsError(t, stderr.Bytes())
			})
		}
	}
}

func assertIDsError(t *testing.T, data []byte) {
	t.Helper()
	var envelope struct {
		Error string         `json:"error"`
		Meta  map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || envelope.Error == "" || envelope.Meta == nil {
		t.Fatalf("expected JSON error envelope, got %q (err=%v)", data, err)
	}
}

func TestContractIDsOnlyDiscoverability(t *testing.T) {
	t.Setenv("TODOIST_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	for _, command := range []string{"", "task", "project", "section", "label", "comment", "filter", "workspace", "reminder", "notification", "activity", "today", "upcoming", "completed", "inbox"} {
		var stdout, stderr bytes.Buffer
		args := []string{"--help"}
		if command != "" {
			args = append([]string{command}, args...)
		}
		if code := Execute(args, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "--ids-only") {
			t.Errorf("%s help missing --ids-only: code=%d stdout=%q stderr=%q", command, code, stdout.String(), stderr.String())
		}
	}
	for _, shell := range []string{"bash", "zsh", "fish"} {
		var stdout, stderr bytes.Buffer
		if code := Execute([]string{"completion", shell}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "ids-only") {
			t.Errorf("%s completion missing ids-only: code=%d", shell, code)
		}
	}
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"schema", "--name", "ids_only"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("schema: code=%d stderr=%q", code, stderr.String())
	}
	var entries []struct {
		Schema map[string]any `json:"schema"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Schema["kind"] != "wire_format" || entries[0].Schema["encoding"] != "raw_id_lines" {
		t.Fatalf("expected a wire-format descriptor: %s", stdout.String())
	}
	if _, ok := entries[0].Schema["type"]; ok {
		t.Fatal("wire format must not masquerade as a JSON Schema")
	}
}

func TestContractIDsOnlyHelpAndVersion(t *testing.T) {
	t.Setenv("TODOIST_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	for _, args := range [][]string{
		{"--ids-only"}, {"--ids-only", "--help"}, {"task", "add", "--ids-only", "--help"},
		{"help", "task", "--ids-only"}, {"task", "help", "--ids-only"},
		{"auth", "logout", "--help", "--ids-only"},
		{"--ids-only", "--json", "--version"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Execute(args, &stdout, &stderr); code != 0 || stdout.Len() == 0 || stderr.Len() != 0 {
			t.Fatalf("args=%v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := Execute([]string{"task", "list", "--ids-only", "--plain", "--help"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
		t.Fatalf("conflict must precede help: code=%d stdout=%q", code, stdout.String())
	}
	assertIDsError(t, stderr.Bytes())
}

func TestContractIDsOnlyRejectsHelpAsData(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "test")
	t.Setenv("TODOIST_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unsupported command reached API: %s %s", r.Method, r.URL)
		http.Error(w, "unexpected request", http.StatusBadRequest)
	}))
	defer server.Close()
	for _, command := range [][]string{{"add", "help"}, {"planner", "help"}, {"view", "help"}} {
		var stdout, stderr bytes.Buffer
		args := append([]string{"--base-url", server.URL, "--ids-only"}, command...)
		if code := Execute(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("args=%v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
		assertIDsError(t, stderr.Bytes())
	}
}

func TestContractIDsOnlyGlobalDelimiter(t *testing.T) {
	// The existing global parser stops at --; it also consumes recognized flags
	// even when they appear after an ordinary positional argument.
	for _, args := range [][]string{
		{"--", "--ids-only", "task", "list"}, {"task", "list", "--", "--ids-only"},
		{"task", "list", "--ids-only=true"},
	} {
		opts, rest, err := parseGlobalFlags(args, &bytes.Buffer{})
		if err != nil {
			t.Fatal(err)
		}
		if opts.IDsOnly || len(rest) == 0 {
			t.Fatal("unexpected parsed arguments")
		}
		if !strings.Contains(strings.Join(rest, " "), "--ids-only") {
			t.Fatalf("flag was consumed: %v", rest)
		}
	}
}

func TestContractGlobalParsingPreservesFirstError(t *testing.T) {
	for _, missing := range []string{"--timeout", "--config", "--profile", "--base-url"} {
		var stdout, stderr bytes.Buffer
		code := Execute([]string{"--timeout=bad", missing}, &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "invalid value for --timeout: bad\n") {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	}
}

func TestContractIDsOnlyErrors(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "test")
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("TODOIST_CONFIG", configPath)
	for _, args := range [][]string{
		{"--ids-only", "--timeout=bad", "task", "list"},
		{"--timeout=bad", "task", "list", "--ids-only"},
		{"--timeout", "bad", "task", "list", "--ids-only"},
		{"--ids-only", "--timeout"}, {"--ids-only", "--config"},
		{"--ids-only", "--quiet", "--verbose", "task", "list"},
		{"comment", "list", "--ids-only"}, {"upcoming", "1", "2", "--ids-only"},
		{"task", "list", "--bogus", "--ids-only"},
		{"unknown", "--help", "--ids-only"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Execute(args, &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			assertIDsError(t, stderr.Bytes())
		})
	}
	if err := os.WriteFile(configPath, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Execute([]string{"task", "list", "--ids-only"}, &stdout, &stderr); code != 1 || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout.String())
	}
	assertIDsError(t, stderr.Bytes())
}

func TestContractIDsOnlyRuntimeErrors(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "test")
	t.Setenv("TODOIST_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	for _, tc := range []struct {
		status, want int
		body         string
	}{
		{200, 1, `{"results":[{"id":"valid"},{"id":""}]}`},
		{401, 3, "unauthorized"}, {404, 4, "not found"}, {409, 5, "conflict"},
	} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Request-Id", "req-test")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			var stdout, stderr bytes.Buffer
			code := Execute([]string{"task", "list", "--all-projects", "--base-url", server.URL, "--ids-only", "--quiet-json"}, &stdout, &stderr)
			if code != tc.want || stdout.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			assertIDsError(t, stderr.Bytes())
			if strings.Count(stderr.String(), "\n") != 1 {
				t.Fatalf("not compact: %q", stderr.String())
			}
		})
	}
	t.Setenv("TODOIST_TOKEN", "")
	for _, command := range [][]string{{"task", "list"}, {"today"}, {"inbox"}} {
		var stdout, stderr bytes.Buffer
		code := Execute(append(command, "--ids-only"), &stdout, &stderr)
		if code != 3 || stdout.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		assertIDsError(t, stderr.Bytes())
	}
}
