package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	creds := Credentials{Profiles: map[string]Credential{"default": {Token: "abc"}}}
	if err := SaveCredentials(path, creds); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	loaded, found, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if !found {
		t.Fatalf("expected credentials to be found")
	}
	if loaded.Profiles["default"].Token != "abc" {
		t.Fatalf("unexpected token: %q", loaded.Profiles["default"].Token)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected permissions: %v", info.Mode().Perm())
	}
}

func TestMergeConfig(t *testing.T) {
	base := Config{BaseURL: "https://example.com", TimeoutSeconds: 5, DefaultProfile: "base"}
	override := Config{TimeoutSeconds: 10, DefaultProfile: "override"}
	merged := MergeConfig(base, override)
	if merged.BaseURL != "https://example.com" {
		t.Fatalf("expected base_url to persist")
	}
	if merged.TimeoutSeconds != 10 {
		t.Fatalf("expected timeout override")
	}
	if merged.DefaultProfile != "override" {
		t.Fatalf("expected profile override")
	}
}
