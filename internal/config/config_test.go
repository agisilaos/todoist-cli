package config

import (
	"testing"
)

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
