package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCorruptConfigLeavesHelpAndDoctorAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected API request: %s", r.URL.Path)
		http.Error(w, "unexpected", 500)
	}))
	defer server.Close()
	t.Setenv("TODOIST_TOKEN", "synthetic-corrupt-config-token")
	t.Setenv("TODOIST_BASE_URL", server.URL)
	path := filepath.Join(t.TempDir(), "config.json")
	original := []byte(`{not valid JSON`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"--help"}, {"help", "task"}, {"task", "help"}, {"auth", "repair", "--help"}, {"auth", "help", "login"}, {"doctor", "--help"}} {
		code, out, errOut := executeAuthorization(t, path, args...)
		if code != 0 || !strings.Contains(out, "Usage:") || errOut != "" {
			t.Errorf("%v: exit=%d stdout=%q stderr=%q", args, code, out, errOut)
		}
	}
	for _, mode := range []string{"--json", "--ndjson"} {
		code, out, errOut := executeAuthorization(t, path, "doctor", mode)
		if code != 1 {
			t.Fatalf("doctor exit=%d stdout=%q stderr=%q", code, out, errOut)
		}
		var report struct {
			Checks []doctorCheck `json:"checks"`
		}
		if err := json.Unmarshal([]byte(out), &report); err != nil {
			t.Fatal(err)
		}
		if len(report.Checks) != 6 || report.Checks[0].Status != "fail" || !strings.Contains(out, path) {
			t.Fatalf("missing config diagnostic: %s", out)
		}
		for _, check := range report.Checks[1:] {
			if !strings.Contains(check.Message, "skipped") {
				t.Errorf("dependent check was not skipped: %+v", check)
			}
		}
	}
	code, out, _ := executeAuthorization(t, path, "task", "add", "--content", "Blocked")
	if code == 0 || out != "" {
		t.Fatalf("mutation unexpectedly proceeded: exit=%d stdout=%q", code, out)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(original) {
		t.Fatalf("config changed: %s (%v)", data, err)
	}
}
