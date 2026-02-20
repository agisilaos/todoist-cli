package cli

import (
	"bytes"
	"encoding/json"
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
		{ID: "123456789", Content: "Write launch blog", ProjectID: "proj1", SectionID: "sec1", Labels: []string{"focus", "writing"}, Priority: 4, Checked: false, Due: &api.Due{Date: "2026-01-01"}},
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
	want := `[
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
]
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
	lines := strings.Split(strings.TrimSpace(ctx.Stdout.(*bytes.Buffer).String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 ndjson lines, got %d", len(lines))
	}
	var got1 map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got1); err != nil {
		t.Fatalf("invalid ndjson line 1: %v", err)
	}
	var got2 map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &got2); err != nil {
		t.Fatalf("invalid ndjson line 2: %v", err)
	}
	if got1["id"] != "1" || got2["id"] != "2" {
		t.Fatalf("unexpected ndjson ids: %#v %#v", got1["id"], got2["id"])
	}
	if _, ok := got1["priority"]; !ok {
		t.Fatalf("line 1 missing priority field")
	}
	if _, ok := got2["priority"]; !ok {
		t.Fatalf("line 2 missing priority field")
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

func TestWriteErrorJSONQuietSingleLine(t *testing.T) {
	ctx := &Context{
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeJSON,
		Global: GlobalOptions{QuietJSON: true},
	}
	writeError(ctx, errors.New("boom"))
	got := strings.TrimSpace(ctx.Stderr.(*bytes.Buffer).String())
	want := `{"error":"boom","meta":{}}`
	if got != want {
		t.Fatalf("unexpected quiet json error: %q", got)
	}
}

func TestWriteErrorJSONUsesAPIRequestID(t *testing.T) {
	ctx := &Context{
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeJSON,
	}
	writeError(ctx, &api.APIError{Status: 500, Message: "boom", RequestID: "req-api-1"})
	got := ctx.Stderr.(*bytes.Buffer).String()
	want := `{
  "error": "api error: status 500: boom",
  "meta": {
    "request_id": "req-api-1"
  }
}
`
	if got != want {
		t.Fatalf("unexpected json api error: %q", got)
	}
}

func TestWriteErrorJSONIncludesAmbiguousDetails(t *testing.T) {
	ctx := &Context{
		Stderr: &bytes.Buffer{},
		Mode:   output.ModeJSON,
	}
	err := &CodeError{
		Code: exitUsage,
		Err: &AmbiguousMatchError{
			Entity:  "project",
			Input:   "hom",
			Matches: []string{"Home", "Homework"},
		},
	}
	writeError(ctx, err)
	got := ctx.Stderr.(*bytes.Buffer).String()
	if !strings.Contains(got, `"details"`) || !strings.Contains(got, `"ambiguous_match"`) || !strings.Contains(got, `"project"`) {
		t.Fatalf("expected ambiguity details, got %q", got)
	}
}
