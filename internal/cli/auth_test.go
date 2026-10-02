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

	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestAuthLoginOAuthRejectsTokenStdin(t *testing.T) {
	ctx := newAuthTestContext(t)
	err := authLogin(ctx, []string{"--oauth", "--token-stdin"})
	assertUsageErrorContains(t, err, "--token-stdin cannot be used with --oauth")
}

func TestAuthLoginOAuthAndDeviceAreMutuallyExclusive(t *testing.T) {
	ctx := newAuthTestContext(t)
	err := authLogin(ctx, []string{"--oauth", "--oauth-device"})
	assertUsageErrorContains(t, err, "mutually exclusive")
}

func TestAuthLoginOAuthDeviceRejectsTokenStdin(t *testing.T) {
	ctx := newAuthTestContext(t)
	err := authLogin(ctx, []string{"--oauth-device", "--token-stdin"})
	assertUsageErrorContains(t, err, "--token-stdin cannot be used with --oauth-device")
}

func TestAuthLoginOAuthRequiresClientID(t *testing.T) {
	t.Setenv("TODOIST_OAUTH_CLIENT_ID", "")

	ctx := newAuthTestContext(t)
	err := authLogin(ctx, []string{"--oauth"})
	assertUsageErrorContains(t, err, "missing OAuth client id")
}

func TestAuthLoginOAuthStoresToken(t *testing.T) {
	t.Parallel()
	ctx := newAuthTestContext(t)
	restore := stubPerformOAuthLogin(ctx, func(_ *Context, _ oauthConfig) (oauthToken, error) {
		return oauthToken{AccessToken: "oauth-token-123", Authorization: authorization.ManualMetadata()}, nil
	})
	defer restore()

	if err := authLogin(ctx, []string{"--oauth", "--client-id", "client-1"}); err != nil {
		t.Fatalf("authLogin: %v", err)
	}

	credsPath := config.CredentialsPathFromConfig(ctx.ConfigPath)
	cred, err := credentials.New(credsPath, nil, nil).Load(context.Background(), ctx.Profile)
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if _, err := os.Stat(credsPath); err != nil {
		t.Fatalf("expected credentials file to exist")
	}
	got := cred.Token
	if got != "oauth-token-123" {
		t.Fatal("unexpected stored token")
	}
}

func TestAuthLoginOAuthDeviceStoresToken(t *testing.T) {
	t.Parallel()
	ctx := newAuthTestContext(t)
	restore := stubPerformOAuthDeviceLogin(ctx, func(_ *Context, _ oauthConfig) (oauthToken, error) {
		return oauthToken{AccessToken: "oauth-device-token-123", Authorization: authorization.ManualMetadata()}, nil
	})
	defer restore()

	if err := authLogin(ctx, []string{"--oauth-device", "--client-id", "client-1", "--oauth-device-url", "https://provider.example/device"}); err != nil {
		t.Fatalf("authLogin: %v", err)
	}

	credsPath := config.CredentialsPathFromConfig(ctx.ConfigPath)
	cred, err := credentials.New(credsPath, nil, nil).Load(context.Background(), ctx.Profile)
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if _, err := os.Stat(credsPath); err != nil {
		t.Fatalf("expected credentials file to exist")
	}
	got := cred.Token
	if got != "oauth-device-token-123" {
		t.Fatal("unexpected stored token")
	}
}

func TestAuthLoginOAuthPrintEnvDoesNotStore(t *testing.T) {
	ctx := newAuthTestContext(t)
	restore := stubPerformOAuthLogin(ctx, func(_ *Context, _ oauthConfig) (oauthToken, error) {
		return oauthToken{AccessToken: "oauth-token-xyz", Authorization: authorization.ManualMetadata()}, nil
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
	restore := stubPerformOAuthLogin(ctx, func(_ *Context, _ oauthConfig) (oauthToken, error) {
		return oauthToken{AccessToken: "oauth-token-json", Authorization: authorization.ManualMetadata()}, nil
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
	authValidationServer(t, ctx, 200, "stdin-token-123")

	if err := authLogin(ctx, []string{"--token-stdin", "--print-env"}); err != nil {
		t.Fatalf("authLogin: %v", err)
	}
	got := strings.TrimSpace(ctx.Stdout.(*bytes.Buffer).String())
	if !strings.Contains(got, `"env_var":"TODOIST_TOKEN"`) || !strings.Contains(got, `"export":"export TODOIST_TOKEN=stdin-token-123"`) {
		t.Fatalf("unexpected ndjson stdout: %q", got)
	}
}

func TestAuthOAuthLoginContinuesWhenBrowserOpenFails(t *testing.T) {
	t.Parallel()
	ctx := newAuthTestContext(t)
	address := scratchOAuthAddress(t)
	cfg := oauthConfig{
		ClientID:    "client-1",
		ListenAddr:  address,
		RedirectURI: "http://" + address + "/callback",
	}
	restore := stubOAuthFlowDeps(ctx,
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
		func(_ context.Context, _ oauthConfig, _, _ string) (oauthToken, error) {
			return oauthToken{AccessToken: "token-1", Authorization: authorization.ManualMetadata()}, nil
		},
	)
	defer restore()

	token, err := authOAuthLogin(ctx, cfg)
	if err != nil {
		t.Fatalf("authOAuthLogin: %v", err)
	}
	if token.AccessToken != "token-1" {
		t.Fatalf("unexpected token: %q", token.AccessToken)
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
	t.Parallel()
	ctx := newAuthTestContext(t)
	address := scratchOAuthAddress(t)
	cfg := oauthConfig{
		ClientID:    "client-1",
		ListenAddr:  address,
		RedirectURI: "http://" + address + "/callback",
		NoBrowser:   true,
	}
	openCalls := 0
	restore := stubOAuthFlowDeps(ctx,
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
		func(_ context.Context, _ oauthConfig, _, _ string) (oauthToken, error) {
			return oauthToken{AccessToken: "token-1", Authorization: authorization.ManualMetadata()}, nil
		},
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
		local:      localDependencies{credentialStore: testCredentialStore},
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		Stdin:      strings.NewReader(""),
		Config:     config.Config{CredentialStore: "file", TimeoutSeconds: 10},
		Profile:    "default",
		ConfigPath: filepath.Join(tmp, "config.json"),
	}
}

func stubPerformOAuthLogin(ctx *Context, fn func(ctx *Context, cfg oauthConfig) (oauthToken, error)) func() {
	prev := ctx.oauth.login
	ctx.oauth.login = fn
	return func() {
		ctx.oauth.login = prev
	}
}

func stubPerformOAuthDeviceLogin(ctx *Context, fn func(ctx *Context, cfg oauthConfig) (oauthToken, error)) func() {
	prev := ctx.oauth.deviceLogin
	ctx.oauth.deviceLogin = fn
	return func() {
		ctx.oauth.deviceLogin = prev
	}
}

func stubOAuthFlowDeps(
	ctx *Context,
	randomFn func(size int) (string, error),
	authURLFn func(cfg oauthConfig, codeChallenge, state string) (string, error),
	openFn func(url string) error,
	waitFn func(ctx context.Context, cfg oauthConfig, expectedState string, timeout time.Duration) (string, error),
	exchangeFn func(ctx context.Context, cfg oauthConfig, code, codeVerifier string) (oauthToken, error),
) func() {
	prevRandom := ctx.oauth.random
	prevAuthURL := ctx.oauth.authorizationURL
	prevOpen := ctx.oauth.openBrowser
	prevWait := ctx.oauth.waitForCode
	prevExchange := ctx.oauth.exchangeToken
	ctx.oauth.random = randomFn
	ctx.oauth.authorizationURL = authURLFn
	ctx.oauth.openBrowser = openFn
	ctx.oauth.waitForCode = waitFn
	ctx.oauth.exchangeToken = exchangeFn
	return func() {
		ctx.oauth.random = prevRandom
		ctx.oauth.authorizationURL = prevAuthURL
		ctx.oauth.openBrowser = prevOpen
		ctx.oauth.waitForCode = prevWait
		ctx.oauth.exchangeToken = prevExchange
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
