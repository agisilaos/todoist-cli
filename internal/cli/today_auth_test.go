package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
)

func TestTodayLoadsStoredCredentialsInEveryOutputMode(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/filter" || r.URL.Query().Get("query") != "overdue | today" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer synthetic-today-token" {
			t.Error("stored credential was not used")
		}
		fmt.Fprint(w, `{"results":[{"id":"101","content":"Synthetic task"}],"next_cursor":null}`)
	}))
	defer server.Close()
	t.Setenv("TODOIST_BASE_URL", server.URL)
	old := newCredentialStore
	defer func() { newCredentialStore = old }()
	for _, backend := range []string{"file", "native"} {
		t.Run(backend, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			store := credentials.New(config.CredentialsPathFromConfig(path), &cliSecrets{values: map[string]string{}}, nil)
			if err := store.Save(context.Background(), "default", config.Credential{Token: "synthetic-today-token"}, backend); err != nil {
				t.Fatal(err)
			}
			newCredentialStore = func(string) credentials.Store { return store }
			for _, mode := range []string{"--json", "--ndjson", "--plain", "--ids-only"} {
				code, out, errOut := executeAuthorization(t, path, "today", mode)
				if code != 0 || !strings.Contains(out, "101") || errOut != "" {
					t.Fatalf("%s exit=%d stdout=%q stderr=%q", mode, code, out, errOut)
				}
			}
			code, _, errOut := executeAuthorization(t, path, "view", "https://app.todoist.com/app/today", "--json")
			if code != 0 {
				t.Fatalf("today URL: exit=%d stderr=%q", code, errOut)
			}
		})
	}
}

func TestTodayWithoutCredentialsReturnsAuthError(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	code, out, errOut := executeAuthorization(t, filepath.Join(t.TempDir(), "config.json"), "today", "--json")
	if code != exitAuth || out != "" || !strings.Contains(errOut, "missing auth token") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, errOut)
	}
}
