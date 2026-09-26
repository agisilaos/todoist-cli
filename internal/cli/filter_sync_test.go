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

func TestFilterCommandsUseSyncAndPreserveResults(t *testing.T) {
	var saved map[string]any
	var mutations []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" || r.Method != "POST" {
			t.Errorf("unsupported filter endpoint: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Form.Get("resource_types") != `["filters"]` || r.Form.Get("sync_token") != "*" {
			t.Errorf("missing filter resource request: %v", r.Form)
		}
		var commands []struct {
			Type   string         `json:"type"`
			UUID   string         `json:"uuid"`
			TempID string         `json:"temp_id"`
			Args   map[string]any `json:"args"`
		}
		if raw := r.Form.Get("commands"); raw != "" {
			if err := json.Unmarshal([]byte(raw), &commands); err != nil {
				t.Error(err)
			}
		}
		status := map[string]any{}
		mapping := map[string]string{}
		for _, cmd := range commands {
			mutations = append(mutations, cmd.Type)
			switch cmd.Type {
			case "filter_add":
				saved = cmd.Args
				saved["id"] = "601"
				mapping[cmd.TempID] = "601"
			case "filter_update":
				for k, v := range cmd.Args {
					saved[k] = v
				}
			case "filter_delete":
				saved = nil
			default:
				t.Errorf("unexpected command: %s", cmd.Type)
			}
			status[cmd.UUID] = "ok"
		}
		filters := []any{}
		if saved != nil {
			filters = append(filters, saved)
		}
		// Full sync can contain tombstones; consumers must omit them.
		filters = append(filters, map[string]any{"id": "deleted", "is_deleted": true})
		json.NewEncoder(w).Encode(map[string]any{"filters": filters, "sync_status": status, "temp_id_mapping": mapping})
	}))
	defer server.Close()
	t.Setenv("TODOIST_TOKEN", "synthetic-filter-token")
	t.Setenv("TODOIST_BASE_URL", server.URL)
	path := filepath.Join(t.TempDir(), "config.json")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"filter", "list", "--json"}, "[]"},
		{[]string{"filter", "add", "--name", "Focus", "--query", "today", "--favorite", "--ndjson"}, `"is_favorite":true`},
		{[]string{"filter", "list", "--ids-only"}, "601"},
		{[]string{"filter", "update", "601", "--name", "Renamed", "--unfavorite", "--ndjson"}, `"is_favorite":false`},
		{[]string{"filter", "delete", "601", "--yes", "--ndjson"}, `"status":"deleted"`},
		{[]string{"filter", "list", "--json"}, "[]"},
	} {
		code, out, errOut := executeAuthorization(t, path, tc.args...)
		if code != 0 || !strings.Contains(out, tc.want) || errOut != "" {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", tc.args, code, out, errOut)
		}
	}
	if fmt.Sprint(mutations) != "[filter_add filter_update filter_delete]" || saved != nil {
		t.Fatalf("incorrect resulting state or mutations: %v %v", saved, mutations)
	}
}
