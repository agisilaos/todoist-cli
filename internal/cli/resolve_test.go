package cli

import "testing"

type dummyItem struct {
	Name string
	ID   string
}

func TestResolveFuzzySingle(t *testing.T) {
	items := []dummyItem{{Name: "Work", ID: "1"}, {Name: "Personal", ID: "2"}}
	id, err := resolveFuzzy("pers", items, func(d dummyItem) string { return d.Name }, func(d dummyItem) string { return d.ID })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "2" {
		t.Fatalf("expected ID 2, got %s", id)
	}
}

func TestResolveFuzzyAmbiguous(t *testing.T) {
	items := []dummyItem{{Name: "Work", ID: "1"}, {Name: "Workout", ID: "2"}}
	_, err := resolveFuzzy("work", items, func(d dummyItem) string { return d.Name }, func(d dummyItem) string { return d.ID })
	if err == nil {
		t.Fatalf("expected ambiguity error")
	}
}

func TestUseFuzzyFlag(t *testing.T) {
	ctx := &Context{Fuzzy: true}
	if !useFuzzy(ctx) {
		t.Fatalf("expected fuzzy to be enabled")
	}
}
