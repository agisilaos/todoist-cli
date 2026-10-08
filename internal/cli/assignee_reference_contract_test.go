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

func TestTaskAssigneePreservesEmailAndRejectsAmbiguousNames(t *testing.T) {
	for _, tc := range []struct {
		name, collaborators, ref, wantID string
		wantCode                         int
	}{
		{"email before display name", `[{"id":"alice","name":"bob@example.test","email":"alice@example.test"},{"id":"bob","name":"Bob","email":"bob@example.test"}]`, "bob@example.test", "bob", exitOK},
		{"duplicate exact names", `[{"id":"alice","name":"Alex","email":"alice@example.test"},{"id":"bob","name":"Alex","email":"bob@example.test"}]`, "Alex", "", exitUsage},
		{"unique exact name before partial", `[{"id":"alice","name":"Alexandra","email":"alice@example.test"},{"id":"bob","name":"Alex","email":"bob@example.test"}]`, "Alex", "bob", exitOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/projects/p/collaborators" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				fmt.Fprintf(w, `{"results":%s,"next_cursor":null}`, tc.collaborators)
			}))
			defer server.Close()
			env := Environment{Getenv: func(key string) string {
				switch key {
				case "TODOIST_TOKEN":
					return "synthetic-token"
				case "TODOIST_BASE_URL":
					return server.URL
				}
				return ""
			}}
			code, out, errOut := executeAuthorizationWithEnvironment(t, filepath.Join(t.TempDir(), "config.json"), env,
				"task", "add", "Task", "--project", "id:p", "--assignee", tc.ref, "--dry-run", "--json", "--no-input")
			if code != tc.wantCode {
				t.Fatalf("assignee resolution exit=%d, want %d; stdout=%s stderr=%s", code, tc.wantCode, out, errOut)
			}
			if tc.wantCode == exitUsage {
				if out != "" || !strings.Contains(errOut, `"ambiguous_match"`) || !strings.Contains(errOut, "alice@example.test") || !strings.Contains(errOut, "bob@example.test") {
					t.Fatalf("expected both collaborators in ambiguity error: stdout=%s stderr=%s", out, errOut)
				}
				return
			}
			var result struct {
				Payload struct {
					AssigneeID string `json:"assignee_id"`
				} `json:"payload"`
			}
			if json.Unmarshal([]byte(out), &result) != nil || result.Payload.AssigneeID != tc.wantID {
				t.Fatalf("expected assignee %s: stdout=%s stderr=%s", tc.wantID, out, errOut)
			}
		})
	}
}
