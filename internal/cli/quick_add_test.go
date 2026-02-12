package cli

import (
	"bytes"
	"strings"
	"testing"
)

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

func TestPriorityFlagAcceptsNumeric(t *testing.T) {
	var p priorityFlag
	if err := p.Set("1"); err != nil {
		t.Fatalf("set numeric: %v", err)
	}
	if int(p) != 1 {
		t.Fatalf("expected priority 1, got %d", p)
	}
}

func TestPriorityFlagRejectsInvalid(t *testing.T) {
	var p priorityFlag
	if err := p.Set("p9"); err == nil {
		t.Fatalf("expected error for invalid priority")
	}
}

func TestQuickAddCommandRejectsSectionWithoutStrict(t *testing.T) {
	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Stdin:  strings.NewReader(""),
	}
	err := quickAddCommand(ctx, []string{"--content", "Buy milk", "--section", "Errands"})
	if err == nil || !strings.Contains(err.Error(), "--section is only supported with --strict") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQuickAddCommandRejectsProjectIDWithoutStrict(t *testing.T) {
	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Stdin:  strings.NewReader(""),
	}
	err := quickAddCommand(ctx, []string{"--content", "Buy milk", "--project", "id:123"})
	if err == nil || !strings.Contains(err.Error(), "project by id is not supported with quick add") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQuickAddCommandRejectsMixedStrictSyntax(t *testing.T) {
	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Stdin:  strings.NewReader(""),
	}
	err := quickAddCommand(ctx, []string{"--content", "Buy milk", "--strict", "--project", "#Home"})
	if err == nil || !strings.Contains(err.Error(), "cannot start with '#'") {
		t.Fatalf("unexpected error: %v", err)
	}
}
