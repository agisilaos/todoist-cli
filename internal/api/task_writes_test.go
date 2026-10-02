package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/authorization"
)

func TestNativeTaskCommandAcknowledgements(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		outcome      TaskWriteState
	}{
		{"ok", `"ok"`, "accepted"}, {"error", `{"error":"forbidden","http_code":403}`, "rejected"},
		{"code", `{"error_code":42}`, "rejected"}, {"empty object", `{}`, "uncertain"},
		{"null", `null`, "uncertain"}, {"unknown string", `"done"`, "uncertain"},
		{"wrong code type", `{"error":"forbidden","http_code":"403"}`, "uncertain"},
		{"contradictory code", `{"error":"forbidden","http_code":200}`, "uncertain"},
		{"duplicate error", `{"error":"no","error":"yes"}`, "uncertain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/sync" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.Header.Get("X-Request-Id") == "" || r.Header.Get("Authorization") != "Bearer synthetic" {
					t.Errorf("request: %#v", r)
				}
				_ = r.ParseForm()
				if r.Form.Get("resource_types") != `[]` || r.Form.Get("sync_token") != "*" {
					t.Error(r.Form)
				}
				var commands []struct {
					Type, UUID string
					Args       map[string]any
				}
				if err := json.Unmarshal([]byte(r.Form.Get("commands")), &commands); err != nil || len(commands) != 1 {
					t.Fatal(commands, err)
				}
				cmd := commands[0]
				if cmd.Type != "item_update" || cmd.UUID == "" || len(cmd.Args) != 2 || cmd.Args["id"] != "opaque" {
					t.Error(cmd)
				}
				if v, exists := cmd.Args["due"]; !exists || v != nil {
					t.Error("missing explicit due:null")
				}
				fmt.Fprintf(w, `{"sync_status":{%q:%s},"items":"optional malformed data"}`, cmd.UUID, tc.status)
			}))
			defer server.Close()
			client := NewClient(server.URL, "synthetic", time.Second, authorization.Resolve(nil, "env", true))
			raw, id, err := client.TaskCommand(context.Background(), "item_update", map[string]any{"id": "opaque", "due": nil})
			if TaskWriteOutcome(err) != tc.outcome || calls != 1 || id == "" || len(raw) != 0 {
				t.Fatalf("outcome=%s calls=%d id=%q raw=%s err=%v", TaskWriteOutcome(err), calls, id, raw, err)
			}
		})
	}
}

func TestNativeTaskCommandRequiredEvidenceAndOptionalResource(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response func(string) string
		outcome  TaskWriteState
		resource bool
	}{
		{"missing", func(u string) string { return `{}` }, "uncertain", false},
		{"wrong uuid", func(u string) string { return `{"sync_status":{"other":"ok"}}` }, "uncertain", false},
		{"duplicate status", func(u string) string {
			return fmt.Sprintf(`{"sync_status":{%q:{"error":"no"}},"sync_status":{%q:"ok"}}`, u, u)
		}, "uncertain", false},
		{"duplicate uuid", func(u string) string { return fmt.Sprintf(`{"sync_status":{%q:{"error":"no"},%q:"ok"}}`, u, u) }, "uncertain", false},
		{"valid item", func(u string) string {
			return fmt.Sprintf(`{"sync_status":{%q:"ok"},"items":[{"id":"opaque","checked":false}]}`, u)
		}, "accepted", true},
		{"duplicate optional key", func(u string) string {
			return fmt.Sprintf(`{"sync_status":{%q:"ok"},"items":[{"id":"opaque"}],"items":[{"id":"opaque","checked":true}]}`, u)
		}, "accepted", false},
		{"duplicate item", func(u string) string {
			return fmt.Sprintf(`{"sync_status":{%q:"ok"},"items":[{"id":"opaque"},{"id":"opaque"}]}`, u)
		}, "accepted", false},
		{"malformed", func(u string) string { return `{broken` }, "uncertain", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				var c []struct{ UUID string }
				_ = json.Unmarshal([]byte(r.Form.Get("commands")), &c)
				fmt.Fprint(w, tc.response(c[0].UUID))
			}))
			defer server.Close()
			raw, _, err := NewClient(server.URL, "synthetic", time.Second, authorization.Resolve(nil, "env", true)).TaskCommand(context.Background(), "item_complete", map[string]any{"id": "opaque"})
			if TaskWriteOutcome(err) != tc.outcome || (len(raw) > 0) != tc.resource {
				t.Fatalf("%s %s %v", TaskWriteOutcome(err), raw, err)
			}
		})
	}
}

func TestTaskWritesDispatchOnceAndNeverFollowRedirects(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308, 408, 429, 500, 503} {
		for _, method := range []string{"POST", "DELETE", "Sync"} {
			t.Run(fmt.Sprintf("%s/%d", method, status), func(t *testing.T) {
				source, target := 0, 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/redirected" {
						target++
						fmt.Fprint(w, `{}`)
						return
					}
					source++
					w.Header().Set("Location", "/redirected")
					w.WriteHeader(status)
				}))
				defer server.Close()
				c := NewClient(server.URL, "synthetic", time.Second, authorization.Resolve(nil, "env", true))
				var err error
				if method == "Sync" {
					_, _, err = c.TaskCommand(context.Background(), "item_complete", map[string]any{"id": "opaque"})
				} else if method == "DELETE" {
					_, err = c.Delete(context.Background(), "/tasks/opaque", nil)
				} else {
					_, err = c.Post(context.Background(), "/tasks/opaque", nil, map[string]any{"description": ""}, nil, true)
				}
				want := TaskWriteUncertain
				if status == 429 {
					want = TaskWriteRejected
				}
				if source != 1 || target != 0 || TaskWriteOutcome(err) != want {
					t.Fatalf("source=%d target=%d outcome=%s err=%v", source, target, TaskWriteOutcome(err), err)
				}
			})
		}
	}
}
func TestTaskWriteTransportAndAcceptedDecodeFailure(t *testing.T) {
	c := NewClient("https://example.test", "synthetic", time.Second, authorization.Resolve(nil, "env", true))
	calls := 0
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) { calls++; return nil, fmt.Errorf("transport lost") })
	_, err := c.Post(context.Background(), "/tasks/opaque", nil, map[string]any{"content": "A"}, nil, true)
	if calls != 1 || TaskWriteOutcome(err) != "uncertain" {
		t.Fatal(calls, err)
	}
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{broken`))}, nil
	})
	var response map[string]any
	_, err = c.Post(context.Background(), "/tasks/opaque", nil, map[string]any{"content": "A"}, &response, true)
	if TaskWriteOutcome(err) != "accepted" || calls != 2 {
		t.Fatal(calls, err)
	}
}

func TestGenericJSONSyncTaskWriteAlsoDispatchesOnce(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(503) }))
	defer server.Close()
	c := NewClient(server.URL, "synthetic", time.Second, authorization.Resolve(nil, "env", true))
	_, err := c.Post(context.Background(), "/sync", nil, map[string]any{"commands": []any{map[string]any{"type": "item_update", "uuid": "command", "args": map[string]any{"id": "t", "due": nil}}}}, nil, true)
	if calls != 1 || TaskWriteOutcome(err) != "uncertain" {
		t.Fatal(calls, err)
	}
}
