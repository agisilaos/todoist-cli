package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestFilterExplicitReferencePreservesID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("commands") != "" {
			t.Errorf("dry run dispatched mutation: %v", r.Form)
			http.Error(w, "unexpected mutation", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"filters":[{"id":"wrong","name":"real","query":"today"},{"id":"real","name":"Target","query":"tomorrow"}]}`)
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
	for _, ref := range []string{"id:real", "https://app.todoist.com/app/filter/target-real"} {
		t.Run(ref, func(t *testing.T) {
			code, out, errOut := executeAuthorizationWithEnvironment(t, filepath.Join(t.TempDir(), "config.json"), env,
				"filter", "delete", ref, "--yes", "--dry-run", "--json", "--no-input")
			var result struct {
				Payload struct{ ID string } `json:"payload"`
			}
			if code != exitOK || json.Unmarshal([]byte(out), &result) != nil || result.Payload.ID != "real" {
				t.Fatalf("explicit filter reference must target real: exit=%d stdout=%s stderr=%s", code, out, errOut)
			}
		})
	}
}

func TestFilterReferenceHonorsFuzzySelection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/sync" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("commands") != "" {
			t.Errorf("dry run dispatched mutation: %v", r.Form)
			http.Error(w, "unexpected mutation", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"filters":[{"id":"work","name":"Work Inbox","query":"today"}]}`)
	}))
	defer server.Close()
	for _, tc := range []struct {
		name, ref, fuzzyEnv string
		flags               []string
		wantCode            int
	}{
		{name: "default exact matching", ref: "WrkInbx", wantCode: exitNotFound},
		{name: "explicit no fuzzy", ref: "WrkInbx", flags: []string{"--no-fuzzy"}, wantCode: exitNotFound},
		{name: "no fuzzy overrides flag", ref: "WrkInbx", flags: []string{"--fuzzy", "--no-fuzzy"}, wantCode: exitNotFound},
		{name: "no fuzzy overrides environment", ref: "WrkInbx", fuzzyEnv: "1", flags: []string{"--no-fuzzy"}, wantCode: exitNotFound},
		{name: "fuzzy opt in", ref: "WrkInbx", flags: []string{"--fuzzy"}, wantCode: exitOK},
		{name: "environment opt in", ref: "WrkInbx", fuzzyEnv: "1", wantCode: exitOK},
		{name: "exact name", ref: "Work Inbox", wantCode: exitOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := Environment{Getenv: func(key string) string {
				switch key {
				case "TODOIST_TOKEN":
					return "synthetic-token"
				case "TODOIST_BASE_URL":
					return server.URL
				case "TODOIST_FUZZY":
					return tc.fuzzyEnv
				}
				return ""
			}}
			args := append([]string{"filter", "delete", tc.ref, "--yes", "--dry-run", "--json", "--no-input"}, tc.flags...)
			code, out, errOut := executeAuthorizationWithEnvironment(t, filepath.Join(t.TempDir(), "config.json"), env, args...)
			if code != tc.wantCode || tc.wantCode != exitOK && out != "" {
				t.Fatalf("fuzzy policy exit=%d want=%d stdout=%s stderr=%s", code, tc.wantCode, out, errOut)
			}
			if code == exitOK {
				var result struct {
					Payload struct{ ID string } `json:"payload"`
				}
				if json.Unmarshal([]byte(out), &result) != nil || result.Payload.ID != "work" {
					t.Fatalf("expected resolved filter work: stdout=%s stderr=%s", out, errOut)
				}
			}
		})
	}
}
