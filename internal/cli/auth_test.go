package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
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
