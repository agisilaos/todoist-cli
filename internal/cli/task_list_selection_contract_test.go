package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilteredTaskListRejectsIgnoredSelectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unsupported selector combination accessed API: %s %s", r.Method, r.URL.Path)
		fmt.Fprint(w, `{"results":[{"id":"home","content":"Home task","project_id":"home"}],"next_cursor":null}`)
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
	for _, filterFlag := range []string{"--filter", "--preset"} {
		for _, selector := range []string{"--project", "--section", "--parent", "--label", "--id"} {
			t.Run(filterFlag+"/"+selector, func(t *testing.T) {
				code, out, errOut := executeAuthorizationWithEnvironment(t, filepath.Join(t.TempDir(), "config.json"), env,
					"task", "list", filterFlag, "today", selector, "work", "--all", "--sort", "priority", "--json", "--no-input")
				if code != exitUsage || out != "" || !strings.Contains(errOut, "cannot be combined") || !strings.Contains(errOut, selector) {
					t.Fatalf("unsupported selection must not return broader tasks: exit=%d stdout=%s stderr=%s", code, out, errOut)
				}
			})
		}
	}
}
