package agent

import (
	"errors"
	"testing"

	coreagent "github.com/agisilaos/todoist-cli/internal/agent"
)

func TestPreparePlanRequiresInstructionWithoutPlanPath(t *testing.T) {
	_, err := PreparePlan(PrepareInput{}, PrepareDeps{})
	if err == nil || err.Error() != "instruction is required when --plan is not provided" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreparePlanLoadsAndConfirms(t *testing.T) {
	plan, err := PreparePlan(PrepareInput{
		PlanPath: "x.json",
		Confirm:  "abcd",
	}, PrepareDeps{
		LoadPlan: func(path string) (coreagent.Plan, error) {
			return coreagent.Plan{ConfirmToken: "abcd"}, nil
		},
	})
	if err != nil {
		t.Fatalf("PreparePlan: %v", err)
	}
	if plan.ConfirmToken != "abcd" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestPreparePlanEnforcesPolicy(t *testing.T) {
	_, err := PreparePlan(PrepareInput{
		Instruction: "x",
		Confirm:     "abcd",
	}, PrepareDeps{
		Plan: func(instruction string) (coreagent.Plan, error) {
			return coreagent.Plan{ConfirmToken: "abcd"}, nil
		},
		EnforcePolicy: func(plan coreagent.Plan) error {
			return errors.New("blocked")
		},
	})
	if err == nil || err.Error() != "blocked" {
		t.Fatalf("unexpected error: %v", err)
	}
}
