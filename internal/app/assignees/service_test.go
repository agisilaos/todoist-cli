package assignees

import "testing"

func TestParseRef(t *testing.T) {
	got := ParseRef("id:123")
	if got.ID != "123" || got.IsMe || got.NeedsLookup {
		t.Fatalf("unexpected parsed ref: %#v", got)
	}
}

func TestMatchCollaboratorID(t *testing.T) {
	id, candidates, found := MatchCollaboratorID("ada@example.com", []Collaborator{
		{ID: "u1", Name: "Ada Lovelace", Email: "ada@example.com"},
	})
	if !found || id != "u1" || len(candidates) != 0 {
		t.Fatalf("unexpected match: id=%q candidates=%#v found=%v", id, candidates, found)
	}
}

func TestMatchCollaboratorIDAmbiguous(t *testing.T) {
	id, candidates, found := MatchCollaboratorID("ada", []Collaborator{
		{ID: "u1", Name: "Ada Lovelace", Email: "ada@example.com"},
		{ID: "u2", Name: "Ada Byron", Email: "ab@example.com"},
	})
	if found || id != "" || len(candidates) < 2 {
		t.Fatalf("unexpected result: id=%q candidates=%#v found=%v", id, candidates, found)
	}
}
