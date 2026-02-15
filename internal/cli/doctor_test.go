package cli

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestDoctorCommandJSON(t *testing.T) {
	dir := t.TempDir()
	ctx := &Context{
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		Mode:       output.ModeJSON,
		ConfigPath: filepath.Join(dir, "config.json"),
		Profile:    "default",
	}
	if err := doctorCommand(ctx, nil); err != nil {
		t.Fatalf("doctorCommand: %v", err)
	}
	got := ctx.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(got, `"checks"`) || !strings.Contains(got, `"summary"`) {
		t.Fatalf("unexpected doctor json output: %q", got)
	}
}

func TestDoctorCommandStrictFailsOnWarnings(t *testing.T) {
	dir := t.TempDir()
	ctx := &Context{
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		Mode:       output.ModeJSON,
		ConfigPath: filepath.Join(dir, "config.json"),
		Profile:    "default",
	}
	err := doctorCommand(ctx, []string{"--strict"})
	if err == nil {
		t.Fatalf("expected strict doctor to fail on warnings")
	}
	var codeErr *CodeError
	if !strings.Contains(err.Error(), "doctor checks failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errors.As(err, &codeErr) || codeErr.Code != exitError {
		t.Fatalf("expected CodeError exitError, got %T %#v", err, codeErr)
	}
}

func TestDoctorCommandFailsWhenAPIProbeFails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer ts.Close()

	dir := t.TempDir()
	ctx := &Context{
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		Mode:       output.ModeHuman,
		ConfigPath: filepath.Join(dir, "config.json"),
		Profile:    "default",
		Token:      "token",
		Client:     api.NewClient(ts.URL, "token", time.Second),
		Config:     config.Config{TimeoutSeconds: 1},
	}
	err := doctorCommand(ctx, nil)
	if err == nil {
		t.Fatalf("expected API failure to return error")
	}
	if !strings.Contains(err.Error(), "doctor checks failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckAPIConnectivitySuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"p1","name":"Home"}],"next_cursor":""}`))
	}))
	defer ts.Close()

	ctx := &Context{
		Token:  "token",
		Client: api.NewClient(ts.URL, "token", time.Second),
		Config: config.Config{TimeoutSeconds: 1},
	}
	check := checkAPIConnectivity(ctx)
	if check.Status != "ok" {
		t.Fatalf("expected ok status, got %#v", check)
	}
}

func TestCheckCredentialsWarnsOnInsecurePermissions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	credsPath := config.CredentialsPathFromConfig(configPath)
	if err := os.WriteFile(credsPath, []byte(`{"profiles":{"default":{"token":"x"}}}`), 0o644); err != nil {
		t.Fatalf("write creds: %v", err)
	}

	ctx := &Context{
		ConfigPath:  configPath,
		Profile:     "default",
		Token:       "token",
		TokenSource: "credentials",
	}
	check := checkCredentials(ctx)
	if check.Status != "warn" {
		t.Fatalf("expected warn status, got %#v", check)
	}
	if !strings.Contains(check.Message, "permissions") {
		t.Fatalf("expected permission warning, got %#v", check)
	}
}

func TestCheckPlannerSetupWarnsOnMissingBinary(t *testing.T) {
	ctx := &Context{Config: config.Config{PlannerCmd: "totally-missing-planner-bin --flag"}}
	check := checkPlannerSetup(ctx)
	if check.Status != "warn" {
		t.Fatalf("expected warn status, got %#v", check)
	}
	if !strings.Contains(check.Message, "not found") {
		t.Fatalf("expected missing binary message, got %#v", check)
	}
}

func TestCheckPolicyFileParseFailure(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	policyPath := filepath.Join(dir, "agent_policy.json")
	if err := os.WriteFile(policyPath, []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	ctx := &Context{ConfigPath: configPath}
	check := checkPolicyFile(ctx)
	if check.Status != "fail" {
		t.Fatalf("expected fail status, got %#v", check)
	}
	if !strings.Contains(check.Message, "parse failed") {
		t.Fatalf("expected parse failed message, got %#v", check)
	}
}
