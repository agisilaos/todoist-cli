package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestProfileStructuredOutputMatchesPublishedContracts(t *testing.T) {
	path, _ := profileFixture(t)
	writeProfileFixture(t, config.CredentialsPathFromConfig(path), `{"profiles":{"one":{"token":"synthetic-contract-one"},"two":{"token":"synthetic-contract-two"}}}`)
	for _, operation := range []string{"list", "current", "use", "remove"} {
		t.Run(operation, func(t *testing.T) {
			args := []string{"profile", operation}
			switch operation {
			case "current":
				args = append(args, "--profile", "one")
			case "use":
				args = append(args, "one")
			case "remove":
				args = append(args, "two")
			}
			var first map[string]any
			for _, mode := range []string{"--json", "--ndjson"} {
				code, out, errOut := executeAuthorization(t, path, append(args, mode)...)
				if code != 0 || errOut != "" || strings.Contains(out, "synthetic-contract-") {
					t.Fatal("structured profile operation failed or disclosed a credential")
				}
				got := profileObject(t, out)
				if mode == "--ndjson" && strings.Count(out, "\n") != 1 {
					t.Fatal("NDJSON profile output must be one complete report")
				}
				if first == nil {
					first = got
				} else {
					a, _ := json.Marshal(first)
					b, _ := json.Marshal(got)
					if !bytes.Equal(a, b) {
						t.Fatal("JSON and NDJSON profile contracts differ")
					}
				}
				code, schemaOut, _ := executeAuthorization(t, path, "schema", "--name", "profile_"+operation)
				var definitions []schemaDef
				if code != 0 || json.Unmarshal([]byte(schemaOut), &definitions) != nil || len(definitions) != 1 {
					t.Fatal("profile schema is not discoverable")
				}
				definition := definitions[0].Schema.(map[string]any)
				for _, required := range definition["required"].([]any) {
					if _, ok := got[required.(string)]; !ok {
						t.Fatalf("profile output missing documented field %s", required)
					}
				}
			}
		})
	}
}

func TestOAuthScopeErrorsDoNotBorrowStoredAuthorization(t *testing.T) {
	var stderr bytes.Buffer
	ctx := &Context{Stderr: &stderr, Mode: output.ModeJSON, Profile: "existing", TokenSource: "credentials"}
	r := authorization.Resolve([]byte(readOnlyMetadata), "credentials", true)
	ctx.Authorization = &r
	writeError(ctx, &authorization.Error{Code: "OAUTH_SCOPE_INVALID", Message: "OAuth returned an unacceptable scope grant; the stored credential was not changed.", Reason: "broader-than-requested"})
	var report map[string]any
	if json.Unmarshal(stderr.Bytes(), &report) != nil || report["code"] != "OAUTH_SCOPE_INVALID" {
		t.Fatal("scope error contract is missing")
	}
	details := report["details"].(map[string]any)
	if details["authorization"] != nil || details["source"] != nil || details["reason"] != "broader-than-requested" {
		t.Fatal("candidate grant error attached another credential's evidence")
	}
}

func TestProfileUseHonorsEnvironmentConfigPath(t *testing.T) {
	path, _ := profileFixture(t)
	writeProfileFixture(t, config.CredentialsPathFromConfig(path), `{"profiles":{"scratch":{"token":"synthetic-config-path"}}}`)
	t.Setenv("TODOIST_CONFIG", path)
	var stdout, stderr bytes.Buffer
	code := executeTest([]string{"profile", "use", "scratch", "--no-input", "--json"}, &stdout, &stderr)
	if code != 0 || profileObject(t, stdout.String())["config_path"] != path {
		t.Fatal("profile use ignored TODOIST_CONFIG")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte(`"default_profile": "scratch"`)) {
		t.Fatal("profile use did not persist to the resolved config")
	}
}

func TestProfileRemovalCleanupGuidanceTargetsNamedProfile(t *testing.T) {
	path, _ := profileFixture(t)
	writeProfileFixture(t, path, `{"default_profile":"other"}`)
	native := &profileCleanupSecrets{cliSecrets: cliSecrets{values: map[string]string{}}}
	store := credentials.New(config.CredentialsPathFromConfig(path), native, nil)
	if err := store.Save(operationContext(&Context{}), "scratch", config.Credential{Token: "synthetic-cleanup-target"}, "native"); err != nil {
		t.Fatal(err)
	}
	native.failDelete = true
	env := profileStoreEnvironment(store)
	code, out, errOut := executeAuthorizationWithEnvironment(t, path, env, "profile", "remove", "scratch", "--json")
	if code != 3 || out != "" {
		t.Fatal("cleanup failure did not preserve the error contract")
	}
	result := profileObject(t, errOut)
	details := result["details"].(map[string]any)
	if result["code"] != "CREDENTIAL_CLEANUP_PENDING" || details["committed"] != true || details["profile"] != "scratch" || details["authorization"] != nil {
		t.Fatal("cleanup error associated the wrong credential")
	}
	if !strings.Contains(details["retry_command"].(string), "profile remove scratch") || !strings.Contains(details["repair_command"].(string), "--profile scratch auth repair") || !strings.Contains(details["repair_command"].(string), path) {
		t.Fatal("cleanup remediation lost its target or configuration")
	}
}

func TestLoginNextCommandRetainsConfigurationAndEndpoint(t *testing.T) {
	ctx := newAuthTestContext(t)
	ctx.Profile = "scratch profile"
	ctx.SavingBackend = "file"
	ctx.Global.BaseURL = "http://127.0.0.1:12345/api/v1"
	if err := storeProfileToken(ctx, "synthetic-next-command"); err != nil {
		t.Fatal(err)
	}
	out := ctx.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "--config "+shellEscape(ctx.ConfigPath)) || !strings.Contains(out, "--profile "+shellEscape(ctx.Profile)) || !strings.Contains(out, "--base-url "+shellEscape(ctx.Global.BaseURL)) || strings.Contains(out, "synthetic-next-command") {
		t.Fatal("login next command lost explicit configuration or exposed a credential")
	}
}

func TestProfileInspectionRecoveryRetainsConfiguration(t *testing.T) {
	path, _ := profileFixture(t)
	writeProfileFixture(t, path, `{"default_profile":"scratch"}`)
	native := &profileCleanupSecrets{cliSecrets: cliSecrets{values: map[string]string{}}}
	store := credentials.New(config.CredentialsPathFromConfig(path), native, nil)
	if err := store.Save(operationContext(&Context{}), "scratch", config.Credential{Token: "synthetic-inspection-recovery"}, "native"); err != nil {
		t.Fatal(err)
	}
	native.failDelete = true
	if err := store.Delete(operationContext(&Context{}), "scratch"); err == nil {
		t.Fatal("fixture did not retain pending native cleanup")
	}
	env := profileStoreEnvironment(store)
	for _, operation := range []string{"list", "current"} {
		_, out, _ := executeAuthorizationWithEnvironment(t, path, env, "profile", operation)
		if !strings.Contains(out, "--config "+shellEscape(path)) || !strings.Contains(out, "--profile scratch auth repair") || strings.Contains(out, "synthetic-inspection-recovery") {
			t.Fatalf("profile %s recovery lost its configuration or target", operation)
		}
	}
}
