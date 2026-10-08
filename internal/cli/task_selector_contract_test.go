package cli

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskDeleteAndReopenRejectConflictingSelectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("conflicting task selectors accessed API: %s %s", r.Method, r.URL.Path)
		http.Error(w, "unexpected request", http.StatusBadRequest)
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
	for _, command := range []string{"delete", "reopen"} {
		t.Run(command, func(t *testing.T) {
			args := []string{"task", command, "id:exact", "--id", "other", "--dry-run", "--json", "--no-input"}
			if command == "delete" {
				args = append(args, "--yes")
			}
			code, out, errOut := executeAuthorizationWithEnvironment(t, filepath.Join(t.TempDir(), "config.json"), env, args...)
			if code != exitUsage || out != "" || !strings.Contains(errOut, "--id cannot be combined with a positional task reference") {
				t.Fatalf("conflicting selectors must fail before any operation: exit=%d stdout=%s stderr=%s", code, out, errOut)
			}
		})
	}
}
