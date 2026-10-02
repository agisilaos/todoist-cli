package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteInvocationInputsAreIndependent(t *testing.T) {
	t.Parallel()
	for _, year := range []int{2020, 2040} {
		t.Run(fmt.Sprint(year), func(t *testing.T) {
			t.Parallel()
			// Local midnight differs from the UTC date used by upcoming.
			now := time.Date(year, 1, 2, 1, 0, 0, 0, time.FixedZone("east", 2*60*60))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != fmt.Sprintf("Bearer synthetic-%d", year) {
					t.Error("another invocation's credential was used")
				}
				fmt.Fprintf(w, `{"results":[{"id":"overdue","due":{"date":"%d-12-31"}},{"id":"today","due":{"date":"%d-01-01"}},{"id":"last","due":{"date":"%d-01-07"}},{"id":"outside","due":{"date":"%d-01-08"}},{"id":"undated","due":null}]}`, year-1, year, year, year)
			}))
			defer server.Close()
			env := map[string]string{
				"TODOIST_TOKEN":    fmt.Sprintf("synthetic-%d", year),
				"TODOIST_BASE_URL": server.URL,
				"TODOIST_CONFIG":   filepath.Join(t.TempDir(), "config.json"),
			}
			var out, errOut bytes.Buffer
			code := executeTestWithEnvironment([]string{"upcoming", "--no-input", "--ids-only"}, &out, &errOut, Environment{
				Now:    func() time.Time { return now },
				Getenv: func(key string) string { return env[key] },
			})
			if code != 0 || out.String() != "today\nlast\n" || errOut.Len() != 0 {
				t.Fatalf("exit %d stdout %q stderr %q", code, out.String(), errOut.String())
			}
		})
	}
}

func TestExecuteUsesInjectedStdinForManualLogin(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects" || r.Header.Get("Authorization") != "Bearer synthetic-input" {
			t.Error("login did not validate the injected stdin credential")
		}
		fmt.Fprint(w, `{"results":[]}`)
	}))
	defer server.Close()
	var out, errOut bytes.Buffer
	env := map[string]string{"TODOIST_CONFIG": filepath.Join(t.TempDir(), "config.json"), "TODOIST_BASE_URL": server.URL}
	code := executeTestWithEnvironment([]string{"auth", "login", "--token-stdin", "--print-env", "--no-input", "--json"}, &out, &errOut, Environment{
		Stdin:  strings.NewReader("synthetic-input\n"),
		Getenv: func(key string) string { return env[key] },
	})
	var result struct {
		Export string `json:"export"`
	}
	if code != 0 || json.Unmarshal(out.Bytes(), &result) != nil || result.Export != "export TODOIST_TOKEN=synthetic-input" || errOut.Len() != 0 {
		t.Fatalf("exit %d stdout %q stderr %q", code, out.String(), errOut.String())
	}
}

func TestInvocationEnvironmentSelectsConfigAndProfile(t *testing.T) {
	t.Parallel()
	xdg := t.TempDir()
	env := map[string]string{"XDG_CONFIG_HOME": xdg, "TODOIST_PROFILE": "isolated", "TODOIST_TOKEN": "synthetic", "TODOIST_TIMEOUT": "25", "TODOIST_TABLE_WIDTH": "88", "TODOIST_FUZZY": "1", "TODOIST_ACCESSIBLE": "1", "COLUMNS": "72"}
	ctx := &Context{Getenv: func(key string) string { return env[key] }}
	if err := loadConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.ConfigPath != filepath.Join(xdg, "todoist", "config.json") || ctx.Profile != "isolated" || ctx.SelectionSource != "environment" || ctx.Config.TimeoutSeconds != 25 || ctx.Config.TableWidth != 88 || !ctx.Fuzzy || !ctx.Accessible || terminalWidth(ctx) != 72 {
		t.Fatalf("injected environment not applied: %+v", ctx.Config)
	}
	if ctx.TokenSource != "env" || currentAuthorization(ctx).Mode == nil || *currentAuthorization(ctx).Mode != "unknown" {
		t.Fatal("injected environment token changed authorization semantics")
	}
}

func TestInvocationLocalStoresAndPersistenceAreIndependent(t *testing.T) {
	t.Parallel()
	for _, profile := range []string{"first", "second"} {
		t.Run(profile, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "config.json")
			store := credentials.New(config.CredentialsPathFromConfig(path), &cliSecrets{values: map[string]string{}}, nil)
			if err := store.Save(context.Background(), profile, config.Credential{Token: "synthetic-" + profile}, "native"); err != nil {
				t.Fatal(err)
			}
			factories, persists := 0, 0
			env := Environment{Getenv: func(key string) string {
				if key == "TODOIST_CONFIG" {
					return path
				}
				return ""
			}, local: localDependencies{
				credentialStore: func(got string) credentials.Store {
					factories++
					if got != path {
						t.Errorf("wrong config: %s", got)
					}
					return store
				},
				persistProfileSelection: func(_ context.Context, gotPath, gotProfile string) error {
					persists++
					if gotPath != path || gotProfile != profile {
						t.Errorf("another invocation's persistence: %s %s", gotPath, gotProfile)
					}
					return nil
				},
			}}
			var out, errOut bytes.Buffer
			code := executeTestWithEnvironment([]string{"profile", "use", profile, "--json", "--no-input"}, &out, &errOut, env)
			if code != 0 || factories != 1 || persists != 1 || !strings.Contains(out.String(), `"saved": true`) || errOut.Len() != 0 {
				t.Fatalf("local adapters crossed: exit=%d factory=%d persist=%d output=%s stderr=%s", code, factories, persists, out.String(), errOut.String())
			}
		})
	}
}

func TestExecuteDefaultEntrypointHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Execute([]string{"--help"}, &out, &errOut); code != 0 || out.Len() == 0 || errOut.Len() != 0 {
		t.Fatalf("process-default entrypoint: %d %s %s", code, out.String(), errOut.String())
	}
}
