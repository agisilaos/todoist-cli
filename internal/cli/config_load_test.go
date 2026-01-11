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
