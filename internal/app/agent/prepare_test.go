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
			return coreagent.Plan{ConfirmToken: "abcd", Actions: []coreagent.Action{{Type: "task_add", Content: "Do"}}}, nil
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
			return coreagent.Plan{ConfirmToken: "abcd", Actions: []coreagent.Action{{Type: "task_add", Content: "Do"}}}, nil
		},
		EnforcePolicy: func(plan coreagent.Plan) error {
			return errors.New("blocked")
		},
	})
	if err == nil || err.Error() != "blocked" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreparePlanAlwaysValidatesBeforePolicyEvenWithForce(t *testing.T) {
	for _, source := range []string{"file", "planner"} {
		t.Run(source, func(t *testing.T) {
			policyCalled := false
			invalid := coreagent.Plan{ConfirmToken: "abcd", Actions: []coreagent.Action{{Type: "unsupported"}}}
			deps := PrepareDeps{
				LoadPlan:      func(string) (coreagent.Plan, error) { return invalid, nil },
				Plan:          func(string) (coreagent.Plan, error) { return invalid, nil },
				EnforcePolicy: func(coreagent.Plan) error { policyCalled = true; return nil },
			}
			in := PrepareInput{Instruction: "Do", Force: true}
			if source == "file" {
				in.PlanPath = "plan.json"
			}
			_, err := PreparePlan(in, deps)
			var validation *PlanValidationError
			if !errors.As(err, &validation) || err.Error() != "unsupported action type: unsupported" || policyCalled {
				t.Fatalf("validation skipped or misclassified: %v, policy=%v", err, policyCalled)
			}
		})
	}
}

func TestPreparePlanPreservesLoaderErrorClassification(t *testing.T) {
	want := errors.New("read failed")
	_, err := PreparePlan(PrepareInput{PlanPath: "plan.json"}, PrepareDeps{LoadPlan: func(string) (coreagent.Plan, error) { return coreagent.Plan{}, want }})
	var validation *PlanValidationError
	if !errors.Is(err, want) || errors.As(err, &validation) {
		t.Fatalf("loader error reclassified: %v", err)
	}
}
