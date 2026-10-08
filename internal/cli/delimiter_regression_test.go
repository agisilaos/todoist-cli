package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskAddPreservesOperandsAfterDelimiter(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	env := Environment{Getenv: func(key string) string {
		if key == "TODOIST_TOKEN" {
			return "synthetic-delimiter-token"
		}
		return ""
	}}
	for _, tc := range []struct {
		name, content string
		args          []string
		priority      float64
	}{
		{"option-shaped content", "--priority 4 hello", []string{"--", "--priority", "4", "hello"}, 0},
		{"help-shaped content", "--help", []string{"--", "--help"}, 0},
		{"flags before delimiter", "--priority 4 hello", []string{"--priority", "p2", "--", "--priority", "4", "hello"}, 3},
		{"earlier positional", "literal --priority 4", []string{"literal", "--", "--priority", "4"}, 0},
		{"literal delimiter", "--", []string{"--", "--"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--config", filepath.Join(dir, "config.json"), "--base-url", "http://127.0.0.1:1", "--json", "--dry-run", "task", "add"}, tc.args...)
			var out, errOut bytes.Buffer
			code := executeTestWithEnvironment(args, &out, &errOut, env)
			if code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, errOut.String())
			}
			var preview struct {
				Payload map[string]any `json:"payload"`
			}
			if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
				t.Fatalf("expected JSON task preview, got %s", out.String())
			}
			if preview.Payload["content"] != tc.content {
				t.Errorf("content=%q, want %q", preview.Payload["content"], tc.content)
			}
			priority, exists := preview.Payload["priority"]
			if tc.priority == 0 && exists || tc.priority != 0 && priority != tc.priority {
				t.Errorf("priority=%v (present=%t), want %v (zero means absent)", priority, exists, tc.priority)
			}
		})
	}
}

func TestDelimiterBeforeSubcommandKeepsCommandRouting(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/tasks" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"results":[{"id":"task-one","content":"One task"}],"next_cursor":null}`)
	}))
	defer server.Close()
	env := Environment{Getenv: func(key string) string {
		if key == "TODOIST_TOKEN" {
			return "synthetic-delimiter-token"
		}
		return ""
	}}
	for _, tc := range []struct {
		name, want string
		args       []string
	}{
		{"completion shell", "# todoist completion", []string{"completion", "--", "bash"}},
		{"task list", `"content": "One task"`, []string{"--json", "task", "--", "list", "--all-projects"}},
		{"task list ID output", "task-one\n", []string{"--ids-only", "task", "--", "ls", "--all-projects"}},
		{"task resource selection", `"content": "One task"`, []string{"--json", "--task-output-version", "2", "task", "--", "list", "--all-projects"}},
		{"nested schedule", "0 9 * * 1 ", []string{"agent", "schedule", "--", "print", "--cron", "--weekly", "mon 09:00", "--planner", "cat", "--bin", "todoist"}},
		{"help lookup", "todoist task list [flags]", []string{"--help", "task", "--", "list"}},
		{"help command", "todoist task list [flags]", []string{"help", "--", "task", "list"}},
		{"group then operand delimiter", `"content": "--priority 4 hello"`, []string{"--dry-run", "--json", "task", "--", "add", "--", "--priority", "4", "hello"}},
		{"leading global delimiter", "# todoist completion", []string{"--", "completion", "bash"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--config", filepath.Join(dir, "config.json"), "--base-url", server.URL, "--no-input"}, tc.args...)
			var out, errOut bytes.Buffer
			code := executeTestWithEnvironment(args, &out, &errOut, env)
			if code != exitOK || !strings.Contains(out.String(), tc.want) || errOut.Len() != 0 {
				t.Fatalf("group delimiter must preserve routing: exit=%d stdout=%q stderr=%q; want %q", code, out.String(), errOut.String(), tc.want)
			}
		})
	}
}
