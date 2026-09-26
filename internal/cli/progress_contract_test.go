package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestUnavailableProgressLogPreventsDispatch(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	t.Setenv("TODOIST_TOKEN", "synthetic-progress-token")
	t.Setenv("TODOIST_BASE_URL", server.URL)
	dir := t.TempDir()
	code, out, errOut := executeAuthorization(t, filepath.Join(dir, "config.json"), "task", "add", "--content", "Must not be created", "--progress-jsonl="+filepath.Join(dir, "missing", "progress.jsonl"), "--json")
	if code != 1 || out != "" || requests.Load() != 0 {
		t.Fatal("unavailable log did not fail before dispatch")
	}
	var payload map[string]any
	if json.Unmarshal([]byte(errOut), &payload) != nil || !strings.Contains(errOut, "open progress log") {
		t.Fatal("missing structured progress error")
	}
}
