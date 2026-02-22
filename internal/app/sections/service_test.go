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
