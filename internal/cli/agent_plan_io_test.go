package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestWritePlanApplyResultHumanSummary(t *testing.T) {
	var out bytes.Buffer
	ctx := &Context{Stdout: &out}
	plan := Plan{Instruction: "Triage inbox"}
	results := []applyResult{
		{Action: Action{Type: "task_add"}},
		{Action: Action{Type: "task_delete"}, Error: errors.New("api failure")},
		{Action: Action{Type: "task_add"}, SkippedReplay: true},
	}
	if err := writePlanApplyResult(ctx, plan, results, nil); err != nil {
		t.Fatalf("writePlanApplyResult: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Applied plan: Triage inbox",
		"Summary: actions=3 ok=1 failed=1 skipped_replay=1",
		"Risk: destructive_actions=1",
		"By action type:",
		"  - task_add: 2",
		"  - task_delete: 1",
		"Results:",
		"1. task_add [ok]",
		"2. task_delete [error: api failure]",
		"3. task_add [skipped (replay)]",
		"Outcome: success",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestWritePlanApplyResultHumanErrorOutcome(t *testing.T) {
	var out bytes.Buffer
	ctx := &Context{Stdout: &out}
	plan := Plan{Instruction: "Triage inbox"}
	results := []applyResult{{Action: Action{Type: "task_add"}, Error: errors.New("boom")}}
	applyErr := errors.New("apply halted")
	if err := writePlanApplyResult(ctx, plan, results, applyErr); err == nil || err.Error() != "apply halted" {
		t.Fatalf("expected apply error, got %v", err)
	}
	if !strings.Contains(out.String(), "Outcome: completed with error: apply halted") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}
