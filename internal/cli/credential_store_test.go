package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
)

type cliSecrets struct {
	values       map[string]string
	inaccessible bool
}

func (s *cliSecrets) Probe(context.Context) error {
	if s.inaccessible {
		return &credentials.Error{Kind: credentials.Locked}
	}
	return nil
}
func (s *cliSecrets) Read(_ context.Context, id string) (string, error) {
	if s.inaccessible {
		return "", errors.New("untrusted-secret-from-adapter")
	}
	v, ok := s.values[id]
	if !ok {
		return "", &credentials.Error{Kind: credentials.Missing}
	}
	return v, nil
}
func (s *cliSecrets) Write(_ context.Context, id, token string) error {
	s.values[id] = token
	return nil
}
func (s *cliSecrets) Delete(_ context.Context, id string) error { delete(s.values, id); return nil }
func TestNativeStatusAndHelpNeverRetrieveSecrets(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	t.Setenv("TODOIST_PROFILE", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	native := &cliSecrets{values: map[string]string{}}
	store := credentials.New(filepath.Join(dir, "credentials.json"), native, nil)
	if err := store.Save(context.Background(), "default", config.Credential{Token: "fixture-native-token"}, "native"); err != nil {
		t.Fatal(err)
	}
	native.inaccessible = true
	old := newCredentialStore
	newCredentialStore = func(string) credentials.Store { return store }
	defer func() { newCredentialStore = old }()
	for _, args := range [][]string{{"auth", "status", "--json"}, {"auth", "--help"}, {"--help"}} {
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 0 || strings.Contains(out+errOut, "fixture-native-token") {
			t.Fatal("metadata-only command failed or leaked a token")
		}
	}
	code, _, errOut := executeAuthorization(t, path, "task", "list", "--json")
	if code != 3 || !strings.Contains(errOut, "CREDENTIAL_STORE_IO") || strings.Contains(errOut, "untrusted-secret") {
		t.Fatal("native error contract failed")
	}
	t.Setenv("TODOIST_TOKEN", "environment-fixture")
	code, out, _ := executeAuthorization(t, path, "auth", "status", "--json")
	if code != 0 || !strings.Contains(out, `"source": "env"`) {
		t.Fatal("environment did not bypass native store")
	}
}

func TestCredentialCLISelectionMigrationAndRecoveryCommands(t *testing.T) {
	probe := newAuthTestContext(t)
	authValidationServer(t, probe, 200, "cli-synthetic-token")
	t.Setenv("TODOIST_BASE_URL", probe.Config.BaseURL)
	t.Setenv("TODOIST_TOKEN", "")
	t.Setenv("TODOIST_PROFILE", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	native := &cliSecrets{values: map[string]string{}}
	old := newCredentialStore
	newCredentialStore = func(configPath string) credentials.Store {
		return credentials.New(config.CredentialsPathFromConfig(configPath), native, nil)
	}
	defer func() { newCredentialStore = old }()
	input, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	originalStdin := os.Stdin
	os.Stdin = input
	defer func() { os.Stdin = originalStdin }()
	resetInput := func() {
		input.Truncate(0)
		input.Seek(0, 0)
		input.WriteString("cli-synthetic-token\n")
		input.Seek(0, 0)
	}
	resetInput()
	code, out, errOut := executeAuthorization(t, path, "auth", "login", "--token-stdin", "--json")
	if code != 0 || !strings.Contains(out, `"backend": "keychain"`) || strings.Contains(out+errOut, "cli-synthetic-token") {
		t.Fatal("new login did not use native storage safely")
	}
	resetInput()
	code, _, errOut = executeAuthorization(t, path, "auth", "login", "--token-stdin", "--credential-store=file", "--json")
	if code != 2 || !strings.Contains(errOut, "CREDENTIAL_STORE_SELECTION_CONFLICT") {
		t.Fatal("login silently changed backend")
	}
	code, out, _ = executeAuthorization(t, path, "auth", "migrate", "--credential-store=file", "--json")
	if code != 0 || !strings.Contains(out, `"backend": "file"`) || len(native.values) != 0 {
		t.Fatal("explicit migration failed")
	}
	code, _, _ = executeAuthorization(t, path, "auth", "repair", "--json")
	if code != 0 {
		t.Fatal("idempotent repair failed")
	}
	code, _, _ = executeAuthorization(t, path, "auth", "logout", "--json")
	if code != 0 {
		t.Fatal("logout failed")
	}
	code, out, _ = executeAuthorization(t, path, "auth", "status", "--json")
	if code != 0 || !strings.Contains(out, `"configured": false`) {
		t.Fatal("logout left profile configured")
	}
	native.inaccessible = true
	resetInput()
	code, _, errOut = executeAuthorization(t, path, "auth", "login", "--token-stdin", "--json")
	if code != 3 || !strings.Contains(errOut, "CREDENTIAL_STORE_LOCKED") {
		t.Fatal("locked new login did not fail explicitly")
	}
}

func TestProjectConfigurationCannotSelectCredentialBackend(t *testing.T) {
	dir := t.TempDir()
	project := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"credential_store":"file"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".todoist.json"), []byte(`{"credential_store":"native"}`), 0600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	ctx := newAuthTestContext(t)
	ctx.Global.ConfigPath = path
	if err := loadConfig(ctx); err != nil {
		t.Fatal(err)
	}
	ctx.Stdin = strings.NewReader("synthetic-file-token\n")
	authValidationServer(t, ctx, 200, "synthetic-file-token")
	if err := authLogin(ctx, []string{"--token-stdin"}); err != nil {
		t.Fatal(err)
	}
	info, err := profileStore(ctx).Inspect(context.Background(), ctx.Profile)
	if err != nil || info.Backend != "file" {
		t.Fatal("project config overrode user storage choice")
	}
}
