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

func TestFuzzyCandidatesRanksAndCapsResults(t *testing.T) {
	items := []dummyItem{
		{Name: "Homework", ID: "1"},
		{Name: "Work", ID: "2"},
		{Name: "Workshop", ID: "3"},
		{Name: "Network", ID: "4"},
		{Name: "Worx", ID: "5"},
		{Name: "World", ID: "6"},
		{Name: "Word", ID: "7"},
		{Name: "Worm", ID: "8"},
		{Name: "Workstream", ID: "9"},
		{Name: "Working Group", ID: "10"},
	}
	got := fuzzyCandidates("work", items, func(d dummyItem) string { return d.Name }, func(d dummyItem) string { return d.ID })
	if len(got) == 0 {
		t.Fatalf("expected fuzzy candidates")
	}
	if got[0].Name != "Work" {
		t.Fatalf("expected exact match first, got %q", got[0].Name)
	}
	if len(got) > 8 {
		t.Fatalf("expected at most 8 candidates, got %d", len(got))
	}
}
