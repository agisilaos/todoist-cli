package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialReplacementPreservesOtherProfileMetadataAndUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	initial := []byte(`{"future_root":true,"profiles":{"first":{"token":"old"},"second":{"token":"other","future_field":{"kept":true},"authorization":{"version":99,"future":true}}}}`)
	if err := os.WriteFile(path, initial, 0600); err != nil {
		t.Fatal(err)
	}
	creds, _, err := LoadCredentials(path)
	if err != nil {
		t.Fatal(err)
	}
	creds.Profiles["first"] = Credential{Token: "replacement"}
	if err := SaveCredentials(path, creds); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var saved map[string]any
	json.Unmarshal(data, &saved)
	profiles := saved["profiles"].(map[string]any)
	second := profiles["second"].(map[string]any)
	if saved["future_root"] != true || second["future_field"] == nil || second["authorization"].(map[string]any)["version"] != float64(99) {
		t.Fatalf("lost unrelated fields: %s", data)
	}
	// An encoding failure must not truncate the last readable credential file.
	creds.Profiles["first"] = Credential{Token: "replacement", Authorization: json.RawMessage(`{`)}
	if SaveCredentials(path, creds) == nil {
		t.Fatal("expected encoding error")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(data, after) {
		t.Fatal("failed save modified credentials")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode: %v", info.Mode())
	}
}
