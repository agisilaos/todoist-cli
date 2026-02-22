package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
)

func TestLoadConfigPrecedence(t *testing.T) {
	tmp := t.TempDir()

	// User config
	userCfgPath := filepath.Join(tmp, "user", "config.json")
	userCfg := config.Config{
		BaseURL:        "https://user",
		TimeoutSeconds: 5,
		DefaultProfile: "user",
	}
	if err := writeJSON(userCfgPath, userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	// Project config
	projectDir := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}
	projectCfgPath := filepath.Join(projectDir, ".todoist.json")
	projectCfg := config.Config{
		BaseURL:        "https://project",
		TimeoutSeconds: 7,
		DefaultProfile: "project",
	}
	if err := writeJSON(projectCfgPath, projectCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	// Env overrides
	t.Setenv("TODOIST_BASE_URL", "https://env")
	t.Setenv("TODOIST_TIMEOUT", "15")

	ctx := &Context{
		Global: GlobalOptions{
			ConfigPath: userCfgPath,
			TimeoutSec: 20, // flags override env
		},
	}

	// Execute in project directory to pick up .todoist.json.
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := loadConfig(ctx); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if ctx.Config.BaseURL != "https://env" {
		t.Fatalf("expected env base_url to win, got %q", ctx.Config.BaseURL)
	}
	if ctx.Config.TimeoutSeconds != 20 {
		t.Fatalf("expected flag timeout to win, got %d", ctx.Config.TimeoutSeconds)
	}
	if ctx.Config.DefaultProfile != "project" {
		t.Fatalf("expected project default profile to win, got %q", ctx.Config.DefaultProfile)
	}
	if ctx.Profile != "project" {
		t.Fatalf("expected profile selection to follow config default, got %q", ctx.Profile)
	}
	if ctx.ConfigPath != userCfgPath {
		t.Fatalf("expected user config path to be stored, got %q", ctx.ConfigPath)
	}
}

func TestResolveProfilePrecedence(t *testing.T) {
	t.Setenv("TODOIST_PROFILE", "env-profile")
	if got := resolveProfile("flag-profile", "default-profile"); got != "flag-profile" {
		t.Fatalf("flag should win, got %q", got)
	}
	if got := resolveProfile("", "default-profile"); got != "env-profile" {
		t.Fatalf("env should win when flag absent, got %q", got)
	}
	t.Setenv("TODOIST_PROFILE", "")
	if got := resolveProfile("", "default-profile"); got != "default-profile" {
		t.Fatalf("default profile should win when env absent, got %q", got)
	}
	if got := resolveProfile("", ""); got != "default" {
		t.Fatalf("fallback should be default, got %q", got)
	}
}

func TestApplyEnvIntAndFlagParser(t *testing.T) {
	v := 10
	t.Setenv("TEST_POSITIVE", "0")
	applyEnvInt("TEST_POSITIVE", &v, true)
	if v != 10 {
		t.Fatalf("positiveOnly should ignore non-positive values")
	}
	t.Setenv("TEST_POSITIVE", "12")
	applyEnvInt("TEST_POSITIVE", &v, true)
	if v != 12 {
		t.Fatalf("expected env int override, got %d", v)
	}
	t.Setenv("TEST_FUZZY", "1")
	if !parsePositiveEnvFlag("TEST_FUZZY") {
		t.Fatalf("expected positive env flag true")
	}
	t.Setenv("TEST_FUZZY", "0")
	if parsePositiveEnvFlag("TEST_FUZZY") {
		t.Fatalf("expected non-positive env flag false")
	}
}

func TestLoadConfigAccessibleEnvAndFlag(t *testing.T) {
	tmp := t.TempDir()
	userCfgPath := filepath.Join(tmp, "config.json")
	if err := writeJSON(userCfgPath, config.Config{}); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("TODOIST_ACCESSIBLE", "1")
	ctx := &Context{Global: GlobalOptions{ConfigPath: userCfgPath}}
	if err := loadConfig(ctx); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if !ctx.Accessible {
		t.Fatalf("expected accessible enabled from env")
	}

	t.Setenv("TODOIST_ACCESSIBLE", "0")
	ctx = &Context{Global: GlobalOptions{ConfigPath: userCfgPath, Accessible: true}}
	if err := loadConfig(ctx); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if !ctx.Accessible {
		t.Fatalf("expected accessible enabled from flag")
	}
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
