package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
)

func profileFixture(t *testing.T) (string, string) {
	t.Helper()
	t.Setenv("TODOIST_TOKEN", "")
	t.Setenv("TODOIST_PROFILE", "")
	project := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(t.TempDir(), "config.json")
	return path, project
}

func writeProfileFixture(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func profileObject(t *testing.T, out string) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid output: %s: %v", out, err)
	}
	return result
}

func TestProfileSelectionPrecedence(t *testing.T) {
	for _, tc := range []struct{ name, user, project, env, flag, want, source string }{
		{"fallback", "", "", "", "", "default", "fallback"},
		{"user", "personal", "", "", "", "personal", "user"},
		{"project", "personal", "work", "", "", "work", "project"},
		{"environment", "personal", "work", "reader", "", "reader", "environment"},
		{"flag", "personal", "work", "reader", "scratch", "scratch", "flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, project := profileFixture(t)
			writeProfileFixture(t, path, `{"default_profile":"`+tc.user+`"}`)
			writeProfileFixture(t, filepath.Join(project, ".todoist.json"), `{"default_profile":"`+tc.project+`"}`)
			writeProfileFixture(t, config.CredentialsPathFromConfig(path), `{"profiles":{"`+tc.want+`":{"token":"synthetic-selection-secret"}}}`)
			t.Setenv("TODOIST_PROFILE", tc.env)
			args := []string{"profile", "current", "--json"}
			if tc.flag != "" {
				args = append(args, "--profile", tc.flag)
			}
			code, out, errOut := executeAuthorization(t, path, args...)
			if code != 0 {
				t.Fatalf("current: %d %s %s", code, out, errOut)
			}
			got := profileObject(t, out)
			if got["selected_profile"] != tc.want || got["selection_source"] != tc.source || got["profile_active"] != true {
				t.Fatalf("incorrect selection: %s", out)
			}
			if strings.Contains(out+errOut, "synthetic-selection-secret") {
				t.Fatal("metadata output leaked a token")
			}
		})
	}
}

type profileObservedStore struct {
	credentials.Store
	inspects, loads, probes int
}

func (s *profileObservedStore) Inspect(ctx context.Context, name string) (credentials.Info, error) {
	s.inspects++
	return s.Store.Inspect(ctx, name)
}
func (s *profileObservedStore) Load(ctx context.Context, name string) (config.Credential, error) {
	s.loads++
	return s.Store.Load(ctx, name)
}
func (s *profileObservedStore) Probe(ctx context.Context, name string) error {
	s.probes++
	return s.Store.Probe(ctx, name)
}

func profileStoreEnvironment(store credentials.Store) Environment {
	return Environment{local: localDependencies{credentialStore: func(string) credentials.Store { return store }}}
}

func TestProfileMetadataOperationsNeverRetrieveNativeSecrets(t *testing.T) {
	path, _ := profileFixture(t)
	native := &cliSecrets{values: map[string]string{}}
	store := credentials.New(config.CredentialsPathFromConfig(path), native, nil)
	if err := store.Save(context.Background(), "reader", config.Credential{Token: "synthetic-native-secret", Authorization: json.RawMessage(readOnlyMetadata)}, "native"); err != nil {
		t.Fatal(err)
	}
	native.inaccessible = true
	observed := &profileObservedStore{Store: store}
	env := profileStoreEnvironment(observed)
	for _, args := range [][]string{{"--profile", "reader", "profile", "list", "--json"}, {"--profile", "reader", "profile", "current", "--json"}, {"profile", "use", "reader", "--json"}} {
		code, out, errOut := executeAuthorizationWithEnvironment(t, path, env, args...)
		if code != 0 || strings.Contains(out+errOut, "synthetic-native-secret") {
			t.Fatalf("metadata operation: %d %s %s", code, out, errOut)
		}
	}
	if observed.loads != 0 || observed.probes != 0 {
		t.Fatalf("metadata retrieved/probed secrets: loads=%d probes=%d", observed.loads, observed.probes)
	}
	observed.inspects = 0
	t.Setenv("TODOIST_TOKEN", "synthetic-environment-secret")
	code, out, errOut := executeAuthorizationWithEnvironment(t, path, env, "profile", "current", "--json")
	if code != 0 || observed.inspects != 0 || strings.Contains(out+errOut, "synthetic-environment-secret") {
		t.Fatalf("environment override inspected storage: %d %s %s", code, out, errOut)
	}
	got := profileObject(t, out)
	report := got["authorization"].(map[string]any)
	if got["source"] != "env" || got["backend"] != "environment" || got["profile_active"] != false || report["mode"] != "unknown" || report["metadata_status"] != "external" || report["effective_scopes"] != nil || report["write_capable"] != true {
		t.Fatalf("environment borrowed stored authorization: %s", out)
	}
}

func TestProfileCurrentMissingAndInvalidMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, record string
		code         int
		symbol       string
	}{
		{"missing", `{}`, 4, "PROFILE_NOT_FOUND"},
		{"invalid", `{"default":{"token":"synthetic-invalid-secret","authorization":null}}`, 3, "AUTH_METADATA_INVALID"},
		{"unsupported", `{"default":{"token":"synthetic-invalid-secret","authorization":{"version":8}}}`, 3, "AUTH_METADATA_UNSUPPORTED"},
		{"storage", `{"default":{"token":"synthetic-invalid-secret","storage":{"version":8}}}`, 3, "CREDENTIAL_STORE_UNSUPPORTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, _ := profileFixture(t)
			writeProfileFixture(t, config.CredentialsPathFromConfig(path), `{"profiles":`+tc.record+`}`)
			code, out, errOut := executeAuthorization(t, path, "profile", "current", "--json")
			if code != tc.code || !strings.Contains(out, tc.symbol) || strings.Contains(out+errOut, "synthetic-invalid-secret") {
				t.Fatalf("current failure: %d %s %s", code, out, errOut)
			}
			got := profileObject(t, out)
			if got["profile_active"] != false {
				t.Fatalf("invalid profile reported active: %s", out)
			}
		})
	}
}

func TestProfileListReportsInactiveErrorsWithoutPoisoningCurrent(t *testing.T) {
	path, _ := profileFixture(t)
	writeProfileFixture(t, config.CredentialsPathFromConfig(path), `{"profiles":{"default":{"token":"synthetic-good"},"invalid":{"token":"synthetic-bad","authorization":null},"storage":{"storage":{"version":9}}}}`)
	code, out, errOut := executeAuthorization(t, path, "profile", "list", "--json")
	if code != 3 || !strings.Contains(out, "AUTH_METADATA_INVALID") || !strings.Contains(out, "CREDENTIAL_STORE_UNSUPPORTED") || strings.Contains(out+errOut, "synthetic-") {
		t.Fatalf("list did not report safe complete results: %d %s %s", code, out, errOut)
	}
	rows := profileObject(t, out)["profiles"].([]any)
	if len(rows) != 3 || rows[0].(map[string]any)["profile"] != "default" {
		t.Fatalf("missing/unsorted profiles: %s", out)
	}
	code, out, errOut = executeAuthorization(t, path, "profile", "current", "--json")
	if code != 0 || !strings.Contains(out, `"metadata_status": "legacy"`) {
		t.Fatalf("inactive invalid profile poisoned selection: %d %s %s", code, out, errOut)
	}
}

func TestProfileUsePersistsOnlyUserSelectionAndReportsOverrides(t *testing.T) {
	path, project := profileFixture(t)
	writeProfileFixture(t, path, `{"default_profile":"old","base_url":"https://user.example","future":{"keep":true}}`)
	writeProfileFixture(t, filepath.Join(project, ".todoist.json"), `{"default_profile":"project","base_url":"https://project.example","table_width":7}`)
	credentialsPath := config.CredentialsPathFromConfig(path)
	writeProfileFixture(t, credentialsPath, `{"future":"keep","profiles":{"old":{"token":"synthetic-old","future":"keep"},"next":{"token":"synthetic-next"}}}`)
	before, _ := os.ReadFile(credentialsPath)
	t.Setenv("TODOIST_BASE_URL", "https://environment.example")
	t.Setenv("TODOIST_TOKEN", "synthetic-environment-secret")
	code, out, errOut := executeAuthorization(t, path, "profile", "use", "next", "--json")
	if code != 0 {
		t.Fatalf("use: %d %s %s", code, out, errOut)
	}
	got := profileObject(t, out)
	if got["profile"] != "next" || got["selected_profile"] != "project" || got["selection_source"] != "project" || got["shadowed"] != true || got["environment_token_active"] != true {
		t.Fatalf("overrides not reported: %s", out)
	}
	data, _ := os.ReadFile(path)
	if !bytes.Contains(data, []byte(`"next"`)) || !bytes.Contains(data, []byte(`"future"`)) || !bytes.Contains(data, []byte("https://user.example")) || bytes.Contains(data, []byte("table_width")) || bytes.Contains(data, []byte("project.example")) || bytes.Contains(data, []byte("environment.example")) {
		t.Fatalf("use saved merged config: %s", data)
	}
	after, _ := os.ReadFile(credentialsPath)
	if !bytes.Equal(before, after) {
		t.Fatal("profile use rewrote credentials")
	}
	for _, tc := range []struct{ env, flag, want, source string }{{"environment", "", "environment", "environment"}, {"environment", "flag", "flag", "flag"}} {
		t.Setenv("TODOIST_PROFILE", tc.env)
		args := []string{"profile", "use", "next", "--json"}
		if tc.flag != "" {
			args = append(args, "--profile", tc.flag)
		}
		code, out, errOut = executeAuthorization(t, path, args...)
		if code != 0 {
			t.Fatalf("use override: %d %s %s", code, out, errOut)
		}
		got = profileObject(t, out)
		if got["selected_profile"] != tc.want || got["selection_source"] != tc.source {
			t.Fatalf("override precedence: %s", out)
		}
	}
}

func TestProfileUseRejectsMissingInvalidAndFailedPersistence(t *testing.T) {
	path, _ := profileFixture(t)
	writeProfileFixture(t, path, `{"default_profile":"default","future":true}`)
	writeProfileFixture(t, config.CredentialsPathFromConfig(path), `{"profiles":{"default":{"token":"synthetic-current","authorization":`+readOnlyMetadata+`},"invalid":{"token":"synthetic-other","authorization":null},"healthy":{"token":"synthetic-next"}}}`)
	before, _ := os.ReadFile(path)
	for _, tc := range []struct {
		name   string
		code   int
		symbol string
	}{{"absent", 4, "PROFILE_NOT_FOUND"}, {"invalid", 3, "AUTH_METADATA_INVALID"}} {
		code, out, errOut := executeAuthorization(t, path, "profile", "use", tc.name, "--json")
		if code != tc.code || !strings.Contains(errOut, tc.symbol) || strings.Contains(out+errOut, "synthetic-") || strings.Contains(errOut, "data:read") {
			t.Fatalf("target failure borrowed another credential evidence: %d %s %s", code, out, errOut)
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("rejected use changed config")
		}
	}
	env := Environment{local: localDependencies{persistProfileSelection: func(context.Context, string, string) error { return &config.SelectionError{Uncertain: true} }}}
	code, out, errOut := executeAuthorizationWithEnvironment(t, path, env, "profile", "use", "healthy", "--json")
	if code != 1 || out != "" || !strings.Contains(errOut, "may have changed") {
		t.Fatalf("persistence failure claimed success: %d %s %s", code, out, errOut)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed persistence changed config fixture")
	}
}

type profileCleanupSecrets struct {
	cliSecrets
	failDelete bool
}

func (s *profileCleanupSecrets) Delete(ctx context.Context, id string) error {
	if s.failDelete {
		return errors.New("synthetic-delete-secret")
	}
	return s.cliSecrets.Delete(ctx, id)
}

func TestProfileRemoveRetainsSelectionAndRecoversNativeCleanup(t *testing.T) {
	path, _ := profileFixture(t)
	writeProfileFixture(t, path, `{"default_profile":"selected","future":true}`)
	native := &profileCleanupSecrets{cliSecrets: cliSecrets{values: map[string]string{}}}
	store := credentials.New(config.CredentialsPathFromConfig(path), native, nil)
	for _, name := range []string{"selected", "other"} {
		if err := store.Save(context.Background(), name, config.Credential{Token: "synthetic-" + name}, "native"); err != nil {
			t.Fatal(err)
		}
	}
	env := profileStoreEnvironment(store)
	before, _ := os.ReadFile(path)
	native.failDelete = true
	code, out, errOut := executeAuthorizationWithEnvironment(t, path, env, "profile", "remove", "selected", "--json")
	if code != 3 || out != "" || !strings.Contains(errOut, "CREDENTIAL_CLEANUP_PENDING") || !strings.Contains(errOut, `"committed": true`) || strings.Contains(errOut, "synthetic-delete-secret") {
		t.Fatalf("cleanup failure: %d %s %s", code, out, errOut)
	}
	info, err := store.Inspect(context.Background(), "selected")
	if err != nil || info.Configured || info.Recovery != "cleanup" || info.Authorization != nil {
		t.Fatalf("removed profile not disabled: %+v %v", info, err)
	}
	code, out, errOut = executeAuthorizationWithEnvironment(t, path, env, "profile", "current", "--json")
	if code != 4 || !strings.Contains(out, `"selected_profile": "selected"`) || !strings.Contains(out, `"recovery": "cleanup"`) {
		t.Fatalf("removal silently activated another profile: %d %s %s", code, out, errOut)
	}
	code, out, errOut = executeAuthorizationWithEnvironment(t, path, env, "profile", "list", "--json")
	if code != 0 || !strings.Contains(out, `"profile": "selected"`) || !strings.Contains(out, `"recovery": "cleanup"`) {
		t.Fatalf("cleanup profile hidden: %d %s %s", code, out, errOut)
	}
	code, _, _ = executeAuthorizationWithEnvironment(t, path, env, "profile", "use", "selected", "--json")
	if code != 4 {
		t.Fatalf("disabled profile selectable: %d", code)
	}
	native.failDelete = false
	for i := 0; i < 2; i++ {
		code, out, errOut = executeAuthorizationWithEnvironment(t, path, env, "profile", "remove", "selected", "--json")
		if code != 0 {
			t.Fatalf("retry/idempotent removal: %d %s %s", code, out, errOut)
		}
	}
	names, err := store.List(context.Background())
	if err != nil || len(names) != 1 || names[0] != "other" || len(native.values) != 1 {
		t.Fatalf("cleanup changed other profile: %v %v", names, err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("removal changed saved default")
	}
}

func TestProfileRemoveInvalidAuthorizationAndEnvironmentOverride(t *testing.T) {
	path, _ := profileFixture(t)
	writeProfileFixture(t, config.CredentialsPathFromConfig(path), `{"future":"keep","profiles":{"bad":{"token":"synthetic-bad","authorization":null},"default":{"token":"synthetic-default","future":{"keep":true}}}}`)
	t.Setenv("TODOIST_TOKEN", "synthetic-environment-secret")
	code, out, errOut := executeAuthorization(t, path, "profile", "remove", "bad", "--json")
	if code != 0 || !strings.Contains(out, `"environment_token_active": true`) || strings.Contains(out+errOut, "synthetic-") {
		t.Fatalf("remove invalid profile: %d %s %s", code, out, errOut)
	}
	data, _ := os.ReadFile(config.CredentialsPathFromConfig(path))
	if bytes.Contains(data, []byte(`"bad"`)) || !bytes.Contains(data, []byte("synthetic-default")) || !bytes.Contains(data, []byte(`"future"`)) {
		t.Fatal("removal changed unrelated credential")
	}
}

func TestProfileArgumentErrorsDoNotMutateState(t *testing.T) {
	path, _ := profileFixture(t)
	for _, args := range [][]string{{"profile", "list", "extra"}, {"profile", "current", "extra"}, {"profile", "use"}, {"profile", "remove"}, {"profile", "use", "one", "two"}, {"profile", "remove", "one", "two"}} {
		code, _, _ := executeAuthorization(t, path, append(args, "--json")...)
		if code != 2 {
			t.Fatalf("unexpected argument status %d for %v", code, args)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("bad arguments created configuration")
	}
}
