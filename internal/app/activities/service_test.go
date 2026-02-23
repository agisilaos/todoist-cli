package activities

import "testing"

func TestNormalizeListInput(t *testing.T) {
	out, err := NormalizeListInput(ListInput{Type: "TASK", Limit: 500})
	if err != nil {
		t.Fatalf("NormalizeListInput: %v", err)
	}
	if out.Type != "task" || out.Limit != 100 {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestNormalizeListInputRejectsInvalidType(t *testing.T) {
	if _, err := NormalizeListInput(ListInput{Type: "workspace"}); err == nil {
		t.Fatalf("expected type validation error")
	}
}

func TestBuildQueryMapsTaskType(t *testing.T) {
	query := BuildQuery(ListInput{
		Since:     "2026-02-01",
		Until:     "2026-02-23",
		Type:      "task",
		Event:     "added",
		ProjectID: "p1",
		By:        "u1",
		Limit:     20,
		Cursor:    "abc",
	})
	if query.Get("object_type") != "item" || query.Get("event_type") != "added" || query.Get("parent_project_id") != "p1" || query.Get("initiator_id") != "u1" || query.Get("limit") != "20" {
		t.Fatalf("unexpected query: %v", query.Encode())
	}
}
