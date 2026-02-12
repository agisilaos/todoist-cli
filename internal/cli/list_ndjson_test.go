package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

func TestWriteProjectListNDJSON(t *testing.T) {
	ctx := &Context{Stdout: &bytes.Buffer{}, Mode: output.ModeNDJSON}
	if err := writeProjectList(ctx, []api.Project{{ID: "p1", Name: "Proj"}}, ""); err != nil {
		t.Fatalf("writeProjectList: %v", err)
	}
	assertNDJSONLineHasField(t, ctx.Stdout.(*bytes.Buffer).String(), "id", "p1")
}

func TestWriteSectionListNDJSON(t *testing.T) {
	ctx := &Context{Stdout: &bytes.Buffer{}, Mode: output.ModeNDJSON}
	if err := writeSectionList(ctx, []api.Section{{ID: "s1", Name: "Sec", ProjectID: "p1"}}, ""); err != nil {
		t.Fatalf("writeSectionList: %v", err)
	}
	assertNDJSONLineHasField(t, ctx.Stdout.(*bytes.Buffer).String(), "id", "s1")
}

func TestWriteLabelListNDJSON(t *testing.T) {
	ctx := &Context{Stdout: &bytes.Buffer{}, Mode: output.ModeNDJSON}
	if err := writeLabelList(ctx, []api.Label{{ID: "l1", Name: "urgent"}}, ""); err != nil {
		t.Fatalf("writeLabelList: %v", err)
	}
	assertNDJSONLineHasField(t, ctx.Stdout.(*bytes.Buffer).String(), "id", "l1")
}

func TestWriteCommentListNDJSON(t *testing.T) {
	ctx := &Context{Stdout: &bytes.Buffer{}, Mode: output.ModeNDJSON}
	if err := writeCommentList(ctx, []api.Comment{{ID: "c1", Content: "hello"}}, ""); err != nil {
		t.Fatalf("writeCommentList: %v", err)
	}
	assertNDJSONLineHasField(t, ctx.Stdout.(*bytes.Buffer).String(), "id", "c1")
}

func assertNDJSONLineHasField(t *testing.T, data string, key, want string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(data), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one ndjson line, got %d (%q)", len(lines), data)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("invalid ndjson line: %v", err)
	}
	if got[key] != want {
		t.Fatalf("unexpected %s value: %v", key, got[key])
	}
}
