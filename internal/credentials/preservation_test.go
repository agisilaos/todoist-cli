package credentials_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
)

func TestCredentialReplacementPreservesOtherProfileMetadataAndUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	initial := []byte(`{"future_root":true,"profiles":{"first":{"token":"old"},"second":{"token":"other","future_field":{"kept":true},"authorization":{"version":99,"future":true}}}}`)
	if err := os.WriteFile(path, initial, 0600); err != nil {
		t.Fatal(err)
	}
	s := credentials.New(path, nil, nil)
	ctx := context.Background()
	if err := s.Save(ctx, "first", config.Credential{Token: "replacement"}, "file"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	profiles := saved["profiles"].(map[string]any)
	second := profiles["second"].(map[string]any)
	if saved["future_root"] != true || second["future_field"] == nil || second["authorization"].(map[string]any)["version"] != float64(99) {
		t.Fatalf("lost unrelated fields: %s", data)
	}
	// An encoding failure must not truncate the last readable credential file.
	if s.Save(ctx, "first", config.Credential{Token: "replacement", Authorization: json.RawMessage(`{`)}, "file") == nil {
		t.Fatal("expected encoding error")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, after) {
		t.Fatal("failed save modified credentials")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode: %v", info.Mode())
	}
}
