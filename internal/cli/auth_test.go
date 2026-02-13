package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestAuthLoginOAuthRejectsTokenStdin(t *testing.T) {
	ctx := newAuthTestContext(t)
	err := authLogin(ctx, []string{"--oauth", "--token-stdin"})
	assertUsageErrorContains(t, err, "--token-stdin cannot be used with --oauth")
}

func TestAuthLoginOAuthRequiresClientID(t *testing.T) {
	t.Setenv("TODOIST_OAUTH_CLIENT_ID", "")

	ctx := newAuthTestContext(t)
	err := authLogin(ctx, []string{"--oauth"})
	assertUsageErrorContains(t, err, "missing OAuth client id")
}

func TestAuthLoginOAuthStoresToken(t *testing.T) {
	ctx := newAuthTestContext(t)
	restore := stubPerformOAuthLogin(func(_ *Context, _ oauthConfig) (string, error) {
		return "oauth-token-123", nil
	})
	defer restore()

	if err := authLogin(ctx, []string{"--oauth", "--client-id", "client-1"}); err != nil {
		t.Fatalf("authLogin: %v", err)
	}

	credsPath := config.CredentialsPathFromConfig(ctx.ConfigPath)
	creds, exists, err := config.LoadCredentials(credsPath)
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if !exists {
		t.Fatalf("expected credentials file to exist")
	}
	got := creds.Profiles[ctx.Profile].Token
	if got != "oauth-token-123" {
		t.Fatalf("unexpected stored token: %q", got)
	}
}

func TestAuthLoginOAuthPrintEnvDoesNotStore(t *testing.T) {
	ctx := newAuthTestContext(t)
	restore := stubPerformOAuthLogin(func(_ *Context, _ oauthConfig) (string, error) {
		return "oauth-token-xyz", nil
	})
	defer restore()

	if err := authLogin(ctx, []string{"--oauth", "--client-id", "client-1", "--print-env"}); err != nil {
		t.Fatalf("authLogin: %v", err)
	}

	if got := ctx.Stdout.(*bytes.Buffer).String(); !strings.Contains(got, "export TODOIST_TOKEN=oauth-token-xyz") {
		t.Fatalf("unexpected stdout: %q", got)
	}

	credsPath := config.CredentialsPathFromConfig(ctx.ConfigPath)
	if _, err := os.Stat(credsPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no credentials file, stat err=%v", err)
	}
}

func TestAuthLoginOAuthPrintEnvJSONMode(t *testing.T) {
	ctx := newAuthTestContext(t)
	ctx.Mode = output.ModeJSON
	restore := stubPerformOAuthLogin(func(_ *Context, _ oauthConfig) (string, error) {
		return "oauth-token-json", nil
	})
	defer restore()

	if err := authLogin(ctx, []string{"--oauth", "--client-id", "client-1", "--print-env"}); err != nil {
		t.Fatalf("authLogin: %v", err)
	}
	got := ctx.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(got, `"env_var": "TODOIST_TOKEN"`) || !strings.Contains(got, `"export": "export TODOIST_TOKEN=oauth-token-json"`) {
		t.Fatalf("unexpected json stdout: %q", got)
	}
}

func TestAuthLoginTokenStdinPrintEnvNDJSONMode(t *testing.T) {
	ctx := newAuthTestContext(t)
	ctx.Mode = output.ModeNDJSON
	ctx.Stdin = strings.NewReader("stdin-token-123\n")

	if err := authLogin(ctx, []string{"--token-stdin", "--print-env"}); err != nil {
		t.Fatalf("authLogin: %v", err)
	}
	got := strings.TrimSpace(ctx.Stdout.(*bytes.Buffer).String())
	if !strings.Contains(got, `"env_var":"TODOIST_TOKEN"`) || !strings.Contains(got, `"export":"export TODOIST_TOKEN=stdin-token-123"`) {
		t.Fatalf("unexpected ndjson stdout: %q", got)
	}
}

func TestAuthOAuthLoginContinuesWhenBrowserOpenFails(t *testing.T) {
	ctx := newAuthTestContext(t)
	cfg := oauthConfig{
		ClientID:    "client-1",
		RedirectURI: "http://127.0.0.1:8765/callback",
	}
	restore := stubOAuthFlowDeps(
		func(size int) (string, error) {
			if size == 32 {
				return "verifier-1", nil
			}
			return "state-1", nil
		},
		func(_ oauthConfig, _, _ string) (string, error) { return "https://auth.example/authorize", nil },
		func(_ string) error { return fmt.Errorf("open failed") },
		func(_ context.Context, _ oauthConfig, _ string, _ time.Duration) (string, error) {
			return "code-1", nil
		},
		func(_ context.Context, _ oauthConfig, _, _ string) (string, error) { return "token-1", nil },
	)
	defer restore()

	token, err := authOAuthLogin(ctx, cfg)
	if err != nil {
		t.Fatalf("authOAuthLogin: %v", err)
	}
	if token != "token-1" {
		t.Fatalf("unexpected token: %q", token)
	}
	stderr := ctx.Stderr.(*bytes.Buffer).String()
	if !strings.Contains(stderr, "warning: could not open browser automatically") {
		t.Fatalf("expected browser warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "Open the OAuth authorization URL manually to continue.") {
		t.Fatalf("expected manual-open guidance, got %q", stderr)
	}
}

func TestAuthOAuthLoginNoBrowserSkipsBrowserOpen(t *testing.T) {
	ctx := newAuthTestContext(t)
	cfg := oauthConfig{
		ClientID:    "client-1",
		RedirectURI: "http://127.0.0.1:8765/callback",
		NoBrowser:   true,
	}
	openCalls := 0
	restore := stubOAuthFlowDeps(
		func(size int) (string, error) {
			if size == 32 {
				return "verifier-1", nil
			}
			return "state-1", nil
		},
		func(_ oauthConfig, _, _ string) (string, error) { return "https://auth.example/authorize", nil },
		func(_ string) error {
			openCalls++
			return nil
		},
		func(_ context.Context, _ oauthConfig, _ string, _ time.Duration) (string, error) {
			return "code-1", nil
		},
		func(_ context.Context, _ oauthConfig, _, _ string) (string, error) { return "token-1", nil },
	)
	defer restore()

	if _, err := authOAuthLogin(ctx, cfg); err != nil {
		t.Fatalf("authOAuthLogin: %v", err)
	}
	if openCalls != 0 {
		t.Fatalf("expected no browser open calls, got %d", openCalls)
	}
}

func newAuthTestContext(t *testing.T) *Context {
	t.Helper()
	tmp := t.TempDir()
	return &Context{
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		Stdin:      strings.NewReader(""),
		Profile:    "default",
		ConfigPath: filepath.Join(tmp, "config.json"),
	}
}

func stubPerformOAuthLogin(fn func(ctx *Context, cfg oauthConfig) (string, error)) func() {
	prev := performOAuthLogin
	performOAuthLogin = fn
	return func() {
		performOAuthLogin = prev
	}
}

func stubOAuthFlowDeps(
	randomFn func(size int) (string, error),
	authURLFn func(cfg oauthConfig, codeChallenge, state string) (string, error),
	openFn func(url string) error,
	waitFn func(ctx context.Context, cfg oauthConfig, expectedState string, timeout time.Duration) (string, error),
	exchangeFn func(ctx context.Context, cfg oauthConfig, code, codeVerifier string) (string, error),
) func() {
	prevRandom := generateOAuthRandomFn
	prevAuthURL := buildOAuthAuthorizationURLFn
	prevOpen := openOAuthBrowserFn
	prevWait := waitForOAuthCodeFn
	prevExchange := exchangeOAuthTokenFn
	generateOAuthRandomFn = randomFn
	buildOAuthAuthorizationURLFn = authURLFn
	openOAuthBrowserFn = openFn
	waitForOAuthCodeFn = waitFn
	exchangeOAuthTokenFn = exchangeFn
	return func() {
		generateOAuthRandomFn = prevRandom
		buildOAuthAuthorizationURLFn = prevAuthURL
		openOAuthBrowserFn = prevOpen
		waitForOAuthCodeFn = prevWait
		exchangeOAuthTokenFn = prevExchange
	}
}

func assertUsageErrorContains(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error")
	}
	var codeErr *CodeError
	if !errors.As(err, &codeErr) {
		t.Fatalf("expected CodeError, got %T", err)
	}
	if codeErr.Code != exitUsage {
		t.Fatalf("expected exitUsage, got %d", codeErr.Code)
	}
	if !strings.Contains(err.Error(), contains) {
		t.Fatalf("expected %q in error, got %v", contains, err)
	}
}
