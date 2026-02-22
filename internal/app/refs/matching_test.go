package refs

import "testing"

type dummy struct {
	Name string
	ID   string
}

func TestNormalizeRefDirectID(t *testing.T) {
	got, direct := NormalizeRef("id:123")
	if !direct || got != "123" {
		t.Fatalf("unexpected output: %q %v", got, direct)
	}
}

func TestResolveFuzzyAmbiguous(t *testing.T) {
	_, candidates := ResolveFuzzy("work", []dummy{
		{Name: "Work", ID: "1"},
		{Name: "Workout", ID: "2"},
	}, func(d dummy) string { return d.Name }, func(d dummy) string { return d.ID })
	if len(candidates) < 2 {
		t.Fatalf("expected ambiguous candidates, got %#v", candidates)
	}
}

func TestFuzzyCandidatesExactFirst(t *testing.T) {
	out := FuzzyCandidates("work", []dummy{
		{Name: "Workshop", ID: "2"},
		{Name: "Work", ID: "1"},
	}, func(d dummy) string { return d.Name }, func(d dummy) string { return d.ID })
	if len(out) == 0 || out[0].Name != "Work" {
		t.Fatalf("unexpected candidates: %#v", out)
	}
}
