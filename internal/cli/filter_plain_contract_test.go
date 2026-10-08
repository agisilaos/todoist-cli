package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestFilterListPlainAndRedirectedOutputAreTSV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/sync" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("commands") != "" {
			t.Errorf("listing dispatched mutation: %v", r.Form)
			http.Error(w, "unexpected mutation", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"filters":[{"id":"f1","name":"Work Inbox","query":"today","color":"berry","is_favorite":true}]}`)
	}))
	defer server.Close()
	env := Environment{Getenv: func(key string) string {
		switch key {
		case "TODOIST_TOKEN":
			return "synthetic-token"
		case "TODOIST_BASE_URL":
			return server.URL
		}
		return ""
	}}
	for _, plain := range []bool{true, false} {
		t.Run(fmt.Sprintf("explicit_plain=%t", plain), func(t *testing.T) {
			args := []string{"filter", "list", "--no-input"}
			if plain {
				args = append(args, "--plain")
			}
			code, out, errOut := executeAuthorizationWithEnvironment(t, filepath.Join(t.TempDir(), "config.json"), env, args...)
			if code != exitOK || out != "f1\tWork Inbox\ttoday\tberry\tyes\n" || errOut != "" {
				t.Fatalf("plain filter output must be TSV without headings: exit=%d stdout=%q stderr=%s", code, out, errOut)
			}
		})
	}
}
