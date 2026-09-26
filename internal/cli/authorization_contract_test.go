package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

const readOnlyMetadata = `{"version":1,"mode":"read-only","origin":"oauth-pkce","requested_scopes":["data:read"],"effective_scopes":["data:read"],"scope_evidence":"token-response"}`

func authorizationFixture(t *testing.T, metadata string) string {
	t.Helper()
	t.Setenv("TODOIST_TOKEN", "")
	t.Setenv("TODOIST_PROFILE", "")
	dir := t.TempDir()
	suffix := ""
	if metadata != "" {
		suffix = `,"authorization":` + metadata
	}
	data := `{"profiles":{"default":{"token":"secret-fixture"` + suffix + `}}}`
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "config.json")
}

func executeAuthorization(t *testing.T, path string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Execute(append([]string{"--config", path}, args...), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestAuthorizationLegacyStatusDoesNotInventScopesOrRewriteCredential(t *testing.T) {
	path := authorizationFixture(t, "")
	credentials := filepath.Join(filepath.Dir(path), "credentials.json")
	before, _ := os.ReadFile(credentials)
	code, out, errOut := executeAuthorization(t, path, "auth", "status", "--json")
	var status struct {
		Authorization struct {
			Mode            string `json:"mode"`
			EffectiveScopes any    `json:"effective_scopes"`
			WriteCapable    bool   `json:"write_capable"`
			Reason          string `json:"write_capability_reason"`
		} `json:"authorization"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &status) != nil {
		t.Fatalf("status %d: %s %s", code, out, errOut)
	}
	if status.Authorization.Mode != "unknown" || status.Authorization.EffectiveScopes != nil || !status.Authorization.WriteCapable || status.Authorization.Reason != "unknown-compatibility" {
		t.Fatalf("unexpected authorization: %s", out)
	}
	after, _ := os.ReadFile(credentials)
	if !bytes.Equal(before, after) {
		t.Fatal("status rewrote legacy credentials")
	}
}

func TestAuthorizationReadOnlyStatusAndEnvironmentOverride(t *testing.T) {
	path := authorizationFixture(t, readOnlyMetadata)
	for _, env := range []string{"", "environment-secret"} {
		t.Setenv("TODOIST_TOKEN", env)
		code, out, errOut := executeAuthorization(t, path, "auth", "status", "--json")
		var status struct {
			Source        string `json:"source"`
			Authorization struct {
				Mode            string   `json:"mode"`
				EffectiveScopes []string `json:"effective_scopes"`
				WriteCapable    bool     `json:"write_capable"`
			} `json:"authorization"`
		}
		if code != 0 || json.Unmarshal([]byte(out), &status) != nil {
			t.Fatalf("status %d: %s %s", code, out, errOut)
		}
		if env == "" && (status.Authorization.Mode != "read-only" || status.Authorization.WriteCapable || len(status.Authorization.EffectiveScopes) != 1 || status.Authorization.EffectiveScopes[0] != "data:read") {
			t.Fatalf("read-only status: %s", out)
		}
		if env != "" && (status.Source != "env" || status.Authorization.Mode != "unknown" || !status.Authorization.WriteCapable || status.Authorization.EffectiveScopes != nil) {
			t.Fatalf("environment status: %s", out)
		}
		if bytes.Contains([]byte(out+errOut), []byte("secret")) {
			t.Fatal("credential leaked")
		}
	}
}

func TestAuthorizationBlocksMutationBeforeHTTPWithStableError(t *testing.T) {
	path := authorizationFixture(t, readOnlyMetadata)
	var mutations atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutations.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"1"}`))
	}))
	defer server.Close()
	for _, flags := range [][]string{{"--plain"}, {"--json"}, {"--json", "--quiet-json"}} {
		args := append([]string{"--base-url", server.URL, "add", "test"}, flags...)
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 3 || out != "" || !strings.Contains(errOut, "Todoist mutation blocked: the active credential is read-only.") {
			t.Fatalf("blocked write: %d %s %s", code, out, errOut)
		}
		if flags[0] == "--json" {
			var e map[string]any
			if json.Unmarshal([]byte(errOut), &e) != nil || e["code"] != "READ_ONLY" || e["meta"] == nil {
				t.Fatalf("error contract: %s", errOut)
			}
		}
		if len(flags) > 1 && strings.Count(errOut, "\n") != 1 {
			t.Fatalf("quiet JSON not compact: %q", errOut)
		}
	}
	if mutations.Load() != 0 {
		t.Fatalf("sent %d mutations", mutations.Load())
	}
}

func TestAuthorizationInvalidMetadataIsNotLegacyAndCanBeInspected(t *testing.T) {
	for _, metadata := range []string{`null`, `{}`, `{"version":2}`, strings.Replace(readOnlyMetadata, `"read-only"`, `"read-write"`, 1)} {
		t.Run(metadata, func(t *testing.T) {
			path := authorizationFixture(t, metadata)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write([]byte(`{"results":[]}`)) }))
			defer server.Close()
			code, out, errOut := executeAuthorization(t, path, "auth", "status", "--json")
			var status struct {
				Authorization struct {
					Mode         any    `json:"mode"`
					WriteCapable bool   `json:"write_capable"`
					Status       string `json:"metadata_status"`
				} `json:"authorization"`
			}
			if code != 3 || json.Unmarshal([]byte(out), &status) != nil || status.Authorization.Mode != nil || status.Authorization.WriteCapable || !strings.Contains(errOut, "AUTH_METADATA_") {
				t.Fatalf("invalid status %d %s %s", code, out, errOut)
			}
			code, _, errOut = executeAuthorization(t, path, "--base-url", server.URL, "project", "list", "--json")
			if code != 3 || requests.Load() != 0 {
				t.Fatalf("invalid metadata sent HTTP: %d %d %s", code, requests.Load(), errOut)
			}
		})
	}
}

func TestAuthorizationDeviceLoginRequestsReadOnlyAndPersistsEvidence(t *testing.T) {
	path := authorizationFixture(t, "")
	requested := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.URL.Path == "/device" {
			requested <- r.Form.Get("scope")
			w.Write([]byte(`{"device_code":"d","user_code":"u","verification_uri":"https://example.test","interval":1,"expires_in":60}`))
			return
		}
		w.Write([]byte(`{"access_token":"new-secret","scope":"data:read"}`))
	}))
	defer server.Close()
	code, out, errOut := executeAuthorization(t, path, "auth", "login", "--oauth-device", "--read-only", "--client-id", "client", "--oauth-device-url", server.URL+"/device", "--oauth-token-url", server.URL+"/token", "--json")
	if code != 0 {
		t.Fatalf("login %d: %s %s", code, out, errOut)
	}
	if got := <-requested; got != "data:read" {
		t.Fatalf("requested scope: %s", got)
	}
	code, out, errOut = executeAuthorization(t, path, "auth", "status", "--json")
	if code != 0 || !strings.Contains(out, `"mode": "read-only"`) || !strings.Contains(out, `"origin": "oauth-device"`) || !strings.Contains(out, `"scope_evidence": "token-response"`) || strings.Contains(out+errOut, "secret") {
		t.Fatalf("stored authorization %d: %s %s", code, out, errOut)
	}
}

func TestAuthorizationDoctorReportsEvidenceAndSkipsInvalidCredentialProbe(t *testing.T) {
	for _, metadata := range []string{readOnlyMetadata, `null`} {
		t.Run(metadata, func(t *testing.T) {
			path := authorizationFixture(t, metadata)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write([]byte(`{"results":[]}`)) }))
			defer server.Close()
			_, out, errOut := executeAuthorization(t, path, "--base-url", server.URL, "doctor", "--json")
			if !strings.Contains(out, `"authorization"`) || strings.Contains(out+errOut, "secret-fixture") {
				t.Fatalf("unsafe or missing diagnostics: %s %s", out, errOut)
			}
			if metadata == `null` && requests.Load() != 0 {
				t.Fatal("invalid credential was used by doctor")
			}
			_, out, _ = executeAuthorization(t, path, "--base-url", server.URL, "doctor", "--plain")
			want := "read-only; writes blocked"
			if metadata == `null` {
				want = "invalid authorization metadata"
			}
			if !strings.Contains(out, want) {
				t.Fatalf("missing human diagnostics: %s", out)
			}
		})
	}
}

func TestAuthorizationAgentCannotContinuePastDeniedMutation(t *testing.T) {
	path := authorizationFixture(t, readOnlyMetadata)
	planPath := filepath.Join(filepath.Dir(path), "plan.json")
	plan := `{"version":1,"confirm_token":"reviewed","actions":[{"type":"task_add","content":"first"},{"type":"project_add","name":"second"}]}`
	if err := os.WriteFile(planPath, []byte(plan), 0600); err != nil {
		t.Fatal(err)
	}
	var mutations atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mutations.Add(1); w.Write([]byte(`{}`)) }))
	defer server.Close()
	for _, command := range []string{"apply", "run"} {
		args := []string{"--base-url", server.URL, "agent", command, "--plan", planPath, "--force", "--on-error", "continue", "--json"}
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 3 || out != "" || !strings.Contains(errOut, `"code": "READ_ONLY"`) {
			t.Fatalf("%s accepted blocked plan: %d %s %s", command, code, out, errOut)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(path), "agent_replay.json")); !os.IsNotExist(err) {
			t.Fatal("blocked plan wrote replay records")
		}
	}
	if mutations.Load() != 0 {
		t.Fatal("blocked plan sent mutations")
	}
}

func TestAuthorizationBulkCompletionDoesNotSwallowDenial(t *testing.T) {
	path := authorizationFixture(t, readOnlyMetadata)
	var mutations atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			mutations.Add(1)
		}
		w.Write([]byte(`{"results":[{"id":"task1","content":"one"},{"id":"task2","content":"two"}]}`))
	}))
	defer server.Close()
	code, out, errOut := executeAuthorization(t, path, "--base-url", server.URL, "task", "complete", "--filter", "today", "--yes", "--json")
	if code != 3 || mutations.Load() != 0 || !strings.Contains(errOut, "READ_ONLY") {
		t.Fatalf("bulk denial %d: %s %s", code, out, errOut)
	}
}

func TestAuthorizationDryRunReportsPermissionWithoutApplyingPlan(t *testing.T) {
	path := authorizationFixture(t, readOnlyMetadata)
	planPath := filepath.Join(filepath.Dir(path), "plan.json")
	original := []byte(`{"version":1,"confirm_token":"reviewed","actions":[{"type":"task_add","content":"first"}]}`)
	if err := os.WriteFile(planPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"apply", "run"} {
		code, out, errOut := executeAuthorization(t, path, "agent", command, "--plan", planPath, "--force", "--dry-run", "--json")
		if code != 0 || !strings.Contains(out, `"write_capable": false`) || !strings.Contains(out, `"dry_run": true`) {
			t.Fatalf("preview %d: %s %s", code, out, errOut)
		}
	}
	after, _ := os.ReadFile(planPath)
	if !bytes.Equal(original, after) {
		t.Fatal("preview altered executable plan")
	}
	code, out, errOut := executeAuthorization(t, path, "agent", "status", "--json")
	if code != 0 || !strings.Contains(out, `"authorization"`) {
		t.Fatalf("agent status %d: %s %s", code, out, errOut)
	}
}

func TestAuthorizationSchedulePreservesProfileAndConfigurationWithoutToken(t *testing.T) {
	path := authorizationFixture(t, readOnlyMetadata)
	for _, format := range [][]string{{"--cron"}, {}} {
		args := append([]string{"--profile", "reader", "--base-url", "https://example.test/api/v1", "agent", "schedule", "print", "--weekly", "sat 09:00", "--policy", "policy.json", "--instruction", "review", "--force"}, format...)
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 0 || !strings.Contains(out, "--profile") || !strings.Contains(out, "reader") || !strings.Contains(out, "--config") || !strings.Contains(out, path) || !strings.Contains(out, "--base-url") || !strings.Contains(out, "--policy") || strings.Contains(out, "secret-fixture") {
			t.Fatalf("schedule %d: %s %s", code, out, errOut)
		}
	}
}

func TestAuthorizationPlannerProcess(t *testing.T) {
	if os.Getenv("TODOIST_AUTHORIZATION_PLANNER_HELPER") != "1" {
		return
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	if os.WriteFile(os.Args[len(os.Args)-1], input, 0600) != nil {
		os.Exit(2)
	}
	os.Stdout.Write([]byte(`{"version":1,"confirm_token":"reviewed","actions":[{"type":"task_add","content":"proposed"}]}`))
	os.Exit(0)
}

func TestAuthorizationReadOnlyProfileCanPlanWithAdvisoryPermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("existing external planner uses /bin/sh")
	}
	path := authorizationFixture(t, readOnlyMetadata)
	t.Setenv("TODOIST_AUTHORIZATION_PLANNER_HELPER", "1")
	capture := filepath.Join(filepath.Dir(path), "request.json")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	planner := "'" + strings.ReplaceAll(exe, "'", "'\"'\"'") + "' -test.run=TestAuthorizationPlannerProcess -- '" + strings.ReplaceAll(capture, "'", "'\"'\"'") + "'"
	var mutations atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			mutations.Add(1)
		}
		w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()
	code, out, errOut := executeAuthorization(t, path, "--base-url", server.URL, "agent", "plan", "--planner", planner, "propose a task", "--json")
	if code != 0 {
		t.Fatalf("plan %d: %s %s", code, out, errOut)
	}
	request, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(request, []byte(`"write_capable":false`)) || bytes.Contains(request, []byte("secret-fixture")) || mutations.Load() != 0 {
		t.Fatalf("planner request: %s", request)
	}
	plan, err := os.ReadFile(filepath.Join(filepath.Dir(path), "last_plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(plan, []byte("authorization")) || bytes.Contains(plan, []byte("applied_at")) {
		t.Fatalf("plan contains authority or application: %s", plan)
	}
}

func TestAuthorizationReadOnlyReplaySkipsWithoutNewAppliedAction(t *testing.T) {
	path := authorizationFixture(t, "")
	planPath := filepath.Join(filepath.Dir(path), "plan.json")
	os.WriteFile(planPath, []byte(`{"version":1,"confirm_token":"replay","actions":[{"type":"task_add","content":"once"}]}`), 0600)
	var mutations atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mutations.Add(1); w.Write([]byte(`{}`)) }))
	defer server.Close()
	args := []string{"--base-url", server.URL, "agent", "apply", "--plan", planPath, "--force", "--json"}
	code, out, errOut := executeAuthorization(t, path, args...)
	if code != 0 {
		t.Fatalf("first application %d %s %s", code, out, errOut)
	}
	os.WriteFile(filepath.Join(filepath.Dir(path), "credentials.json"), []byte(`{"profiles":{"default":{"token":"secret-fixture","authorization":`+readOnlyMetadata+`}}}`), 0600)
	code, out, errOut = executeAuthorization(t, path, args...)
	if code != 0 || mutations.Load() != 1 || !strings.Contains(out, "skipped_replay") || strings.Contains(out, "applied_at") {
		t.Fatalf("replay application %d requests=%d %s %s", code, mutations.Load(), out, errOut)
	}
}

func TestAuthorizationSchedulesPreserveDryRunAndForce(t *testing.T) {
	path := authorizationFixture(t, readOnlyMetadata)
	for _, cron := range []bool{false, true} {
		args := []string{"agent", "schedule", "print", "--weekly", "sat 09:00", "--instruction", "review", "--confirm", "reviewed", "--dry-run", "--force", "--bin", "todoist"}
		if cron {
			args = append(args, "--cron")
		}
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 0 || !strings.Contains(out, "--dry-run") || !strings.Contains(out, "--force") {
			t.Errorf("schedule cron=%t lost execution flags: code=%d stdout=%s stderr=%s", cron, code, out, errOut)
		}
	}
}

func TestAuthorizationMachineSchemasPublishSafeReport(t *testing.T) {
	path := authorizationFixture(t, "")
	for _, name := range []string{"authorization", "auth_status", "doctor", "plan_preview", "planner_request"} {
		code, out, errOut := executeAuthorization(t, path, "schema", "--name", name, "--json")
		if code != 0 || !strings.Contains(out, "write_capable") || !strings.Contains(out, "scope_evidence") {
			t.Fatalf("schema %s %d: %s %s", name, code, out, errOut)
		}
	}
	code, out, errOut := executeAuthorization(t, path, "schema", "--name", "error", "--json")
	if code != 0 || !strings.Contains(out, `"code"`) {
		t.Fatalf("error schema %d: %s %s", code, out, errOut)
	}
}

func TestAuthorizationReadOnlyHelpCompletionsAndOrdinaryPreview(t *testing.T) {
	path := authorizationFixture(t, readOnlyMetadata)
	for _, args := range [][]string{{"auth", "login", "--help"}, {"completion", "bash"}, {"completion", "zsh"}, {"completion", "fish"}, {"completion", "powershell"}} {
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 0 || !strings.Contains(out, "read-only") {
			t.Fatalf("missing read-only UX %v %d: %s %s", args, code, out, errOut)
		}
	}
	code, out, errOut := executeAuthorization(t, path, "add", "proposed", "--dry-run", "--json")
	if code != 0 || !strings.Contains(out, `"write_capable": false`) {
		t.Fatalf("ordinary preview %d: %s %s", code, out, errOut)
	}
}

func TestAuthorizationManualReplacementRepairsMetadataAndLogoutRemovesIt(t *testing.T) {
	path := authorizationFixture(t, `{"version":99}`)
	input, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if _, err := input.WriteString("manual-secret\n"); err != nil {
		t.Fatal(err)
	}
	input.Seek(0, 0)
	oldStdin := os.Stdin
	os.Stdin = input
	defer func() { os.Stdin = oldStdin }()
	code, out, errOut := executeAuthorization(t, path, "auth", "login", "--token-stdin", "--json")
	if code != 0 || strings.Contains(out+errOut, "manual-secret") {
		t.Fatalf("replacement %d %s %s", code, out, errOut)
	}
	code, out, errOut = executeAuthorization(t, path, "auth", "status", "--json")
	if code != 0 || !strings.Contains(out, `"mode": "unknown"`) || !strings.Contains(out, `"origin": "manual"`) || !strings.Contains(out, `"effective_scopes": null`) {
		t.Fatalf("manual status %d %s %s", code, out, errOut)
	}
	t.Setenv("TODOIST_TOKEN", "external-secret")
	code, out, errOut = executeAuthorization(t, path, "auth", "logout", "--json")
	if code != 0 || !strings.Contains(out, `"environment_token_active": true`) {
		t.Fatalf("logout %d %s %s", code, out, errOut)
	}
	t.Setenv("TODOIST_TOKEN", "")
	code, out, errOut = executeAuthorization(t, path, "auth", "status", "--json")
	if code != 0 || !strings.Contains(out, `"configured": false`) || !strings.Contains(out, `"mode": null`) {
		t.Fatalf("logged-out status %d %s %s", code, out, errOut)
	}
}

func TestAuthorizationCorruptCredentialFileUsesStructuredError(t *testing.T) {
	path := authorizationFixture(t, "")
	os.WriteFile(filepath.Join(filepath.Dir(path), "credentials.json"), []byte(`{"profiles":`), 0600)
	code, out, errOut := executeAuthorization(t, path, "auth", "status", "--json", "--quiet-json")
	var failure map[string]any
	if code != 3 || out != "" || json.Unmarshal([]byte(errOut), &failure) != nil || failure["error"] == nil || strings.Count(errOut, "\n") != 1 {
		t.Fatalf("corrupt-file error %d %s %s", code, out, errOut)
	}
}

func TestAuthorizationDoctorDoesNotEchoCredentialFromAPIError(t *testing.T) {
	path := authorizationFixture(t, readOnlyMetadata)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`invalid bearer secret-fixture`))
	}))
	defer server.Close()
	_, out, errOut := executeAuthorization(t, path, "--base-url", server.URL, "doctor", "--json")
	if strings.Contains(out+errOut, "secret-fixture") {
		t.Fatalf("doctor exposed credential: %s %s", out, errOut)
	}
}
