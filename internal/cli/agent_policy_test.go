package cli

import "testing"

func TestEnforceAgentPolicyAllowDeny(t *testing.T) {
	plan := Plan{
		Actions: []Action{
			{Type: "task_add"},
			{Type: "task_delete"},
		},
	}
	policy := &agentPolicy{
		AllowActionTypes: []string{"task_add"},
	}
	if err := enforceAgentPolicy(plan, policy); err == nil {
		t.Fatalf("expected allow-list enforcement error")
	}

	policy = &agentPolicy{
		DenyActionTypes: []string{"task_delete"},
	}
	if err := enforceAgentPolicy(plan, policy); err == nil {
		t.Fatalf("expected deny-list enforcement error")
	}
}

func TestEnforceAgentPolicyMaxDestructive(t *testing.T) {
	plan := Plan{
		Actions: []Action{
			{Type: "task_delete"},
			{Type: "project_delete"},
		},
	}
	policy := &agentPolicy{MaxDestructiveActions: 1}
	if err := enforceAgentPolicy(plan, policy); err == nil {
		t.Fatalf("expected max destructive enforcement error")
	}
}
