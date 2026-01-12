package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultConfigFile      = "config.json"
	defaultCredentialsFile = "credentials.json"
	projectConfigFile      = ".todoist.json"
)

type Config struct {
	BaseURL            string   `json:"base_url"`
	TimeoutSeconds     int      `json:"timeout_seconds"`
	DefaultProfile     string   `json:"default_profile"`
	DefaultInboxLabels []string `json:"default_inbox_labels"`
	DefaultInboxDue    string   `json:"default_inbox_due"`
	TableWidth         int      `json:"table_width"`
	PlannerCmd         string   `json:"planner_cmd"`
}

type Credentials struct {
	Profiles map[string]Credential `json:"profiles"`
}

type Credential struct {
	Token string `json:"token"`
}

func DefaultUserConfigPath() (string, error) {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, "todoist", defaultConfigFile), nil
}

func DefaultProjectConfigPath(cwd string) string {
	return filepath.Join(cwd, projectConfigFile)
}

func CredentialsPathFromConfig(configPath string) string {
	dir := filepath.Dir(configPath)
	return filepath.Join(dir, defaultCredentialsFile)
}

func LoadConfig(path string) (Config, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false, nil
		}
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, true, fmt.Errorf("parse config: %w", err)
	}
	return cfg, true, nil
}

func LoadCredentials(path string) (Credentials, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Credentials{}, false, nil
		}
		return Credentials{}, false, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, true, fmt.Errorf("parse credentials: %w", err)
	}
	if creds.Profiles == nil {
		creds.Profiles = map[string]Credential{}
	}
	return creds, true, nil
}

func SaveCredentials(path string, creds Credentials) error {
	if creds.Profiles == nil {
		creds.Profiles = map[string]Credential{}
	}
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func EnsureDir(path string) error {
	if path == "" || path == "." {
		return nil
	}
	return os.MkdirAll(path, 0o700)
}

func SaveConfig(path string, cfg Config) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func MergeConfig(base Config, override Config) Config {
	result := base
	if override.BaseURL != "" {
		result.BaseURL = override.BaseURL
	}
	if override.TimeoutSeconds > 0 {
		result.TimeoutSeconds = override.TimeoutSeconds
	}
	if override.DefaultProfile != "" {
		result.DefaultProfile = override.DefaultProfile
	}
	if len(override.DefaultInboxLabels) > 0 {
		result.DefaultInboxLabels = override.DefaultInboxLabels
	}
	if override.DefaultInboxDue != "" {
		result.DefaultInboxDue = override.DefaultInboxDue
	}
	if override.TableWidth > 0 {
		result.TableWidth = override.TableWidth
	}
	if override.PlannerCmd != "" {
		result.PlannerCmd = override.PlannerCmd
	}
	return result
}
