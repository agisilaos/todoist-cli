package cli

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
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
