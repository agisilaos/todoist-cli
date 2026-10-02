package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestNDJSONCommandResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			fmt.Fprint(w, `{"results":[],"next_cursor":null}`)
		case "/tasks/101/close", "/tasks/101/reopen":
			w.WriteHeader(http.StatusNoContent)
		case "/sync":
			r.ParseForm()
			var commands []struct {
				UUID string `json:"uuid"`
			}
			json.Unmarshal([]byte(r.Form.Get("commands")), &commands)
			status := map[string]string{}
			for _, command := range commands {
				status[command.UUID] = "ok"
			}
			json.NewEncoder(w).Encode(map[string]any{"sync_status": status, "live_notifications": []map[string]any{{"id": "901", "notification_type": "item_assigned", "is_unread": true}}})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("TODOIST_TOKEN", "synthetic-ndjson-token")
	t.Setenv("TODOIST_BASE_URL", server.URL)
	for _, tc := range []struct {
		args []string
		key  string
	}{
		{[]string{"task", "complete", "--id", "101"}, "status"},
		{[]string{"task", "reopen", "--id", "101"}, "status"},
		{[]string{"reminder", "delete", "--id", "801", "--yes"}, "status"},
		{[]string{"notification", "read", "901"}, "status"},
		{[]string{"notification", "view", "901"}, "id"},
		{[]string{"task", "add", "--content", "Dry run", "--dry-run"}, "dry_run"},
		{[]string{"doctor"}, "checks"},
		{[]string{"planner"}, "planner_cmd"},
		{[]string{"planner", "--set", "--cmd", "example-planner"}, "planner_cmd"},
		{[]string{"agent", "status"}, "authorization"},
	} {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			code, out, errOut := executeAuthorization(t, filepath.Join(t.TempDir(), "config.json"), append(tc.args, "--ndjson")...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, errOut)
			}
			if strings.Count(out, "\n") != 1 {
				t.Fatalf("expected one NDJSON record: %q", out)
			}
			var record map[string]any
			if err := json.Unmarshal([]byte(out), &record); err != nil {
				t.Fatalf("invalid NDJSON: %s: %v", out, err)
			}
			if _, ok := record[tc.key]; !ok {
				t.Fatalf("missing %s in %s", tc.key, out)
			}
		})
	}
}
