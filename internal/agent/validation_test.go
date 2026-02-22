package agent

import "testing"

func TestValidatePlanAllowsNoopWhenAllowed(t *testing.T) {
	plan := Plan{Version: 1, ConfirmToken: "abcd", Actions: nil}
	if err := ValidatePlan(plan, 1, true); err != nil {
		t.Fatalf("ValidatePlan: %v", err)
	}
}

func TestValidatePlanRejectsUnsupportedAction(t *testing.T) {
	plan := Plan{Version: 1, ConfirmToken: "abcd", Actions: []Action{{Type: "unknown_action"}}}
	if err := ValidatePlan(plan, 1, false); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSummarizeActions(t *testing.T) {
	s := SummarizeActions([]Action{{Type: "task_add"}, {Type: "project_add"}, {Type: "comment_add"}})
	if s.Tasks != 1 || s.Projects != 1 || s.Comments != 1 {
		t.Fatalf("unexpected summary: %#v", s)
	}
}
