package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlannerSetPreservesUserConfiguration(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.json")
	initial := `{"default_profile":"personal","timeout_seconds":10,"future_setting":{"enabled":true},"PLANNER_CMD":"old"}`
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".todoist.json"), []byte(`{"default_profile":"work","default_inbox_due":"tomorrow"}`), 0600); err != nil {
		t.Fatal(err)
	}
	env := Environment{Getenv: func(key string) string {
		switch key {
		case "TODOIST_TOKEN":
			return "synthetic-planner-config-token"
		case "TODOIST_TIMEOUT":
			return "27"
		default:
			return ""
		}
	}}
	t.Chdir(project)
	var out, errOut bytes.Buffer
	code := executeTestWithEnvironment([]string{"--config", path, "agent", "planner", "--set", "--cmd", "cat", "--json"}, &out, &errOut, env)
	if code != 0 {
		t.Fatalf("set planner exit=%d stderr=%s", code, errOut.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"default_profile": "personal", "timeout_seconds": float64(10), "future_setting": map[string]any{"enabled": true}, "planner_cmd": "cat"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("setting planner changed unrelated user configuration: got %s; want only planner_cmd=cat with personal profile and original fields", data)
	}
	t.Chdir(root)
	out.Reset()
	errOut.Reset()
	code = executeTestWithEnvironment([]string{"--config", path, "profile", "current", "--json"}, &out, &errOut, env)
	if code != 0 {
		t.Fatalf("inspect profile exit=%d stderr=%s", code, errOut.String())
	}
	var current struct {
		SelectedProfile string `json:"selected_profile"`
	}
	if err := json.Unmarshal(out.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current.SelectedProfile != "personal" {
		t.Errorf("profile outside project=%q, want personal", current.SelectedProfile)
	}
}
