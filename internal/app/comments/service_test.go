package comments

import "testing"

func TestBuildAddPayload(t *testing.T) {
	body, err := BuildAddPayload(AddInput{Content: "hello", TaskID: "t1"})
	if err != nil {
		t.Fatalf("BuildAddPayload: %v", err)
	}
	if body["content"] != "hello" || body["task_id"] != "t1" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestBuildAddPayloadRequiresTarget(t *testing.T) {
	if _, err := BuildAddPayload(AddInput{Content: "hello"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildUpdatePayload(t *testing.T) {
	id, body, err := BuildUpdatePayload(UpdateInput{ID: "c1", Content: "new"})
	if err != nil {
		t.Fatalf("BuildUpdatePayload: %v", err)
	}
	if id != "c1" || body["content"] != "new" {
		t.Fatalf("unexpected output: id=%q body=%#v", id, body)
	}
}

func TestValidateList(t *testing.T) {
	if err := ValidateList(ListInput{}); err == nil {
		t.Fatalf("expected error")
	}
}
