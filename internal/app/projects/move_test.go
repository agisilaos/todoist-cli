package projects

import "testing"

func TestBuildMovePlanWorkspace(t *testing.T) {
	plan, err := BuildMovePlan(MoveInput{
		Ref:         "Home",
		ToWorkspace: "Acme Corp",
		Visibility:  "TEAM",
	})
	if err != nil {
		t.Fatalf("BuildMovePlan: %v", err)
	}
	if plan.Ref != "Home" || plan.ToWorkspace != "Acme Corp" || plan.Visibility != "team" || plan.ToPersonal {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestBuildMovePlanPersonal(t *testing.T) {
	plan, err := BuildMovePlan(MoveInput{
		Ref:        "id:p1",
		ToPersonal: true,
	})
	if err != nil {
		t.Fatalf("BuildMovePlan: %v", err)
	}
	if !plan.ToPersonal || plan.ToWorkspace != "" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestBuildMovePlanRequiresRef(t *testing.T) {
	if _, err := BuildMovePlan(MoveInput{ToPersonal: true}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildMovePlanRequiresSingleTarget(t *testing.T) {
	if _, err := BuildMovePlan(MoveInput{Ref: "p1"}); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := BuildMovePlan(MoveInput{Ref: "p1", ToWorkspace: "w1", ToPersonal: true}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildMovePlanValidatesVisibility(t *testing.T) {
	if _, err := BuildMovePlan(MoveInput{Ref: "p1", ToWorkspace: "w1", Visibility: "invalid"}); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := BuildMovePlan(MoveInput{Ref: "p1", ToPersonal: true, Visibility: "team"}); err == nil {
		t.Fatalf("expected error")
	}
}
