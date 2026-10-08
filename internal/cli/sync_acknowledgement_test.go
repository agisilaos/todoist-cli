package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSyncMutationsRequireCommandAcknowledgement(t *testing.T) {
	for _, command := range [][]string{
		{"settings", "update", "--timezone", "UTC", "--reminder-email", "on"},
		{"stats", "goals", "--daily", "7"},
		{"stats", "vacation", "--on"},
		{"notification", "read", "id:n1"},
		{"notification", "unread", "id:n1"},
		{"notification", "read", "--all", "--yes"},
		{"notification", "accept", "id:n1"},
		{"notification", "reject", "id:n1"},
	} {
		for _, outcome := range []string{"rejected-first", "rejected-last", "missing", "accepted"} {
			t.Run(strings.Join(command, "_")+"/"+outcome, func(t *testing.T) {
				var invitationMarkedRead atomic.Bool
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if err := r.ParseForm(); err != nil {
						t.Error(err)
						return
					}
					var commands []struct{ UUID, Type string }
					if r.Form.Get("commands") == "" {
						_, _ = w.Write([]byte(`{"live_notifications":[{"id":"n1","notification_type":"share_invitation_sent","invitation_id":"1","invitation_secret":"synthetic","is_unread":true}]}`))
						return
					}
					if err := json.Unmarshal([]byte(r.Form.Get("commands")), &commands); err != nil || len(commands) == 0 {
						t.Errorf("invalid commands: %v", err)
						return
					}
					statuses := map[string]any{}
					for i, cmd := range commands {
						if cmd.Type == "live_notifications_mark_read" && (command[1] == "accept" || command[1] == "reject") {
							invitationMarkedRead.Store(true)
						}
						if outcome == "missing" {
							continue
						}
						statuses[cmd.UUID] = "ok"
						if outcome == "rejected-first" && i == 0 || outcome == "rejected-last" && i == len(commands)-1 {
							statuses[cmd.UUID] = map[string]any{"error": "synthetic rejection", "http_code": 403}
						}
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"sync_status": statuses})
				}))
				defer server.Close()
				var stdout, stderr bytes.Buffer
				args := append([]string{"--config", filepath.Join(t.TempDir(), "config.json"), "--base-url", server.URL, "--json"}, command...)
				code := executeTestWithEnvironment(args, &stdout, &stderr, Environment{Getenv: func(key string) string {
					if key == "TODOIST_TOKEN" {
						return "synthetic"
					}
					return ""
				}})
				if outcome == "accepted" {
					if code != exitOK || stdout.Len() == 0 {
						t.Fatalf("acknowledged mutation failed: exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
					}
					return
				}
				if code == exitOK || stdout.Len() != 0 || stderr.Len() == 0 {
					t.Errorf("unacknowledged mutation reported success: exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
				}
				if invitationMarkedRead.Load() {
					t.Error("failed invitation response still marked the notification read")
				}
			})
		}
	}
}

func successfulSyncResponse(t *testing.T, commands string) []byte {
	t.Helper()
	var entries []struct{ UUID string }
	if err := json.Unmarshal([]byte(commands), &entries); err != nil || len(entries) == 0 {
		t.Fatalf("invalid fixture commands: %v", err)
	}
	statuses := map[string]string{}
	for _, entry := range entries {
		statuses[entry.UUID] = "ok"
	}
	data, err := json.Marshal(map[string]any{"sync_status": statuses})
	if err != nil {
		t.Fatal(err)
	}
	return data
}
