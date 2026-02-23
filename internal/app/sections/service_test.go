package sections

import "testing"

func TestBuildListQueryIncludesProjectID(t *testing.T) {
	query := BuildListQuery(ListInput{ProjectID: "p1"})
	if got := query.Get("project_id"); got != "p1" {
		t.Fatalf("expected project_id p1, got %q", got)
	}
}

func TestBuildAddPayloadRequiresFields(t *testing.T) {
	if _, err := BuildAddPayload(AddInput{Name: "S"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildUpdatePayload(t *testing.T) {
	id, body, err := BuildUpdatePayload(UpdateInput{ID: "s1", Name: "Now"})
	if err != nil {
		t.Fatalf("BuildUpdatePayload: %v", err)
	}
	if id != "s1" || body["name"] != "Now" {
		t.Fatalf("unexpected output id=%q body=%#v", id, body)
	}
}

func TestBuildDeletePlan(t *testing.T) {
	id, requiresConfirm, err := BuildDeletePlan(DeleteInput{ID: "s1"})
	if err != nil {
		t.Fatalf("BuildDeletePlan: %v", err)
	}
	if id != "s1" || !requiresConfirm {
		t.Fatalf("unexpected output: id=%q requiresConfirm=%v", id, requiresConfirm)
	}
}

func TestBuildDeletePlanDryRunSkipsConfirm(t *testing.T) {
	_, requiresConfirm, err := BuildDeletePlan(DeleteInput{ID: "s1", DryRun: true})
	if err != nil {
		t.Fatalf("BuildDeletePlan: %v", err)
	}
	if requiresConfirm {
		t.Fatalf("expected no confirm in dry-run")
	}
}

func TestBuildDeletePlanRequiresID(t *testing.T) {
	if _, _, err := BuildDeletePlan(DeleteInput{}); err == nil {
		t.Fatalf("expected error")
	}
}
