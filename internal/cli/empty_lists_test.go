package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmptyPaginatedResourcesAreJSONArrays(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"results":[],"next_cursor":null}`) }))
	defer server.Close()
	t.Setenv("TODOIST_TOKEN", "synthetic-empty-list-token")
	t.Setenv("TODOIST_BASE_URL", server.URL)
	for _, args := range [][]string{{"project", "list"}, {"section", "list"}, {"label", "list"}, {"comment", "list", "--task", "101"}, {"project", "collaborators", "201"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			code, out, errOut := executeAuthorization(t, filepath.Join(t.TempDir(), "config.json"), append(args, "--json")...)
			if code != 0 || out != "[]\n" {
				t.Fatalf("empty list: exit %d, stdout %q, stderr %q", code, out, errOut)
			}
		})
	}
}
