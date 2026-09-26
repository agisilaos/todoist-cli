package cli

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestMutationHelpRequiresNoTargetOrAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("help made an API request: %s", r.URL.Path)
		http.Error(w, "unexpected", 500)
	}))
	defer server.Close()
	t.Setenv("TODOIST_TOKEN", "synthetic-help-token")
	t.Setenv("TODOIST_BASE_URL", server.URL)
	for _, command := range []string{"task reopen", "project archive", "project unarchive", "project delete", "section delete", "label delete", "comment delete"} {
		for _, flag := range []string{"--help", "-h"} {
			t.Run(command+flag, func(t *testing.T) {
				args := append(strings.Fields(command), flag)
				code, out, errOut := executeAuthorization(t, filepath.Join(t.TempDir(), "config.json"), args...)
				if code != 0 || errOut != "" || !strings.Contains(out, "todoist "+command) {
					t.Fatalf("help exit=%d stdout=%q stderr=%q", code, out, errOut)
				}
			})
		}
	}
}
