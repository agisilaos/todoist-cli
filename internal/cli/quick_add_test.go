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

func TestBuildQuickAddText(t *testing.T) {
	text, err := buildQuickAddText("Buy milk", "Home", []string{"errands"}, 4, "tomorrow")
	if err != nil {
		t.Fatalf("build text: %v", err)
	}
	if text != "Buy milk #Home @errands p1 tomorrow" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestBuildQuickAddTextRejectsProjectID(t *testing.T) {
	if _, err := buildQuickAddText("Buy milk", "id:123", nil, 0, ""); err == nil {
		t.Fatalf("expected error for project id")
	}
}

func TestPriorityFlagAcceptsP1(t *testing.T) {
	var p priorityFlag
	if err := p.Set("p1"); err != nil {
		t.Fatalf("set p1: %v", err)
	}
	if int(p) != 4 {
		t.Fatalf("expected priority 4, got %d", p)
	}
}

func TestPriorityFlagRejectsNumeric(t *testing.T) {
	var p priorityFlag
	if err := p.Set("1"); err == nil {
		t.Fatalf("expected error for numeric priority")
	}
}
