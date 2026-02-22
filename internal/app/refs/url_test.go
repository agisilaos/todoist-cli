package refs

import "testing"

func TestParseTodoistEntityURL(t *testing.T) {
	parsed, ok := ParseTodoistEntityURL("https://app.todoist.com/app/task/call-mom-abc123")
	if !ok {
		t.Fatalf("expected url parse to succeed")
	}
	if parsed.Entity != "task" || parsed.ID != "abc123" {
		t.Fatalf("unexpected parsed value: %#v", parsed)
	}
}

func TestNormalizeEntityRefFromURL(t *testing.T) {
	id, direct, err := NormalizeEntityRef("https://app.todoist.com/app/project/personal-2203306141", "project")
	if err != nil {
		t.Fatalf("NormalizeEntityRef: %v", err)
	}
	if !direct || id != "2203306141" {
		t.Fatalf("unexpected normalize result: id=%q direct=%v", id, direct)
	}
}

func TestNormalizeEntityRefRejectsMismatchedURLType(t *testing.T) {
	_, _, err := NormalizeEntityRef("https://app.todoist.com/app/project/personal-2203306141", "task")
	if err == nil {
		t.Fatalf("expected mismatch error")
	}
}
