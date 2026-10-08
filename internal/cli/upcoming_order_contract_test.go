package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestUpcomingDefaultOrdersCurrentDueFormats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/tasks" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"results":[
{"id":"late","content":"Late timed","due":{"date":"2026-10-11T20:00:00Z","timezone":"UTC"}},
{"id":"floating","content":"Floating timed","due":{"date":"2026-10-10T12:00:00","timezone":null}},
{"id":"early","content":"Early timed","due":{"date":"2026-10-09T09:00:00Z","timezone":"UTC"}},
{"id":"date","content":"Today","due":{"date":"2026-10-08","timezone":null}},
{"id":"legacy","content":"Legacy timestamp","due":{"date":"2026-10-12","datetime":"2026-10-12T10:00:00Z"}}
],"next_cursor":null}`)
	}))
	defer server.Close()
	env := Environment{
		Now: func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) },
		Getenv: func(key string) string {
			switch key {
			case "TODOIST_TOKEN":
				return "synthetic-token"
			case "TODOIST_BASE_URL":
				return server.URL
			}
			return ""
		},
	}
	code, out, errOut := executeAuthorizationWithEnvironment(t, filepath.Join(t.TempDir(), "config.json"), env,
		"upcoming", "7", "--ids-only", "--no-input")
	if code != exitOK || out != "date\nearly\nfloating\nlate\nlegacy\n" {
		t.Fatalf("upcoming default must order supported due values chronologically: exit=%d stdout=%q stderr=%s", code, out, errOut)
	}
}
