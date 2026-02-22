package projects

import "testing"

func TestBuildAddPayload(t *testing.T) {
	body, err := BuildAddPayload(AddInput{Name: "Home", Description: "Personal", Favorite: true})
	if err != nil {
		t.Fatalf("BuildAddPayload: %v", err)
	}
	if body["name"] != "Home" || body["description"] != "Personal" || body["is_favorite"] != true {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestBuildAddPayloadRequiresName(t *testing.T) {
	if _, err := BuildAddPayload(AddInput{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildUpdatePayload(t *testing.T) {
	id, body, err := BuildUpdatePayload(UpdateInput{ID: "p1", Name: "Work"})
	if err != nil {
		t.Fatalf("BuildUpdatePayload: %v", err)
	}
	if id != "p1" || body["name"] != "Work" {
		t.Fatalf("unexpected output: id=%q body=%#v", id, body)
	}
}

func TestBuildUpdatePayloadRequiresFields(t *testing.T) {
	if _, _, err := BuildUpdatePayload(UpdateInput{ID: "p1"}); err == nil {
		t.Fatalf("expected error")
	}
}
