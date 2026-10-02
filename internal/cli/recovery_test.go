package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestRecoveryMissingCredentialAndUsage(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	t.Setenv("TODOIST_PROFILE", "")
	path := filepath.Join(t.TempDir(), "config.json")
	for _, tc := range []struct {
		args []string
		code int
		hint string
	}{
		{[]string{"today", "--no-input"}, 3, "auth login --no-input --token-stdin"},
		{[]string{"task", "add", "--priority", "p9", "--no-input"}, 2, "todoist task add --help"},
		{[]string{"task", "show", "--invalid", "--no-input"}, 2, "todoist task view --help"},
	} {
		code, out, stderr := executeAuthorization(t, path, tc.args...)
		if code != tc.code || out != "" || !strings.Contains(stderr, tc.hint) {
			t.Fatalf("%v: %d %q %q", tc.args, code, out, stderr)
		}
	}
	// The suggested help works before authentication and does not create state.
	code, out, stderr := executeAuthorization(t, path, "task", "add", "--help")
	if code != 0 || !strings.Contains(out, "--priority") || stderr != "" {
		t.Fatalf("help: %d %q %q", code, out, stderr)
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 0 {
		t.Fatalf("unexpected state: %v %v", files, err)
	}
	for _, mode := range []string{"--json", "--ndjson", "--plain", "--ids-only", "--quiet-json"} {
		code, out, stderr := executeAuthorization(t, path, "today", "--no-input", mode)
		if code != 3 || out != "" || strings.Contains(stderr, "noninteractive") {
			t.Fatalf("machine contract %s: %d %q %q", mode, code, out, stderr)
		}
		if mode == "--json" || mode == "--ids-only" {
			var value map[string]any
			if json.Unmarshal([]byte(stderr), &value) != nil || len(value) != 2 || value["error"] != errMissingToken.Error() {
				t.Fatalf("error envelope: %s", stderr)
			}
		} else if stderr != "error: "+errMissingToken.Error()+"\n" {
			t.Fatalf("text contract: %q", stderr)
		}
	}
}

func TestRecoveryManualLoginGuidanceWorks(t *testing.T) {
	for _, tc := range []struct {
		token        string
		status, code int
		sentinel     error
	}{
		{"invalid token", 200, 2, errInvalidManualToken},
		{"synthetic-rejected", 401, 3, errRejectedManualToken},
	} {
		t.Run(tc.token, func(t *testing.T) {
			ctx := newAuthTestContext(t)
			ctx.Global.NoInput = true
			ctx.Mode = output.ModeHuman
			calls := authValidationServer(t, ctx, tc.status, tc.token)
			ctx.Stdin = strings.NewReader(tc.token)
			err := authLogin(ctx, []string{"--token-stdin", "--credential-store=file"})
			if toExitCode(err) != tc.code || !errors.Is(err, tc.sentinel) {
				t.Fatalf("login: %v", err)
			}
			writeError(ctx, err)
			stderr := ctx.Stderr.(*bytes.Buffer).String()
			if !strings.Contains(stderr, "--no-input --token-stdin") || strings.Contains(stderr, tc.token) {
				t.Fatalf("unsafe diagnostic: %s", stderr)
			}
			if tc.code == 2 && calls.Load() != 0 {
				t.Fatal("invalid input reached API")
			}
			// Follow the printed command with an accepted synthetic credential and explicit file selection.
			authValidationServer(t, ctx, 200, "synthetic-accepted")
			ctx.Stdin = strings.NewReader("synthetic-accepted")
			if err := authLogin(ctx, []string{"--token-stdin", "--credential-store=file"}); err != nil {
				t.Fatal(err)
			}
			saved, err := credentials.New(config.CredentialsPathFromConfig(ctx.ConfigPath), nil, nil).Load(context.Background(), ctx.Profile)
			if err != nil || saved.Token != "synthetic-accepted" {
				t.Fatalf("credential not saved: %v", err)
			}
		})
	}
}

func TestRecoveryAPI401AndOverride(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "synthetic-override")
	t.Setenv("TODOIST_PROFILE", "work")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("unexpected mutation")
		}
		http.Error(w, "synthetic-override", http.StatusUnauthorized)
	}))
	defer server.Close()
	t.Setenv("TODOIST_BASE_URL", server.URL)
	path := filepath.Join(t.TempDir(), "config.json")
	code, out, stderr := executeAuthorization(t, path, "task", "view", "id:fixture", "--no-input")
	if code != 3 || out != "" || !strings.Contains(stderr, "Earlier actions may have succeeded") || strings.Contains(stderr, "synthetic-override") {
		t.Fatalf("401: %d %q %q", code, out, stderr)
	}
	code, out, stderr = executeAuthorization(t, path, "auth", "status", "--no-input", "--json")
	if code != 0 || !strings.Contains(out, `"source": "env"`) || !strings.Contains(out, `"profile": "work"`) || stderr != "" {
		t.Fatalf("status: %d %q %q", code, out, stderr)
	}
}

type unavailableRecoverySecrets struct{ cliSecrets }

func (*unavailableRecoverySecrets) Probe(context.Context) error {
	return &credentials.Error{Kind: credentials.Unavailable}
}

func TestRecoveryUnavailableStorage(t *testing.T) {
	ctx := newAuthTestContext(t)
	ctx.Mode = output.ModeHuman
	ctx.Global.NoInput = true
	ctx.Credentials = credentials.New(config.CredentialsPathFromConfig(ctx.ConfigPath), &unavailableRecoverySecrets{}, nil)
	ctx.Stdin = strings.NewReader("synthetic-token")
	authValidationServer(t, ctx, 200, "synthetic-token")
	// Override the fixture's default file selection to request the unavailable backend.
	err := authLogin(ctx, []string{"--token-stdin", "--credential-store=native"})
	var storage *credentials.Error
	if !errors.As(err, &storage) || storage.Kind != credentials.Unavailable || toExitCode(err) != 3 {
		t.Fatalf("storage: %v", err)
	}
	writeError(ctx, err)
	if !strings.Contains(ctx.Stderr.(*bytes.Buffer).String(), "Only for a new or file-backed profile") {
		t.Fatal("missing storage limits")
	}
	if err := authStatus(ctx); err != nil {
		t.Fatalf("offline status: %v", err)
	}
	authValidationServer(t, ctx, 200, "synthetic-token")
	ctx.Stdin = strings.NewReader("synthetic-token")
	if err := authLogin(ctx, []string{"--token-stdin", "--credential-store=file"}); err != nil {
		t.Fatalf("explicit fallback: %v", err)
	}
}

func TestRecoveryHintsPreserveExplicitMachineErrors(t *testing.T) {
	cases := []error{&CodeError{Code: 3, Err: errMissingToken}, &CodeError{Code: 2, Err: errInvalidManualToken}, &CodeError{Code: 3, Err: errRejectedManualToken}, &credentials.Error{Kind: credentials.Unavailable}, &api.APIError{Status: 401}, &CodeError{Code: 2, Err: errors.New("invalid input")}}
	for _, err := range cases {
		for _, opts := range []GlobalOptions{{JSON: true}, {NDJSON: true}, {Plain: true}, {IDsOnly: true}, {QuietJSON: true}} {
			ctx := newAuthTestContext(t)
			ctx.Global = opts
			ctx.HelpPath = "task add"
			ctx.Mode, _ = output.DetectMode(opts.JSON, opts.Plain, opts.NDJSON, opts.IDsOnly, false)
			writeError(ctx, err)
			got := ctx.Stderr.(*bytes.Buffer).String()
			if opts.JSON || opts.IDsOnly {
				var value map[string]any
				if json.Unmarshal([]byte(got), &value) != nil || value["error"] != err.Error() {
					t.Fatalf("machine error: %s", got)
				}
				wantKeys := 2
				if _, ok := err.(*credentials.Error); ok {
					wantKeys = 4
				}
				if len(value) != wantKeys {
					t.Fatalf("new fields: %s", got)
				}
			} else if got != "error: "+err.Error()+"\n" {
				t.Fatalf("changed text error: %s", got)
			}
		}
	}
}

func TestRecoveryUncertainReviewInspectionDoesNotRepeatMutation(t *testing.T) {
	fixture, ctx := newReviewFixture(t)
	fixture.uncertain = true
	plan := fixtureReviewPlan(t, ctx)
	results, cause := applyReviewPlan(ctx, plan)
	if cause == nil {
		t.Fatal("expected uncertain failure")
	}
	ctx.Mode = output.ModeHuman
	if err := writeReviewReport(ctx, plan, results, "applied", "saved-plan.json", cause); err != nil {
		t.Fatal(err)
	}
	hint := ctx.Stderr.(*bytes.Buffer).String()
	if !strings.Contains(hint, "literal reference: \"id:2\"") || !strings.Contains(hint, "Do not reapply") || !strings.Contains(hint, "completed history") {
		t.Fatalf("guidance: %s", hint)
	}
	before, err := os.ReadFile(replayJournalPath(ctx))
	if err != nil {
		t.Fatal(err)
	}
	count := len(fixture.writes)
	if err := taskView(ctx, []string{"id:2", "--full"}); err != nil {
		t.Fatal(err)
	}
	if len(fixture.writes) != count {
		t.Fatal("inspection mutated")
	}
	if _, err := applyReviewPlan(ctx, plan); toExitCode(err) != 5 {
		t.Fatalf("replay not blocked: %v", err)
	}
	after, err := os.ReadFile(replayJournalPath(ctx))
	if err != nil || !bytes.Equal(before, after) || len(fixture.writes) != count {
		t.Fatal("recovery changed evidence or repeated mutation")
	}
	for _, opts := range []GlobalOptions{{JSON: true}, {NDJSON: true}, {Plain: true}} {
		ctx.Global = opts
		ctx.Mode, _ = output.DetectMode(opts.JSON, opts.Plain, opts.NDJSON, false, false)
		ctx.Stdout = &bytes.Buffer{}
		ctx.Stderr = &bytes.Buffer{}
		if err := writeReviewReport(ctx, plan, results, "applied", "saved-plan.json", cause); err != nil {
			t.Fatal(err)
		}
		if ctx.Stderr.(*bytes.Buffer).Len() != 0 {
			t.Fatal("human hints leaked to machine client")
		}
		if !opts.Plain {
			report := readReviewReport(t, ctx)
			if !report.Tasks[0].Actions[0].RemoteOutcomeUncertain {
				t.Fatal("lost uncertainty")
			}
		}
	}
}

func TestRecoveryMissingNativeCredential(t *testing.T) {
	ctx := newAuthTestContext(t)
	ctx.Global.NoInput = true
	native := &cliSecrets{values: map[string]string{}}
	ctx.Credentials = credentials.New(config.CredentialsPathFromConfig(ctx.ConfigPath), native, nil)
	if err := ctx.Credentials.Save(context.Background(), ctx.Profile, config.Credential{Token: "synthetic-lost"}, "native"); err != nil {
		t.Fatal(err)
	}
	native.values = map[string]string{}
	inspectProfile(ctx)
	err := ensureClient(ctx)
	var storage *credentials.Error
	if !errors.As(err, &storage) || storage.Kind != credentials.Missing {
		t.Fatalf("missing native credential: %v", err)
	}
	writeError(ctx, err)
	if !strings.Contains(ctx.Stderr.(*bytes.Buffer).String(), "auth login --no-input --token-stdin") {
		t.Fatal("no noninteractive recovery")
	}
	if err := authStatus(ctx); err != nil {
		t.Fatal(err)
	}
	authValidationServer(t, ctx, 200, "synthetic-replacement")
	ctx.Stdin = strings.NewReader("synthetic-replacement")
	if err := authLogin(ctx, []string{"--token-stdin"}); err != nil {
		t.Fatal(err)
	}
	saved, err := ctx.Credentials.Load(context.Background(), ctx.Profile)
	if err != nil || saved.Token != "synthetic-replacement" {
		t.Fatalf("replacement not available: %v", err)
	}
}

func TestTaskViewMissingReferenceRecovery(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "")
	t.Setenv("TODOIST_PROFILE", "")
	path := filepath.Join(t.TempDir(), "config.json")
	for _, command := range []string{"view", "show"} {
		code, out, stderr := executeAuthorization(t, path, "task", command, "--no-input")
		want := "error: task view requires id or text reference\nExample: todoist task view id:<id> (replace <id> with a task ID)\nSee: todoist task view --help\n"
		if code != exitUsage || out != "" || stderr != want {
			t.Fatalf("%s: exit=%d stdout=%q stderr=%q", command, code, out, stderr)
		}
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 0 {
		t.Fatalf("unexpected state: %v %v", files, err)
	}
}
