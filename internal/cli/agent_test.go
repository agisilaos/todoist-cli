package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestValidatePlanMissingConfirm(t *testing.T) {
	plan := Plan{
		Actions: []Action{{Type: "task_add", Content: "x"}},
	}
	if err := validatePlan(plan, 1, false); err == nil {
		t.Fatalf("expected error for missing confirm_token")
	}
}

func TestValidatePlanAllowsNoActionsWhenExplicitlyAllowed(t *testing.T) {
	plan := Plan{
		ConfirmToken: "abcd",
		Actions:      nil,
	}
	if err := validatePlan(plan, 1, true); err != nil {
		t.Fatalf("expected no-op plan to validate when allowed, got %v", err)
	}
}

func TestWritePlanPreviewJSON(t *testing.T) {
	plan := Plan{
		Instruction:  "do it",
		ConfirmToken: "abcd",
		Actions:      []Action{{Type: "task_add", Content: "x"}},
	}
	var buf bytes.Buffer
	ctx := &Context{Stdout: &buf, Mode: output.ModeJSON}
	if err := writePlanPreview(ctx, plan, true); err != nil {
		t.Fatalf("writePlanPreview: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, `"dry_run": true`) {
		t.Fatalf("expected dry_run true in JSON, got %q", got)
	}
}

func TestWritePlanPreviewHumanIncludesReason(t *testing.T) {
	plan := Plan{
		Instruction:  "triage",
		ConfirmToken: "abcd",
		Summary:      PlanSummary{Tasks: 1},
		Actions:      []Action{{Type: "task_add", Reason: "overdue inbox cleanup"}},
	}
	var buf bytes.Buffer
	ctx := &Context{Stdout: &buf}
	if err := writePlanPreview(ctx, plan, false); err != nil {
		t.Fatalf("writePlanPreview: %v", err)
	}
	if !strings.Contains(buf.String(), "task_add (overdue inbox cleanup)") {
		t.Fatalf("expected reason in preview output, got %q", buf.String())
	}
}

func TestAgentStatusNoPlanJSON(t *testing.T) {
	tmp := t.TempDir()
	var out bytes.Buffer
	ctx := &Context{
		Stdout:     &out,
		Stderr:     io.Discard,
		Mode:       output.ModeJSON,
		ConfigPath: filepath.Join(tmp, "config.json"),
		Config:     config.Config{PlannerCmd: ""},
	}
	if err := agentStatus(ctx); err != nil {
		t.Fatalf("agentStatus: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"last_plan_exists": false`) {
		t.Fatalf("expected empty-state status output, got %q", got)
	}
}

func TestReadPlanFileNotFoundReturnsUsageError(t *testing.T) {
	_, err := readPlanFile("/tmp/definitely-missing-plan-for-test.json", nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	var codeErr *CodeError
	if !errors.As(err, &codeErr) {
		t.Fatalf("expected CodeError, got %T", err)
	}
	if codeErr.Code != exitUsage {
		t.Fatalf("expected exitUsage, got %d", codeErr.Code)
	}
	if !strings.Contains(err.Error(), "plan file not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPlannerMissingPlannerGuidance(t *testing.T) {
	ctx := &Context{
		Config: config.Config{TimeoutSeconds: 10},
		Now:    time.Now,
	}
	_, err := runPlanner(ctx, "", "do it", 1, plannerContextOptions{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "todoist agent planner --set --cmd") {
		t.Fatalf("expected actionable guidance, got %v", err)
	}
}

func TestAgentApplyDryRunAllowsNoActionPlan(t *testing.T) {
	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.json")
	if err := os.WriteFile(planPath, []byte(`{"version":1,"instruction":"noop","confirm_token":"abcd","summary":{"tasks":0,"projects":0,"sections":0,"labels":0,"comments":0},"actions":[]}`), 0o600); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	var out bytes.Buffer
	ctx := &Context{
		Stdout: &out,
		Stderr: io.Discard,
		Stdin:  bytes.NewBuffer(nil),
		Mode:   output.ModeJSON,
		Global: GlobalOptions{DryRun: true},
		Now:    time.Now,
	}
	if err := agentApply(ctx, []string{"--plan", planPath, "--confirm", "abcd"}); err != nil {
		t.Fatalf("agentApply: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"dry_run": true`) || !strings.Contains(got, `"action_count": 0`) {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestAgentExecutionPreservesUsageHelpDifferences(t *testing.T) {
	for _, command := range []string{"apply", "run"} {
		t.Run(command, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.json")
			if err := os.WriteFile(path, []byte(`{"version":1,"confirm_token":"abcd","actions":[{"type":"unsupported"}]}`), 0o600); err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			ctx := &Context{Stdout: &out, Stderr: &stderr, Stdin: strings.NewReader(""), Global: GlobalOptions{DryRun: true}, Now: time.Now}
			err := agentCommand(ctx, []string{command, "--plan", path, "--confirm", "abcd"})
			var codeErr *CodeError
			if !errors.As(err, &codeErr) || codeErr.Code != exitUsage {
				t.Fatalf("validation classification changed: %v", err)
			}
			if (stderr.Len() > 0) != (command == "apply") || out.Len() != 0 {
				t.Fatalf("command-specific help changed: stdout=%q stderr=%q", out.String(), stderr.String())
			}
		})
	}
}

func TestAgentRunExportsPlanBeforeDryRun(t *testing.T) {
	dir := t.TempDir()
	path, exported := filepath.Join(dir, "plan.json"), filepath.Join(dir, "export.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"confirm_token":"abcd","actions":[{"type":"task_add","content":"Do"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := &Context{Stdout: &out, Stderr: io.Discard, Stdin: strings.NewReader(""), Global: GlobalOptions{DryRun: true}, Mode: output.ModeJSON, Now: time.Now}
	if err := agentRun(ctx, []string{"--plan", path, "--confirm", "abcd", "--out", exported}); err != nil {
		t.Fatal(err)
	}
	plan, err := readPlanFile(exported, nil)
	if err != nil || plan.ConfirmToken != "abcd" || len(plan.Actions) != 1 || plan.AppliedAt != "" {
		t.Fatalf("dry-run export changed: plan=%#v err=%v", plan, err)
	}
	if !strings.Contains(out.String(), `"dry_run": true`) || ctx.Client != nil {
		t.Fatalf("dry run acquired client or lost preview: %s", out.String())
	}
}
