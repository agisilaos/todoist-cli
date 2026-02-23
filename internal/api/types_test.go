package api

import (
	"encoding/json"
	"testing"
)

func TestActivityEventUnmarshalJSONPreservesLargeNumericIDs(t *testing.T) {
	var event ActivityEvent
	payload := []byte(`{
		"id": 2141952848445000199,
		"event_type": "completed",
		"event_date": "2026-02-23T10:00:00Z",
		"object_type": "item",
		"object_id": 999999999999999999,
		"parent_project_id": 888888888888888888,
		"initiator_id": 777777777777777777
	}`)
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if event.ID != "2141952848445000199" {
		t.Fatalf("expected precise id string, got %q", event.ID)
	}
	if event.ObjectID != "999999999999999999" {
		t.Fatalf("expected precise object_id string, got %q", event.ObjectID)
	}
}
