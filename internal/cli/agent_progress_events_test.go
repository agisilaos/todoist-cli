package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestAgentApplyDryRunEmitsPlanLoadedAndSummaryEvents(t *testing.T) {
	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.json")
	if err := os.WriteFile(planPath, []byte(`{"version":1,"instruction":"noop","confirm_token":"abcd","summary":{"tasks":0,"projects":0,"sections":0,"labels":0,"comments":0},"actions":[]}`), 0o600); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	var progressOut bytes.Buffer
	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Stdin:  bytes.NewBuffer(nil),
		Mode:   output.ModeJSON,
		Global: GlobalOptions{DryRun: true},
		Now:    time.Now,
		Progress: &progressSink{
			out: &progressOut,
		},
	}
	if err := agentApply(ctx, []string{"--plan", planPath, "--confirm", "abcd"}); err != nil {
		t.Fatalf("agentApply: %v", err)
	}
	lines := progressLines(t, progressOut.String())
	if !containsEvent(lines, "agent_plan_loaded") {
		t.Fatalf("expected agent_plan_loaded event, got %v", eventTypes(lines))
	}
	if !containsEvent(lines, "agent_apply_summary") {
		t.Fatalf("expected agent_apply_summary event, got %v", eventTypes(lines))
	}
}

func TestAgentApplyEmitsActionLifecycleAndSummaryEvents(t *testing.T) {
	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.json")
	if err := os.WriteFile(planPath, []byte(`{"version":1,"instruction":"add","confirm_token":"abcd","summary":{"tasks":1,"projects":0,"sections":0,"labels":0,"comments":0},"actions":[{"type":"task_add","content":"Call mom"}]}`), 0o600); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tasks" && r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"t1","content":"Call mom","project_id":"p1","priority":1}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	var progressOut bytes.Buffer
	ctx := &Context{
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		Stdin:      bytes.NewBuffer(nil),
		Mode:       output.ModeJSON,
		Now:        time.Now,
		Token:      "token",
		Client:     api.NewClient(ts.URL, "token", time.Second),
		Config:     config.Config{TimeoutSeconds: 2},
		ConfigPath: filepath.Join(tmp, ".todoist.json"),
		Progress: &progressSink{
			out: &progressOut,
		},
	}
	if err := agentApply(ctx, []string{"--plan", planPath, "--confirm", "abcd"}); err != nil {
		t.Fatalf("agentApply: %v", err)
	}
	lines := progressLines(t, progressOut.String())
	for _, want := range []string{
		"agent_action_validated",
		"agent_action_dispatched",
		"agent_action_succeeded",
		"agent_apply_summary",
	} {
		if !containsEvent(lines, want) {
			t.Fatalf("expected %s event, got %v", want, eventTypes(lines))
		}
	}
	summary := findEvent(lines, "agent_apply_summary")
	if summary["ok_count"] != float64(1) || summary["failed_count"] != float64(0) {
		t.Fatalf("unexpected summary event: %#v", summary)
	}
}

func progressLines(t *testing.T, raw string) []map[string]any {
	t.Helper()
	out := make([]map[string]any, 0)
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("parse progress line %q: %v", line, err)
		}
		out = append(out, payload)
	}
	return out
}

func containsEvent(lines []map[string]any, eventType string) bool {
	for _, line := range lines {
		if line["type"] == eventType {
			return true
		}
	}
	return false
}

func findEvent(lines []map[string]any, eventType string) map[string]any {
	for _, line := range lines {
		if line["type"] == eventType {
			return line
		}
	}
	return nil
}

func eventTypes(lines []map[string]any) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if value, ok := line["type"].(string); ok {
			out = append(out, value)
		}
	}
	return out
}
