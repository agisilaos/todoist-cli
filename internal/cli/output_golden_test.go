package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestWriteTaskListPlainSnapshot(t *testing.T) {
	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Mode:   output.ModePlain,
		Config: config.Config{TableWidth: 80},
	}
	tasks := []api.Task{
		{ID: "123456789", Content: "Write launch blog", ProjectID: "proj1", SectionID: "sec1", Labels: []string{"focus", "writing"}, Priority: 4, Checked: false, Due: map[string]interface{}{"date": "2026-01-01"}},
		{ID: "987654321", Content: "Fix bug in CLI", ProjectID: "proj2", SectionID: "", Labels: nil, Priority: 2, Checked: true},
	}
	if err := writeTaskList(ctx, tasks, "", false); err != nil {
		t.Fatalf("writeTaskList: %v", err)
	}
	got := ctx.Stdout.(*bytes.Buffer).String()
	want := strings.TrimSpace("123456789\tWrite launch blog\tproj1\tsec1\tfocus,writing\t2026-01-01\t4\tno\n987654321\tFix bug in CLI\tproj2\t\t\t\t2\tyes") + "\n"
	if got != want {
		t.Fatalf("unexpected plain output:\n%s", got)
	}
}

func TestWriteTaskListJSONSnapshot(t *testing.T) {
	ctx := &Context{
		Stdout:    &bytes.Buffer{},
		Mode:      output.ModeJSON,
		Config:    config.Config{TableWidth: 80},
		RequestID: "req-1",
	}
	tasks := []api.Task{
		{ID: "1", Content: "A", ProjectID: "p", SectionID: "s", Labels: []string{}, Priority: 1},
	}
	if err := writeTaskList(ctx, tasks, "next", false); err != nil {
		t.Fatalf("writeTaskList: %v", err)
	}
	got := ctx.Stdout.(*bytes.Buffer).String()
	want := `{
  "data": [
    {
      "id": "1",
      "content": "A",
      "description": "",
      "project_id": "p",
      "section_id": "s",
      "parent_id": "",
      "labels": [],
      "priority": 1,
      "checked": false,
      "due": null,
      "added_at": "",
      "completed_at": "",
      "updated_at": "",
      "note_count": 0
    }
  ],
  "meta": {
    "request_id": "req-1",
    "count": 1,
    "next_cursor": "next"
  }
}
`
	if got != want {
		t.Fatalf("unexpected JSON output:\n%s", got)
	}
}

func TestWriteTaskListNDJSONSnapshot(t *testing.T) {
	ctx := &Context{
		Stdout: &bytes.Buffer{},
		Mode:   output.ModeNDJSON,
	}
	tasks := []api.Task{
		{ID: "1", Content: "A", ProjectID: "p", SectionID: "s", Labels: []string{}, Priority: 1},
		{ID: "2", Content: "B", ProjectID: "p", SectionID: "", Labels: []string{"x"}, Priority: 2},
	}
	if err := writeTaskList(ctx, tasks, "", false); err != nil {
		t.Fatalf("writeTaskList: %v", err)
	}
	got := ctx.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(got, `"id":"1"`) || !strings.Contains(got, `"id":"2"`) {
		t.Fatalf("unexpected ndjson output: %q", got)
	}
}

func TestWriteErrorHuman(t *testing.T) {
	ctx := &Context{
		Stderr:    &bytes.Buffer{},
		Mode:      output.ModeHuman,
		RequestID: "req-123",
	}
	writeError(ctx, errors.New("boom"))
	got := ctx.Stderr.(*bytes.Buffer).String()
	want := "error: boom (request_id=req-123)\n"
	if got != want {
		t.Fatalf("unexpected error output: %q", got)
	}
}

func TestWriteErrorJSON(t *testing.T) {
	ctx := &Context{
		Stderr:    &bytes.Buffer{},
		Mode:      output.ModeJSON,
		RequestID: "req-456",
	}
	writeError(ctx, errors.New("boom"))
	got := ctx.Stderr.(*bytes.Buffer).String()
	want := `{
  "error": "boom",
  "meta": {
    "request_id": "req-456"
  }
}
`
	if got != want {
		t.Fatalf("unexpected json error: %q", got)
	}
}
