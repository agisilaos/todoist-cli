package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSetDefaultProfilePreservesConfigurationFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	before := []byte(`{"DEFAULT_PROFILE":"old","base_url":"https://user.example","credential_store":"file","future":{"value":17},"planner_cmd":"literal"}`)
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	if err := SetDefaultProfile(path, "next", nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	var future struct {
		Value int `json:"value"`
	}
	if err := json.Unmarshal(fields["future"], &future); err != nil {
		t.Fatal(err)
	}
	if string(fields["default_profile"]) != `"next"` || fields["DEFAULT_PROFILE"] != nil || future.Value != 17 || string(fields["base_url"]) != `"https://user.example"` {
		t.Fatalf("selection changed unrelated configuration: %s", data)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("configuration permissions: %v", err)
	}
	staging, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".todoist-config-stage-*"))
	if len(staging) != 0 {
		t.Fatal("configuration staging was retained")
	}
}

type selectionFailure struct {
	SelectionDisk
	after bool
}

func (f selectionFailure) Write(path string, data []byte) error {
	if f.after {
		if err := f.SelectionDisk.Write(path, data); err != nil {
			return err
		}
	}
	return errors.New("untrusted persistence failure")
}

func TestSetDefaultProfileReportsPersistenceUncertainty(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-replace", true: "after-replace"}[after], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			before := []byte(`{"default_profile":"old","future":true}`)
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			err := SetDefaultProfile(path, "next", selectionFailure{after: after})
			var failure *SelectionError
			if !errors.As(err, &failure) || !failure.Uncertain {
				t.Fatalf("failure did not report uncertainty: %v", err)
			}
			data, _ := os.ReadFile(path)
			if !after && !bytes.Equal(data, before) {
				t.Fatal("failed replacement changed configuration")
			}
			if after && !bytes.Contains(data, []byte(`"next"`)) {
				t.Fatal("fixture did not publish replacement")
			}
		})
	}
}

func TestSetDefaultProfilePreservesInvalidConfiguration(t *testing.T) {
	for _, before := range []string{`null`, `[]`, `{"timeout_seconds":"bad"}`, `{"default_profile":`} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(before), 0600); err != nil {
			t.Fatal(err)
		}
		if err := SetDefaultProfile(path, "next", nil); err == nil {
			t.Fatal("invalid configuration was replaced")
		}
		data, _ := os.ReadFile(path)
		if string(data) != before {
			t.Fatal("invalid configuration was changed")
		}
	}
}

func TestSetDefaultProfileCreatesOnlySelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := SetDefaultProfile(path, "scratch", nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil || len(fields) != 1 || fields["default_profile"] != "scratch" {
		t.Fatalf("unexpected new config: %s", data)
	}
}
