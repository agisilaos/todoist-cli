package cli

import "testing"

func TestParseQuickAdd(t *testing.T) {
	got := parseQuickAdd("Buy milk #Home @errands p2 due:tomorrow")
	if got.Content != "Buy milk" {
		t.Fatalf("content: %q", got.Content)
	}
	if got.Project != "Home" {
		t.Fatalf("project: %q", got.Project)
	}
	if got.Priority != 3 {
		t.Fatalf("priority: %d", got.Priority)
	}
	if got.Due != "tomorrow" {
		t.Fatalf("due: %q", got.Due)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "errands" {
		t.Fatalf("labels: %#v", got.Labels)
	}
}
