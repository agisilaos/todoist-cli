package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/authorization"
)

func TestFilterMutationRequiresAcknowledgementAndReturnedIdentity(t *testing.T) {
	for _, scenario := range []string{"command-error", "missing-acknowledgement", "missing-id", "missing-result"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()
				var cmds []map[string]any
				if err := json.Unmarshal([]byte(r.Form.Get("commands")), &cmds); err != nil || len(cmds) != 1 {
					t.Errorf("bad commands: %v", err)
					http.Error(w, "bad commands", 400)
					return
				}
				uuid := cmds[0]["uuid"].(string)
				temp := cmds[0]["temp_id"].(string)
				response := map[string]any{"sync_status": map[string]any{uuid: "ok"}, "temp_id_mapping": map[string]string{temp: "601"}, "filters": []map[string]any{{"id": "601", "name": "Focus", "query": "today"}}}
				switch scenario {
				case "command-error":
					response["sync_status"] = map[string]any{uuid: map[string]any{"error": "Invalid query", "http_code": 400}}
				case "missing-acknowledgement":
					delete(response, "sync_status")
				case "missing-id":
					delete(response, "temp_id_mapping")
				case "missing-result":
					delete(response, "filters")
				}
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			client := api.NewClient(server.URL, "synthetic-filter-token", time.Second, authorization.Resolve(nil, "env", true))
			_, _, err := client.AddFilter(context.Background(), map[string]any{"name": "Focus", "query": "today"})
			if err == nil {
				t.Fatal("mutation reported success without confirmed result")
			}
			if scenario == "command-error" && !strings.Contains(err.Error(), "Invalid query") {
				t.Fatalf("lost provider error: %v", err)
			}
		})
	}
}
