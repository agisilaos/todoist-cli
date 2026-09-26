package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
)

func authValidationServer(t *testing.T, ctx *Context, status int, token string) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.URL.Path != "/projects" || r.URL.Query().Get("limit") != "1" {
			t.Errorf("unexpected validation request: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("validation did not use newly supplied credential")
		}
		w.WriteHeader(status)
		if status == 200 {
			_, _ = w.Write([]byte(`{"results":[],"next_cursor":null}`))
		} else {
			_, _ = w.Write([]byte(token))
		}
	}))
	t.Cleanup(server.Close)
	ctx.Config.BaseURL = server.URL
	return &calls
}

func TestManualLoginValidationPreservesCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, token      string
		status, wantCode int
		wantText         string
	}{
		{"spaces", "not a token", 200, 2, "Paste only"},
		{"bearer", "Bearer synthetic-token", 200, 2, "Paste only"},
		{"quotes", `"synthetic-token"`, 200, 2, "Paste only"},
		{"control", "synthetic\x1btoken", 200, 2, "Paste only"},
		{"rejected", "synthetic-invalid-token", 401, 3, "not accepted"},
		{"forbidden", "synthetic-forbidden-token", 403, 3, "not accepted"},
		{"server", "synthetic-server-token", 500, 1, "Could not verify"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newAuthTestContext(t)
			ctx.Stdin = strings.NewReader(tc.token + "\n")
			calls := authValidationServer(t, ctx, tc.status, tc.token)
			if err := profileStore(ctx).Save(context.Background(), ctx.Profile, config.Credential{Token: "previous-token"}, "file"); err != nil {
				t.Fatal(err)
			}
			path := config.CredentialsPathFromConfig(ctx.ConfigPath)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			err = authLogin(ctx, []string{"--token-stdin"})
			var coded *CodeError
			if !errors.As(err, &coded) || coded.Code != tc.wantCode || !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("unexpected result: %v", err)
			}
			if tc.wantCode == 2 && calls.Load() != 0 {
				t.Fatal("malformed token reached API")
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("failed validation replaced stored credentials")
			}
			if ctx.Stdout.(*bytes.Buffer).Len() != 0 || strings.Contains(err.Error(), tc.token) {
				t.Fatal("failure emitted success or leaked candidate token")
			}
		})
	}
}

func TestManualLoginValidatesCandidateBeforeSaving(t *testing.T) {
	t.Setenv("TODOIST_TOKEN", "different-environment-token")
	ctx := newAuthTestContext(t)
	ctx.Stdin = strings.NewReader("synthetic-new-token\n")
	calls := authValidationServer(t, ctx, 200, "synthetic-new-token")
	if err := authLogin(ctx, []string{"--token-stdin"}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("validation requests: %d", calls.Load())
	}
	stored, err := profileStore(ctx).Load(context.Background(), ctx.Profile)
	if err != nil || stored.Token != "synthetic-new-token" {
		t.Fatalf("credential not saved: %v", err)
	}
	out := ctx.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "Connected to Todoist") || strings.Contains(out, "unknown") {
		t.Fatalf("unexpected onboarding: %s", out)
	}
	if !strings.Contains(ctx.Stderr.(*bytes.Buffer).String(), "TODOIST_TOKEN") {
		t.Fatal("missing environment override warning")
	}
}

func TestManualLoginFailureDoesNotCreateOrExportCredentials(t *testing.T) {
	for _, args := range [][]string{{"--token-stdin"}, {"--token-stdin", "--print-env"}} {
		ctx := newAuthTestContext(t)
		ctx.Stdin = strings.NewReader("synthetic-rejected-token\n")
		authValidationServer(t, ctx, 401, "synthetic-rejected-token")
		err := authLogin(ctx, args)
		var coded *CodeError
		if !errors.As(err, &coded) || coded.Code != exitAuth {
			t.Fatalf("unexpected result: %v", err)
		}
		if _, err := os.Stat(config.CredentialsPathFromConfig(ctx.ConfigPath)); !os.IsNotExist(err) {
			t.Fatal("failed login created credentials")
		}
		if ctx.Stdout.(*bytes.Buffer).Len() != 0 {
			t.Fatal("failed login exported a rejected token")
		}
	}
}

func TestManualLoginUnreachableEndpointDoesNotSaveCredentials(t *testing.T) {
	ctx := newAuthTestContext(t)
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	ctx.Config.BaseURL = server.URL
	server.Close()
	ctx.Stdin = strings.NewReader("synthetic-unreachable-token\n")
	err := authLogin(ctx, []string{"--token-stdin"})
	var coded *CodeError
	if !errors.As(err, &coded) || coded.Code != exitError || !strings.Contains(err.Error(), "Could not verify") {
		t.Fatalf("unexpected result: %v", err)
	}
	if _, err := os.Stat(config.CredentialsPathFromConfig(ctx.ConfigPath)); !os.IsNotExist(err) {
		t.Fatal("unverified credential was saved")
	}
}
