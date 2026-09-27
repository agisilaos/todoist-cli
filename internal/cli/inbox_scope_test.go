package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestInboxWithoutCredentialsReturnsAuthError(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	for _, mode := range []string{"", "--json", "--ndjson", "--plain", "--ids-only"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{"--no-input", "inbox"}
			if mode != "" {
				args = append(args, mode)
			}
			code, out, errOut := executeAuthorization(t, filepath.Join(t.TempDir(), "config.json"), args...)
			if code != exitAuth || out != "" || !strings.Contains(errOut, "missing auth token") {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
			}
			if mode == "--json" || mode == "--ids-only" {
				if !json.Valid([]byte(errOut)) {
					t.Fatalf("invalid JSON error: %q", errOut)
				}
			}
		})
	}
}

func TestImplicitInboxScopeOutput(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		mode       output.Mode
		quiet      bool
		empty      bool
		noInbox    bool
		lookupFail bool
		tasksFail  bool
		wantLabel  bool
		wantScope  string
	}{
		{name: "default", wantLabel: true, wantScope: "100"},
		{name: "alias", args: []string{"ls"}, wantLabel: true, wantScope: "100"},
		{name: "bare inbox", args: []string{"inbox"}, wantLabel: true, wantScope: "100"},
		{name: "empty", empty: true, wantLabel: true, wantScope: "100"},
		{name: "quiet", quiet: true, wantScope: "100"},
		{name: "json", mode: output.ModeJSON, wantScope: "100"},
		{name: "ndjson", mode: output.ModeNDJSON, wantScope: "100"},
		{name: "plain", mode: output.ModePlain, wantScope: "100"},
		{name: "ids", mode: output.ModeIDsOnly, wantScope: "100"},
		{name: "all projects", args: []string{"list", "--all-projects"}},
		{name: "explicit project", args: []string{"list", "--project", "200"}, wantScope: "200"},
		{name: "explicit inbox", args: []string{"list", "--project", "100"}, wantScope: "100"},
		{name: "label", args: []string{"list", "--label", "focus"}},
		{name: "ids selection", args: []string{"list", "--id", "101"}},
		{name: "filter", args: []string{"list", "--filter", "today"}},
		{name: "preset", args: []string{"list", "--preset", "today"}},
		{name: "completed", args: []string{"list", "--completed"}},
		{name: "missing inbox", noInbox: true},
		{name: "failed lookup", lookupFail: true},
		{name: "failed tasks", tasksFail: true, wantScope: "100"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			taskRequests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/projects":
					if tc.lookupFail {
						http.Error(w, "unavailable", http.StatusBadRequest)
						return
					}
					fmt.Fprintf(w, `{"results":[{"id":"100","name":"Inbox","inbox_project":%t},{"id":"200","name":"Work"}]}`, !tc.noInbox)
				case "/sections", "/labels":
					fmt.Fprint(w, `{"results":[]}`)
				default:
					taskRequests++
					if got := r.URL.Query().Get("project_id"); got != tc.wantScope {
						t.Errorf("project scope = %q, want %q", got, tc.wantScope)
					}
					if tc.tasksFail {
						http.Error(w, "unavailable", http.StatusBadRequest)
					} else if tc.empty {
						fmt.Fprint(w, `{"results":[]}`)
					} else {
						fmt.Fprint(w, `{"results":[{"id":"101","content":"Scope fixture","project_id":"100"}]}`)
					}
				}
			}))
			defer server.Close()
			var out, errOut bytes.Buffer
			ctx := &Context{Stdout: &out, Stderr: &errOut, Token: "synthetic-token", Mode: tc.mode,
				Config: config.Config{BaseURL: server.URL, TimeoutSeconds: 2}, Global: GlobalOptions{Quiet: tc.quiet, NoInput: true}}
			if ctx.Mode == "" {
				ctx.Mode = output.ModeHuman
			}
			args := tc.args
			if len(args) == 0 {
				args = []string{"list"}
			}
			var err error
			if args[0] == "inbox" {
				err = inboxCommand(ctx, nil)
			} else {
				err = taskCommand(ctx, args)
			}
			if (err != nil) != (tc.tasksFail || tc.lookupFail || tc.noInbox) {
				t.Fatalf("unexpected error: %v", err)
			}
			wantRequests := 1
			if tc.lookupFail || tc.noInbox {
				wantRequests = 0
			}
			if taskRequests != wantRequests {
				t.Fatalf("task requests = %d, want %d", taskRequests, wantRequests)
			}
			if got := strings.HasPrefix(out.String(), "Inbox\n"); got != tc.wantLabel {
				t.Fatalf("label=%t, want %t; output=%q", got, tc.wantLabel, out.String())
			}
			if err != nil && out.Len() != 0 {
				t.Fatalf("failed request emitted stdout: %q", out.String())
			}
			if tc.mode == output.ModeJSON && !json.Valid(out.Bytes()) {
				t.Fatalf("invalid JSON: %q", out.String())
			}
			if tc.mode == output.ModeIDsOnly && out.String() != "101\n" {
				t.Fatalf("changed ID output: %q", out.String())
			}
		})
	}
}

func TestInboxLookupFailureNeverListsOtherProjects(t *testing.T) {
	for _, response := range []struct {
		name   string
		status int
		body   string
		code   int
	}{
		{"lookup failure", http.StatusBadRequest, `{"error":"lookup failed"}`, exitError},
		{"auth failure", http.StatusUnauthorized, `{"error":"invalid token"}`, exitAuth},
		{"missing Inbox", http.StatusOK, `{"results":[{"id":"200","name":"Work"}]}`, exitNotFound},
	} {
		t.Run(response.name, func(t *testing.T) {
			taskRequests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/projects" {
					w.WriteHeader(response.status)
					fmt.Fprint(w, response.body)
					return
				}
				taskRequests++
				fmt.Fprint(w, `{"results":[{"id":"201","content":"Work task","project_id":"200"}]}`)
			}))
			defer server.Close()
			t.Setenv("TODOIST_TOKEN", "synthetic-token")
			t.Setenv("TODOIST_BASE_URL", server.URL)
			for _, command := range [][]string{{"inbox"}, {"task", "list"}, {"task", "ls"}} {
				for _, mode := range []string{"--json", "--ids-only", "--plain", "--ndjson"} {
					args := append(append([]string{}, command...), "--no-input", mode)
					code, out, errOut := executeAuthorization(t, filepath.Join(t.TempDir(), "config.json"), args...)
					if code != response.code || out != "" || errOut == "" {
						t.Fatalf("%v: exit=%d stdout=%q stderr=%q", args, code, out, errOut)
					}
					if (mode == "--json" || mode == "--ids-only") && !json.Valid([]byte(errOut)) {
						t.Fatalf("invalid JSON error: %q", errOut)
					}
				}
			}
			if taskRequests != 0 {
				t.Fatalf("Inbox lookup failures dispatched %d task requests", taskRequests)
			}
			code, out, errOut := executeAuthorization(t, filepath.Join(t.TempDir(), "config.json"), "task", "list", "--all-projects", "--json")
			if code != 0 || !strings.Contains(out, "201") || errOut != "" || taskRequests != 1 {
				t.Fatalf("explicit all-projects: exit=%d stdout=%q stderr=%q requests=%d", code, out, errOut, taskRequests)
			}
		})
	}
}
