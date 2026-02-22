package filters

import "testing"

func TestBuildAddPayload(t *testing.T) {
	body, err := BuildAddPayload(AddInput{Name: "Today", Query: "today", Favorite: true})
	if err != nil {
		t.Fatalf("BuildAddPayload: %v", err)
	}
	if body["name"] != "Today" || body["query"] != "today" || body["is_favorite"] != true {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestBuildUpdatePayload(t *testing.T) {
	ref, body, err := BuildUpdatePayload(UpdateInput{Ref: "id:f1", Query: "today & @focus"})
	if err != nil {
		t.Fatalf("BuildUpdatePayload: %v", err)
	}
	if ref != "id:f1" || body["query"] != "today & @focus" {
		t.Fatalf("unexpected output: ref=%q body=%#v", ref, body)
	}
}

func TestValidateDeleteRequiresYesOrForce(t *testing.T) {
	if _, err := ValidateDelete(DeleteInput{Ref: "f1"}); err == nil {
		t.Fatalf("expected error")
	}
}
