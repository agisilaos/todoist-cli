package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestValidatePlanMissingConfirm(t *testing.T) {
	plan := Plan{
		Actions: []Action{{Type: "task_add", Content: "x"}},
	}
	if err := validatePlan(plan, 1); err == nil {
		t.Fatalf("expected error for missing confirm_token")
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
