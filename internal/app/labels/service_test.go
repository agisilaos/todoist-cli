package labels

import "testing"

func TestBuildListQueryDefaultsLimit(t *testing.T) {
	query := BuildListQuery(ListInput{})
	if got := query.Get("limit"); got != "50" {
		t.Fatalf("expected default limit 50, got %q", got)
	}
}

func TestBuildAddPayload(t *testing.T) {
	body, err := BuildAddPayload(AddInput{Name: "urgent", Favorite: true})
	if err != nil {
		t.Fatalf("BuildAddPayload: %v", err)
	}
	if body["name"] != "urgent" || body["is_favorite"] != true {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestBuildUpdatePayloadRequiresFields(t *testing.T) {
	if _, _, err := BuildUpdatePayload(UpdateInput{ID: "l1"}); err == nil {
		t.Fatalf("expected error")
	}
}
