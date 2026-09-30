package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTaskViewNDJSONMatchesJSON(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "synthetic-ndjson-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/tasks/task-long-exact-id" {
			t.Errorf("unexpected enrichment or mutation: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, detailTaskJSON)
	}))
	defer server.Close()
	configPath := filepath.Join(t.TempDir(), "config.json")
	var baseline map[string]any
	for _, command := range []string{"view", "show"} {
		for _, full := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/full=%t", command, full), func(t *testing.T) {
				args := []string{"--base-url", server.URL, "task", command, "id:task-long-exact-id", "--no-input"}
				if full {
					args = append(args, "--full")
				}
				var jsonTask map[string]any
				for _, mode := range []string{"--json", "--ndjson"} {
					code, out, stderr := executeAuthorization(t, configPath, append(args, mode)...)
					if code != 0 || stderr != "" {
						t.Fatalf("%s: exit=%d stderr=%s", mode, code, stderr)
					}
					if mode == "--ndjson" && (strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n")) {
						t.Fatalf("expected one newline-terminated NDJSON record: %q", out)
					}
					var task map[string]any
					if err := json.Unmarshal([]byte(out), &task); err != nil {
						t.Fatalf("%s: invalid task object: %v\n%s", mode, err, out)
					}
					if mode == "--json" {
						jsonTask = task
						if baseline == nil {
							baseline = task
							continue
						}
						if !reflect.DeepEqual(task, baseline) {
							t.Fatalf("alias or --full changed JSON payload: %s", out)
						}
						continue
					}
					if !reflect.DeepEqual(task, jsonTask) {
						t.Fatalf("NDJSON payload differs from JSON: %s", out)
					}
					if task["id"] != "task-long-exact-id" || task["priority"] != float64(3) || task["description"] != "First paragraph.\n\n- Confirm owner\n    literal code  spacing" {
						t.Fatalf("task fields changed: %s", out)
					}
				}
			})
		}
	}
}

func TestTaskViewNDJSONNotFound(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "synthetic-ndjson-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/tasks/missing-task" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	code, out, stderr := executeAuthorization(t, filepath.Join(t.TempDir(), "config.json"),
		"--base-url", server.URL, "task", "view", "id:missing-task", "--no-input", "--ndjson", "--full")
	if code != exitNotFound || out != "" || !strings.HasPrefix(stderr, "error: ") {
		t.Fatalf("unexpected not-found result: exit=%d stdout=%q stderr=%q", code, out, stderr)
	}
}
