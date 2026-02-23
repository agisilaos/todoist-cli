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

func TestResolveReferenceExactByName(t *testing.T) {
	got, err := ResolveReference(ResolveReferenceInput{
		Ref: "Today",
		References: []Reference{
			{ID: "f1", Name: "Today"},
			{ID: "f2", Name: "Next"},
		},
	})
	if err != nil {
		t.Fatalf("ResolveReference: %v", err)
	}
	if got.ResolvedID != "f1" || got.NotFound || len(got.Ambiguous) != 0 {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestResolveReferenceFromURL(t *testing.T) {
	got, err := ResolveReference(ResolveReferenceInput{
		Ref: "https://app.todoist.com/app/filter/today-f1",
		References: []Reference{
			{ID: "f1", Name: "Today"},
		},
	})
	if err != nil {
		t.Fatalf("ResolveReference: %v", err)
	}
	if got.ResolvedID != "f1" || !got.DirectID {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestResolveReferenceFuzzyAmbiguous(t *testing.T) {
	got, err := ResolveReference(ResolveReferenceInput{
		Ref:         "tod",
		EnableFuzzy: true,
		References: []Reference{
			{ID: "f1", Name: "Today"},
			{ID: "f2", Name: "Today Focus"},
		},
	})
	if err != nil {
		t.Fatalf("ResolveReference: %v", err)
	}
	if len(got.Ambiguous) != 2 || got.NotFound || got.ResolvedID != "" {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestResolveReferenceNotFoundDirectID(t *testing.T) {
	got, err := ResolveReference(ResolveReferenceInput{
		Ref: "id:f9",
		References: []Reference{
			{ID: "f1", Name: "Today"},
		},
	})
	if err != nil {
		t.Fatalf("ResolveReference: %v", err)
	}
	if !got.NotFound || !got.DirectID {
		t.Fatalf("unexpected result: %#v", got)
	}
}
